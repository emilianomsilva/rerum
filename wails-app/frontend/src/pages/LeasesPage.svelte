<script lang="ts">
  import { onMount } from 'svelte'
  import DataTable from '../components/DataTable.svelte'
  import Icon from '../components/Icon.svelte'
  import InstallmentDialog from '../dialogs/InstallmentDialog.svelte'
  import LeaseBatchDialog from '../dialogs/LeaseBatchDialog.svelte'
  import LeaseDialog from '../dialogs/LeaseDialog.svelte'
  import LeaseEditDialog from '../dialogs/LeaseEditDialog.svelte'
  import {
    buscarLocacao,
    encerrarLocacao,
    excluirLocacao,
    listarImoveis,
    listarInquilinos,
    listarLocacoes,
  } from '../lib/api'
  import { confirmAction } from '../lib/confirm'
  import { dateBR, money, periodLabel, todayISO } from '../lib/format'
  import { getSetting, setSetting } from '../lib/settings'
  import { error as notifyError, errorMessage, info, success } from '../lib/toast'
  import type { EditableLease, Imovel, Inquilino, LeaseSummary, TableColumn } from '../lib/types'

  export let refreshToken = 0

  type StatusFilter = 'todas' | 'ativas' | 'encerradas'

  let leases: LeaseSummary[] = []
  let properties: Imovel[] = []
  let tenants: Inquilino[] = []
  let loading = true
  let loadError = ''
  let statusFilter: StatusFilter = getSetting<StatusFilter>('locacoes/status_filter', 'ativas')
  let search = getSetting<string>('locacoes/search', '')
  let createOpen = false
  let batchOpen = false
  let editState: { lease: EditableLease; mode: 'edit' | 'renew' } | null = null
  let extraLeaseId: number | null = null
  let busy = false

  $: propertyMap = new Map(properties.map((p) => [p.id, p]))
  $: tenantMap = new Map(tenants.map((t) => [t.id, t]))
  $: occupiedIds = leases.filter((l) => l.status === 'ativa').map((l) => l.imovel_id)
  $: visible = filterLeases(leases, statusFilter, search, propertyMap, tenantMap)
  $: activeTotalRent = visible
    .filter((l) => l.status === 'ativa')
    .reduce((acc, l) => acc + (l.valor_mensal_c || 0), 0)

  function filterLeases(
    list: LeaseSummary[],
    flt: StatusFilter,
    termRaw: string,
    propMap: Map<number, Imovel>,
    tenMap: Map<number, Inquilino>,
  ): LeaseSummary[] {
    const term = termRaw.trim().toLocaleLowerCase('pt-BR')
    return list.filter((lease) => {
      if (flt === 'ativas' && lease.status !== 'ativa') return false
      if (flt === 'encerradas' && lease.status === 'ativa') return false
      if (!term) return true
      const prop = propMap.get(lease.imovel_id)
      const tenant = tenMap.get(lease.inquilino_id)
      const blob = `${prop?.apelido || lease.imovel || ''} ${tenant?.nome || lease.inquilino || ''} ${lease.intermediario || ''} ${lease.reajuste_indice || ''}`.toLocaleLowerCase('pt-BR')
      return blob.includes(term)
    })
  }

  function updateSearch(val: string): void {
    search = val
    setSetting('locacoes/search', val)
  }

  function updateStatus(val: StatusFilter): void {
    statusFilter = val
    setSetting('locacoes/status_filter', val)
  }

  async function refresh(): Promise<void> {
    loading = true
    loadError = ''
    try {
      const [l, p, t] = await Promise.all([
        listarLocacoes(),
        listarImoveis(true),
        listarInquilinos(true),
      ])
      leases = l || []
      properties = p || []
      tenants = t || []
    } catch (err) {
      loadError = errorMessage(err)
    } finally {
      loading = false
    }
  }

  onMount(refresh)

  let previousToken = refreshToken
  $: if (refreshToken !== previousToken) {
    previousToken = refreshToken
    void refresh()
  }

  let columns: TableColumn<LeaseSummary>[] = []
  $: columns = [
    {
      key: 'imovel',
      label: 'Imóvel',
      width: 165,
      text: (r) => propertyMap.get(r.imovel_id)?.apelido || r.imovel || `#${r.imovel_id}`,
    },
    {
      key: 'inquilino',
      label: 'Inquilino',
      width: 210,
      text: (r) => tenantMap.get(r.inquilino_id)?.nome || r.inquilino || `#${r.inquilino_id}`,
    },
    {
      key: 'contrato',
      label: 'Contrato',
      width: 175,
      sortValue: (r) => r.periodo_inicio || r.inicio,
      text: (r) =>
        r.periodo_inicio && r.periodo_fim
          ? periodLabel(r.periodo_inicio, r.periodo_fim)
          : periodLabel(r.inicio, r.fim),
    },
    {
      key: 'primeiro_vencimento',
      label: '1º Venc.',
      width: 100,
      sortValue: (r) => r.primeiro_vencimento || '',
      text: (r) => (r.primeiro_vencimento ? dateBR(r.primeiro_vencimento) : '—'),
    },
    {
      key: 'dia_vencimento',
      label: 'Dia',
      align: 'center',
      width: 60,
      value: (r) => r.dia_vencimento,
      text: (r) => `Dia ${r.dia_vencimento}`,
    },
    {
      key: 'valor_mensal_c',
      label: 'Aluguel',
      align: 'right',
      width: 120,
      value: (r) => r.valor_mensal_c,
      text: (r) => money(r.valor_mensal_c),
    },
    {
      key: 'parcelas',
      label: 'Parc.',
      align: 'center',
      width: 65,
      value: (r) => r.parcelas,
    },
    {
      key: 'intermediario',
      label: 'Intermediário',
      width: 135,
      text: (r) => r.intermediario || 'Direto',
    },
    {
      key: 'status',
      label: 'Status',
      width: 95,
      sortValue: (r) => (r.status === 'ativa' ? 0 : 1),
      text: (r) => (r.status === 'ativa' ? 'Ativa' : 'Encerrada'),
      badge: (r) =>
        r.status === 'ativa'
          ? { label: 'Ativa', tone: 'green' }
          : { label: 'Encerrada', tone: 'gray' },
    },
  ]

  async function openEdit(row: LeaseSummary, mode: 'edit' | 'renew' = 'edit'): Promise<void> {
    try {
      const full = await buscarLocacao(row.id)
      editState = {
        lease: {
          ...(full || row),
          imovel: row.imovel,
          inquilino: row.inquilino,
        },
        mode,
      }
    } catch (err) {
      notifyError(errorMessage(err))
    }
  }

  async function handleEndLease(row: LeaseSummary): Promise<void> {
    const ok = await confirmAction({
      title: 'Encerrar locação',
      message: `Deseja encerrar o contrato de ${propertyMap.get(row.imovel_id)?.apelido || row.imovel || 'imóvel'}? Parcelas futuras em aberto serão canceladas.`,
      confirmLabel: 'Encerrar contrato',
      danger: true,
    })
    if (!ok) return
    busy = true
    try {
      await encerrarLocacao(row.id, todayISO())
      info('Locação encerrada.')
      await refresh()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function handleDeleteLease(row: LeaseSummary): Promise<void> {
    const ok = await confirmAction({
      title: 'Excluir locação',
      message: 'Excluir permanentemente esta locação e suas parcelas? Use apenas para cadastros errados sem histórico relevante.',
      confirmLabel: 'Excluir',
      danger: true,
    })
    if (!ok) return
    busy = true
    try {
      await excluirLocacao(row.id)
      success('Locação excluída.')
      await refresh()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }
</script>

<div class="page">
  <div class="page-header">
    <div>
      <h1 class="page-title">Locações</h1>
      <p class="page-subtitle">Gerencie contratos, renovações, intermediários e gere cobranças automáticas.</p>
    </div>
  </div>

  <div class="toolbar">
    <div class="search">
      <Icon name="search" />
      <input
        class="input"
        type="search"
        placeholder="Buscar por imóvel, inquilino ou intermediário…"
        value={search}
        on:input={(e) => updateSearch(e.currentTarget.value)}
        aria-label="Buscar locação"
      />
    </div>

    <span class="toolbar-label">Filtrar:</span>
    <select
      class="select"
      style="width:175px"
      value={statusFilter}
      on:change={(e) => updateStatus(e.currentTarget.value as StatusFilter)}
      aria-label="Filtrar status de locações"
    >
      <option value="todas">Todas as locações</option>
      <option value="ativas">Ativas</option>
      <option value="encerradas">Encerradas</option>
    </select>

    {#if activeTotalRent > 0}
      <span class="summary-pill" title="Soma mensal dos contratos ativos exibidos">
        Receita contratada: {money(activeTotalRent)}/mês
      </span>
    {/if}

    <div class="spacer"></div>
    <button class="btn secondary" on:click={() => (batchOpen = true)} disabled={busy}>
      Lançar em lote…
    </button>
    <button class="btn" on:click={() => (createOpen = true)} disabled={busy}>
      <Icon name="plus" size={15} /> Nova locação
    </button>
  </div>

  {#if loadError}
    <div class="status-banner error">
      <Icon name="warning" size={16} /> {loadError}
      <button class="btn secondary small" style="margin-left:auto" on:click={refresh}>Tentar novamente</button>
    </div>
  {/if}
  {#if busy}<div class="status-banner info"><span class="spinner"></span> Processando…</div>{/if}

  <DataTable
    rows={visible}
    {columns}
    keyFor={(r) => r.id}
    {loading}
    settingsKey="locacoes"
    exportFilename="locacoes-rerum.csv"
    emptyText="Nenhuma locação encontrada."
    onRowDoubleClick={(r) => void openEdit(r, 'edit')}
    actions={[
      { label: 'Editar', onClick: (r) => openEdit(r, 'edit') },
      { label: 'Renovar', onClick: (r) => openEdit(r, 'renew'), when: (r) => r.status === 'ativa' },
      { label: '+ Extra', onClick: (r) => { extraLeaseId = r.id }, when: (r) => r.status === 'ativa' },
      { label: 'Encerrar', onClick: handleEndLease, when: (r) => r.status === 'ativa', danger: true },
      { label: 'Excluir', onClick: handleDeleteLease, danger: true },
    ]}
  />
</div>

{#if createOpen}
  <LeaseDialog
    {properties}
    {occupiedIds}
    {tenants}
    onClose={() => (createOpen = false)}
    onSaved={() => {
      createOpen = false
      void refresh()
    }}
  />
{/if}

{#if batchOpen}
  <LeaseBatchDialog
    {properties}
    {occupiedIds}
    {tenants}
    onClose={() => (batchOpen = false)}
    onSaved={() => {
      batchOpen = false
      void refresh()
    }}
  />
{/if}

{#if editState}
  <LeaseEditDialog
    lease={editState.lease}
    mode={editState.mode}
    onClose={() => (editState = null)}
    onSaved={() => {
      editState = null
      void refresh()
    }}
  />
{/if}

{#if extraLeaseId !== null}
  <InstallmentDialog
    leaseId={extraLeaseId}
    leases={leases}
    onClose={() => (extraLeaseId = null)}
    onSaved={() => {
      extraLeaseId = null
      void refresh()
    }}
  />
{/if}
