<script lang="ts">
  import { onMount } from 'svelte'
  import DataTable from '../components/DataTable.svelte'
  import Icon from '../components/Icon.svelte'
  import ProjectDialog from '../dialogs/ProjectDialog.svelte'
  import {
    arquivarEmpreendimento,
    buscarEmpreendimento,
    excluirEmpreendimento,
    listarEmpreendimentos,
    listarImoveis,
    reativarEmpreendimento,
  } from '../lib/api'
  import { confirmAction } from '../lib/confirm'
  import { getSetting, setSetting } from '../lib/settings'
  import { error as notifyError, errorMessage, success } from '../lib/toast'
  import type { Empreendimento, Imovel, TableColumn } from '../lib/types'

  export let refreshToken = 0

  type StatusFilter = 'Ativos' | 'Arquivados' | 'Todos'
  const STATUS_OPTIONS: StatusFilter[] = ['Ativos', 'Arquivados', 'Todos']

  let items: Empreendimento[] = []
  let properties: Imovel[] = []
  let loading = true
  let loadError = ''
  let search = getSetting<string>('empreendimentos/search', '')
  let status: StatusFilter =
    STATUS_OPTIONS[Number(getSetting('empreendimentos/status_index', 0))] || STATUS_OPTIONS[0]
  let dialog: { project: Empreendimento | null } | null = null
  let busy = false

  $: visible = items.filter((project) => {
    if (status !== 'Todos' && project.ativo !== (status === 'Ativos')) return false
    const term = search.trim().toLocaleLowerCase('pt-BR')
    if (!term) return true
    return `${project.nome} ${project.endereco} ${project.observacao}`
      .toLocaleLowerCase('pt-BR')
      .includes(term)
  })

  $: statusIndex = STATUS_OPTIONS.indexOf(status)

  let columns: TableColumn<Empreendimento>[] = []
  $: columns = [
    { key: 'nome', label: 'Nome', width: '200px' },
    { key: 'endereco', label: 'Endereço', value: (project) => project.endereco || '—' },
    {
      key: 'vinculados',
      label: 'Imóveis vinculados',
      align: 'center',
      value: (project) => properties.filter((item) => item.empreendimento_id === project.id).length,
    },
    {
      key: 'unidades',
      label: 'Unidades para rateio',
      align: 'center',
      value: (project) => (project.unidade_ids || []).length,
    },
    {
      key: 'ativo',
      label: 'Status',
      value: (project) => (project.ativo ? 'Ativo' : 'Arquivado'),
      badge: (project) => ({
        text: project.ativo ? 'Ativo' : 'Arquivado',
        tone: project.ativo ? 'green' : 'gray',
      }),
    },
  ]

  async function load(): Promise<void> {
    loading = true
    loadError = ''
    try {
      const [projectItems, propertyItems] = await Promise.all([
        listarEmpreendimentos(true),
        listarImoveis(true),
      ])
      items = projectItems || []
      properties = propertyItems || []
    } catch (err) {
      loadError = errorMessage(err)
    } finally {
      loading = false
    }
  }

  let previousStatus = status
  $: if (status !== previousStatus) {
    previousStatus = status
    setSetting('empreendimentos/status_index', statusIndex)
  }
  $: if (search !== getSetting('empreendimentos/search', '')) setSetting('empreendimentos/search', search)

  async function edit(project: Empreendimento): Promise<void> {
    try {
      const [fresh, propertyItems] = await Promise.all([
        buscarEmpreendimento(project.id),
        listarImoveis(true),
      ])
      properties = propertyItems || []
      dialog = { project: fresh }
    } catch (err) {
      notifyError(errorMessage(err))
    }
  }

  function openNew(): void {
    dialog = { project: null }
  }

  async function changeActive(project: Empreendimento): Promise<void> {
    const action = project.ativo ? 'Arquivar' : 'Reativar'
    const confirmed = await confirmAction({
      title: `${action} empreendimento`,
      message: `${action} «${project.nome}»? Os imóveis e o histórico financeiro serão preservados.`,
      confirmLabel: action,
    })
    if (!confirmed) return
    busy = true
    try {
      if (project.ativo) await arquivarEmpreendimento(project.id)
      else await reativarEmpreendimento(project.id)
      success(`Empreendimento ${project.ativo ? 'arquivado' : 'reativado'}.`)
      await load()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function remove(project: Empreendimento): Promise<void> {
    const confirmed = await confirmAction({
      title: 'Excluir empreendimento',
      message:
        `Excluir definitivamente «${project.nome}»?\n\n` +
        'Só é permitido excluir empreendimentos sem imóveis, unidades de rateio ou lançamentos vinculados. ' +
        'Caso já tenha sido utilizado, prefira arquivar.',
      confirmLabel: 'Excluir definitivamente',
      destructive: true,
    })
    if (!confirmed) return
    busy = true
    try {
      await excluirEmpreendimento(project.id)
      success('Empreendimento excluído.')
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
      <h1 class="page-title">Empreendimentos</h1>
      <p class="page-subtitle">Gerencie os empreendimentos, seus imóveis e os padrões para rateios futuros.</p>
    </div>
  </div>

  <div class="toolbar">
    <div class="search grow">
      <Icon name="search" />
      <input class="input" bind:value={search} placeholder="Buscar por nome, endereço ou observações…" aria-label="Buscar empreendimento" />
    </div>
    <select class="select" style="width:135px" bind:value={status} aria-label="Filtro de status">
      {#each STATUS_OPTIONS as option}<option>{option}</option>{/each}
    </select>
    <div class="grow"></div>
    <button class="btn" on:click={openNew}><Icon name="plus" size={15} /> Novo empreendimento</button>
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
    rows={visible}
    keyFor={(project) => project.id}
    {loading}
    settingsKey="empreendimentos"
    exportFilename="empreendimentos-rerum.csv"
    empty="Nenhum empreendimento encontrado."
    actions
    onRowDoubleClick={edit}
  >
    <svelte:fragment slot="actions" let:row={project}>
      <button class="btn small secondary" on:click={() => edit(project)}>Editar</button>
      <button class="btn small secondary" disabled={busy} on:click={() => changeActive(project)}>
        {project.ativo ? 'Arquivar' : 'Reativar'}
      </button>
      <button class="btn small destructive" disabled={busy} on:click={() => remove(project)}>Excluir</button>
    </svelte:fragment>
  </DataTable>
</div>

{#if dialog}
  <ProjectDialog
    project={dialog.project}
    properties={properties}
    onClose={() => (dialog = null)}
    onSaved={() => {
      dialog = null
      load()
    }}
  />
{/if}
