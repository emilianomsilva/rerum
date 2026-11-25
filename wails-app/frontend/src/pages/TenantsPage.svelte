<script lang="ts">
  import { onMount } from 'svelte'
  import DataTable from '../components/DataTable.svelte'
  import Icon from '../components/Icon.svelte'
  import TenantDialog from '../dialogs/TenantDialog.svelte'
  import TenantBatchDialog from '../dialogs/TenantBatchDialog.svelte'
  import {
    arquivarInquilino,
    buscarInquilino,
    excluirInquilinoCascata,
    listarInquilinos,
    listarLocacoes,
    reativarInquilino,
  } from '../lib/api'
  import { confirmAction } from '../lib/confirm'
  import { getSetting, setSetting } from '../lib/settings'
  import { error as notifyError, errorMessage, info, success } from '../lib/toast'
  import type { Inquilino, TableColumn } from '../lib/types'

  export let refreshToken = 0

  type StatusFilter = 'Ativos' | 'Arquivados' | 'Todos'
  const STATUS_OPTIONS: StatusFilter[] = ['Ativos', 'Arquivados', 'Todos']

  let items: Inquilino[] = []
  let loading = true
  let loadError = ''
  let search = getSetting<string>('inquilinos/search', '')
  let status: StatusFilter =
    STATUS_OPTIONS[Number(getSetting('inquilinos/status_index', 0))] || STATUS_OPTIONS[0]
  let dialog: { tenant: Inquilino | null } | null = null
  let batchOpen = false
  let busy = false

  $: filtered = items.filter((tenant) => {
    if (status === 'Ativos' && !tenant.ativo) return false
    if (status === 'Arquivados' && tenant.ativo) return false
    const term = search.trim().toLocaleLowerCase('pt-BR')
    if (!term) return true
    return `${tenant.nome} ${tenant.cpf} ${tenant.telefone} ${tenant.email}`
      .toLocaleLowerCase('pt-BR')
      .includes(term)
  })

  $: statusIndex = STATUS_OPTIONS.indexOf(status)

  const columns: TableColumn<Inquilino>[] = [
    { key: 'nome', label: 'Nome' },
    { key: 'cpf', label: 'CPF', value: (tenant) => tenant.cpf || '—' },
    { key: 'telefone', label: 'Telefone', value: (tenant) => tenant.telefone || '—' },
    { key: 'email', label: 'E-mail', value: (tenant) => tenant.email || '—' },
    {
      key: 'ativo',
      label: 'Status',
      value: (tenant) => (tenant.ativo ? 'Ativo' : 'Arquivado'),
      badge: (tenant) => ({
        text: tenant.ativo ? 'Ativo' : 'Arquivado',
        tone: tenant.ativo ? 'green' : 'gray',
      }),
    },
  ]

  async function load(): Promise<void> {
    loading = true
    loadError = ''
    try {
      items = (await listarInquilinos(status !== 'Ativos')) || []
    } catch (err) {
      loadError = errorMessage(err)
    } finally {
      loading = false
    }
  }

  let previousStatus = status
  $: if (status !== previousStatus) {
    previousStatus = status
    setSetting('inquilinos/status_index', statusIndex)
    load()
  }
  $: if (search !== getSetting('inquilinos/search', '')) setSetting('inquilinos/search', search)

  function openNew(): void {
    dialog = { tenant: null }
  }

  async function edit(tenant: Inquilino): Promise<void> {
    try {
      const fresh = await buscarInquilino(tenant.id)
      dialog = { tenant: fresh }
    } catch (err) {
      notifyError(errorMessage(err))
    }
  }

  async function archive(tenant: Inquilino): Promise<void> {
    const confirmed = await confirmAction({
      title: 'Arquivar inquilino',
      message: 'Arquivar este inquilino?',
      confirmLabel: 'Arquivar',
    })
    if (!confirmed) return
    busy = true
    try {
      const active = await listarLocacoes({ ativas: true })
      if ((active || []).some((lease) => lease.inquilino_id === tenant.id)) {
        throw new Error('Encerre a locação ativa antes de arquivar o inquilino.')
      }
      await arquivarInquilino(tenant.id)
      success('Inquilino arquivado.')
      await load()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function reactivate(tenant: Inquilino): Promise<void> {
    busy = true
    try {
      await reativarInquilino(tenant.id)
      success('Inquilino reativado.')
      await load()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function remove(tenant: Inquilino): Promise<void> {
    const confirmed = await confirmAction({
      title: 'Excluir inquilino e dependências',
      message:
        `Excluir permanentemente o inquilino «${tenant.nome}»?\n\n` +
        'Serão removidos contratos, parcelas/pagamentos e movimentações vinculados. ' +
        'Será criado um backup automático do banco antes da exclusão permanente.\n\n' +
        'Esta ação não pode ser desfeita.',
      confirmLabel: 'Excluir definitivamente',
      destructive: true,
    })
    if (!confirmed) return
    busy = true
    try {
      const result = await excluirInquilinoCascata(tenant.id)
      info(`Inquilino e dependências excluídos com sucesso.\nBackup criado em: ${result?.backup_path || 'pasta de backups'}`)
      await load()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  onMount(load)

  let previousToken = refreshToken
  $: if (refreshToken !== previousToken) {
    previousToken = refreshToken
    load()
  }
</script>

<div class="page">
  <div class="page-header">
    <div>
      <h1 class="page-title">Inquilinos</h1>
      <p class="page-subtitle">Gerencie o cadastro de inquilinos e seus dados para contato e IRPF.</p>
    </div>
  </div>

  <div class="toolbar">
    <div class="search grow">
      <Icon name="search" />
      <input class="input" bind:value={search} placeholder="Buscar inquilino por nome, CPF, telefone, e-mail..." aria-label="Buscar inquilino" />
    </div>
    <span class="toolbar-label">Exibir:</span>
    <select class="select" style="width:135px" bind:value={status} aria-label="Exibir inquilinos">
      {#each STATUS_OPTIONS as option}<option>{option}</option>{/each}
    </select>
    <div class="grow"></div>
    <button class="btn secondary" on:click={() => (batchOpen = true)}>Cadastrar em lote…</button>
    <button class="btn" on:click={openNew}><Icon name="plus" size={15} /> Novo inquilino</button>
  </div>

  {#if loadError}
    <div class="status-banner error">
      <Icon name="warning" size={16} /> {loadError}
      <button class="btn secondary small" style="margin-left:auto" on:click={load}>Tentar novamente</button>
    </div>
  {/if}
  {#if busy}<div class="status-banner info"><span class="spinner"></span> Processando…</div>{/if}

  <DataTable
    columns={columns}
    rows={filtered}
    keyFor={(tenant) => tenant.id}
    {loading}
    settingsKey="inquilinos"
    exportFilename="inquilinos-rerum.csv"
    empty="Nenhum inquilino encontrado."
    actions
    onRowDoubleClick={edit}
  >
    <svelte:fragment slot="actions" let:row={tenant}>
      {#if tenant.ativo}
        <button class="btn small secondary" on:click={() => edit(tenant)}>Editar</button>
        <button class="btn small secondary" disabled={busy} on:click={() => archive(tenant)}>Arquivar</button>
      {:else}
        <button class="btn small" disabled={busy} on:click={() => reactivate(tenant)}>Reativar</button>
      {/if}
      <button class="btn small destructive" disabled={busy} on:click={() => remove(tenant)}>Excluir</button>
    </svelte:fragment>
  </DataTable>
</div>

{#if dialog}
  <TenantDialog
    tenant={dialog.tenant}
    onClose={() => (dialog = null)}
    onSaved={() => {
      dialog = null
      load()
    }}
  />
{/if}

{#if batchOpen}
  <TenantBatchDialog
    onClose={() => (batchOpen = false)}
    onSaved={() => {
      batchOpen = false
      load()
    }}
  />
{/if}
