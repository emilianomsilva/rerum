<script lang="ts">
  import { onMount } from 'svelte'
  import DataTable from '../components/DataTable.svelte'
  import Icon from '../components/Icon.svelte'
  import PropertyDialog from '../dialogs/PropertyDialog.svelte'
  import PropertyBatchDialog from '../dialogs/PropertyBatchDialog.svelte'
  import {
    arquivarImovel,
    buscarImovel,
    dashboard as loadDashboard,
    excluirImovelCascata,
    listarEmpreendimentos,
    listarImoveis,
    listarLocacoes,
    listarParcelas,
    reativarImovel,
  } from '../lib/api'
  import { confirmAction } from '../lib/confirm'
  import { money, parseISO, percent, todayISO, toISO } from '../lib/format'
  import { ROI_PERIOD_OPTIONS, addDays, defaultCustomRange, resolvePeriod } from '../lib/periods'
  import { getSetting, setSetting } from '../lib/settings'
  import { error as notifyError, errorMessage, info, success } from '../lib/toast'
  import type { BadgeSpec, Empreendimento, Imovel, Roi, TableColumn } from '../lib/types'

  export let refreshToken = 0

  type CategoryFilter =
    | 'Todos os ativos'
    | 'Unidades de locação'
    | 'Patrimônio / Lotes'
    | 'Vendidos'
    | 'Arquivados'

  const CATEGORIES: CategoryFilter[] = [
    'Todos os ativos',
    'Unidades de locação',
    'Patrimônio / Lotes',
    'Vendidos',
    'Arquivados',
  ]

  const defaults = defaultCustomRange()
  let items: Imovel[] = []
  let projects: Empreendimento[] = []
  let rois = new Map<number, Roi>()
  let occupiedIds = new Set<number>()
  let loading = true
  let loadError = ''
  let search = getSetting<string>('imoveis/search', '')
  let category: CategoryFilter =
    CATEGORIES[Number(getSetting('imoveis/category_index', 0))] || CATEGORIES[0]
  let roiPeriod = getSetting<string>('imoveis/roi_period', '12')
  let customStart = getSetting<string>('imoveis/roi_start', defaults.start)
  let customEnd = getSetting<string>('imoveis/roi_end', defaults.end)
  let days = Number(getSetting('imoveis/roi_days', defaults.days)) || defaults.days

  let dialog: { property: Imovel | null; duplicate?: boolean } | null = null
  let batchOpen = false
  let busy = false

  $: resolvedPeriod = resolvePeriod(roiPeriod, customStart, customEnd)
  $: visible = filterItems(items, category, search)
  $: categoryIndex = CATEGORIES.indexOf(category)

  function filterItems(list: Imovel[], cat: CategoryFilter, termRaw: string): Imovel[] {
    const term = termRaw.trim().toLocaleLowerCase('pt-BR')
    return list.filter((item) => {
      if (cat === 'Arquivados') {
        if (item.ativo) return false
      } else if (
        !item.ativo ||
        (cat === 'Unidades de locação' && (!item.destinado_locacao || item.situacao_patrimonial === 'vendido')) ||
        (cat === 'Patrimônio / Lotes' && (item.destinado_locacao || item.situacao_patrimonial === 'vendido')) ||
        (cat === 'Vendidos' && item.situacao_patrimonial !== 'vendido' && !item.data_venda && !item.valor_venda_c)
      ) {
        return false
      }
      if (term) {
        const blob = `${item.apelido} ${item.endereco} ${item.numero} ${item.unidade} ${item.bairro} ${item.matricula_cartorio} ${item.inscricao_iptu} ${item.unidade_agua} ${item.unidade_energia}`.toLocaleLowerCase('pt-BR')
        if (!blob.includes(term)) return false
      }
      return true
    })
  }

  function investimento(item: Imovel, roiMap: Map<number, Roi>): number {
    const roiEntry = roiMap.get(item.id)
    if (roiEntry && roiEntry.investimento_c > 0) return roiEntry.investimento_c
    return (
      (item.valor_compra_c || 0) +
      (item.reforma_c || 0) +
      (item.itbi_c || 0) +
      (item.corretagem_c || 0) +
      (item.outros_custos_c || 0) +
      (item.investimento_importado_c || 0)
    )
  }

  function roiPctOf(item: Imovel, roiMap: Map<number, Roi>, allTime: boolean): number | null {
    const entry = roiMap.get(item.id)
    if (!entry || investimento(item, roiMap) <= 0) return null
    return allTime ? entry.roi_acumulado_pct : entry.roi_anual_pct
  }

  function statusOf(item: Imovel, occupied: Set<number>): Required<Pick<BadgeSpec, 'text' | 'tone'>> {
    if (!item.ativo) return { text: 'Arquivado', tone: 'gray' }
    if (item.situacao_patrimonial === 'vendido' || item.data_venda) return { text: 'Vendido', tone: 'blue' }
    if (occupied.has(item.id)) return { text: 'Alugado', tone: 'green' }
    if (item.status === 'manutencao') return { text: 'Em manutenção', tone: 'amber' }
    return { text: 'Disponível', tone: 'green' }
  }

  let columns: TableColumn<Imovel>[] = []
  $: columns = [
    { key: 'apelido', label: 'Imóvel', width: '170px' },
    { key: 'endereco', label: 'Endereço', value: (item) => item.endereco || '—' },
    { key: 'destinado', label: 'Destinado à', width: 120, value: (item) => (item.destinado_locacao ? 'Locação' : 'Patrimônio') },
    {
      key: 'status',
      label: 'Status',
      width: 120,
      value: (item) => statusOf(item, occupiedIds).text,
      badge: (item) => statusOf(item, occupiedIds),
    },
    {
      key: 'investimento',
      label: 'Investimento',
      align: 'right',
      width: 135,
      value: (item) => money(investimento(item, rois)),
      sortValue: (item) => investimento(item, rois),
    },
    {
      key: 'roi',
      label: `ROI (${resolvedPeriod.shortLabel})`,
      align: 'right',
      width: 135,
      title: resolvedPeriod.hint,
      value: (item) => {
        const val = roiPctOf(item, rois, resolvedPeriod.allTime)
        return val !== null ? percent(val) : '—'
      },
      sortValue: (item) => roiPctOf(item, rois, resolvedPeriod.allTime) ?? -999999,
    },
  ]

  async function load(): Promise<void> {
    loading = true
    loadError = ''
    try {
      const pInfo = resolvePeriod(roiPeriod, customStart, customEnd)
      const rangeParam = pInfo.allTime ? undefined : { inicio: pInfo.start, fim: pInfo.end }
      const [propertyItems, dashboard, activeLeases, projectItems] = await Promise.all([
        listarImoveis(category === 'Arquivados'),
        loadDashboard(todayISO(), rangeParam),
        listarLocacoes({ ativas: true }),
        listarEmpreendimentos(true),
      ])
      items = propertyItems || []
      projects = projectItems || []
      rois = new Map((dashboard?.rois || []).map((item) => [item.imovel_id, item]))
      occupiedIds = new Set((activeLeases || []).map((lease) => lease.imovel_id))
    } catch (err) {
      loadError = errorMessage(err)
    } finally {
      loading = false
    }
  }

  function persistFilters(): void {
    setSetting('imoveis/search', search)
    setSetting('imoveis/category_index', categoryIndex)
    setSetting('imoveis/roi_period', roiPeriod)
    setSetting('imoveis/roi_start', customStart)
    setSetting('imoveis/roi_end', customEnd)
    setSetting('imoveis/roi_days', days)
  }

  let previousCategory = category
  let previousRoiPeriod = roiPeriod
  $: if (category !== previousCategory || roiPeriod !== previousRoiPeriod) {
    previousCategory = category
    previousRoiPeriod = roiPeriod
    persistFilters()
    void load()
  }

  $: if (search !== getSetting('imoveis/search', '')) setSetting('imoveis/search', search)

  function applyCustomDates(): void {
    const s = parseISO(customStart)
    const e = parseISO(customEnd)
    if (s && e) days = Math.max(1, Math.round((e.getTime() - s.getTime()) / 86400000) + 1)
    persistFilters()
    void load()
  }

  function applyCustomDays(): void {
    const end = parseISO(customEnd) || new Date()
    customStart = toISO(addDays(end, -(Math.max(1, Number(days) || 1) - 1)))
    persistFilters()
    void load()
  }

  function openNew(): void {
    dialog = { property: null }
  }

  async function openEdit(item: Imovel): Promise<void> {
    try {
      const [fresh, projectItems] = await Promise.all([buscarImovel(item.id), listarEmpreendimentos(true)])
      projects = projectItems || []
      dialog = { property: fresh }
    } catch (err) {
      notifyError(errorMessage(err))
    }
  }

  async function duplicate(item: Imovel): Promise<void> {
    try {
      const [fresh, projectItems] = await Promise.all([buscarImovel(item.id), listarEmpreendimentos()])
      projects = projectItems || []
      dialog = {
        property: {
          ...fresh,
          id: 0,
          apelido: `${fresh.apelido} (Cópia)`,
          status: 'disponivel',
          ativo: true,
          situacao_patrimonial: 'proprio',
          data_venda: null,
          valor_venda_c: 0,
        },
        duplicate: true,
      }
    } catch (err) {
      notifyError(errorMessage(err))
    }
  }

  async function archive(item: Imovel): Promise<void> {
    const confirmed = await confirmAction({
      title: 'Arquivar imóvel',
      message: 'Arquivar este imóvel?',
      confirmLabel: 'Arquivar',
    })
    if (!confirmed) return
    busy = true
    try {
      const [active, installments] = await Promise.all([
        listarLocacoes({ imovel_id: item.id, ativas: true }),
        listarParcelas({ imovel_id: item.id }),
      ])
      if ((active || []).length) throw new Error('Encerre a locação ativa antes de arquivar.')
      if ((installments || []).some((row) => row.status === 'aberta' || row.status === 'parcial')) {
        throw new Error('O imóvel possui parcelas em aberto.')
      }
      await arquivarImovel(item.id)
      success('Imóvel arquivado.')
      await load()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function reactivate(item: Imovel): Promise<void> {
    busy = true
    try {
      await reativarImovel(item.id)
      success('Imóvel reativado.')
      await load()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function remove(item: Imovel): Promise<void> {
    const confirmed = await confirmAction({
      title: 'Excluir imóvel e dependências',
      message:
        `Excluir permanentemente o imóvel «${item.apelido}»?\n\n` +
        'Serão removidos contratos, parcelas e lançamentos vinculados a este imóvel. ' +
        'Será criado um backup automático do banco antes da exclusão permanente.\n\n' +
        'Esta ação não pode ser desfeita.',
      confirmLabel: 'Excluir definitivamente',
      destructive: true,
    })
    if (!confirmed) return
    busy = true
    try {
      const result = await excluirImovelCascata(item.id)
      info(`Imóvel e dependências excluídos com sucesso.\nBackup criado em: ${result?.backup_path || 'pasta de backups'}`)
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
    void load()
  }
</script>

<div class="page">
  <div class="page-header">
    <div>
      <h1 class="page-title">Imóveis</h1>
      <p class="page-subtitle">Cadastre seus imóveis, custos de aquisição e destinação para cálculo de ROI.</p>
    </div>
  </div>

  <div class="toolbar">
    <div class="search grow">
      <Icon name="search" />
      <input class="input" bind:value={search} placeholder="Buscar por apelido, endereço, matrícula, IPTU, UC..." aria-label="Buscar imóvel" />
    </div>

    <span class="toolbar-label">Categoria:</span>
    <select class="select" style="width:185px" bind:value={category} aria-label="Categoria de imóveis">
      {#each CATEGORIES as option}<option>{option}</option>{/each}
    </select>

    <span class="toolbar-label">Janela de ROI:</span>
    <select class="select" style="width:190px" bind:value={roiPeriod} aria-label="Período do ROI">
      {#each ROI_PERIOD_OPTIONS as [value, label]}
        <option {value}>{label}</option>
      {/each}
    </select>

    {#if roiPeriod === 'custom'}
      <input class="input" type="date" bind:value={customStart} on:change={applyCustomDates} aria-label="Início do ROI" />
      <input class="input" type="date" bind:value={customEnd} on:change={applyCustomDates} aria-label="Fim do ROI" />
      <input class="input" type="number" min="1" max="3650" bind:value={days} on:change={applyCustomDays} aria-label="Dias" />
      <span class="helper">dias</span>
    {/if}

    <button class="btn secondary" on:click={() => (batchOpen = true)}>Cadastrar em lote…</button>
    <button class="btn" on:click={openNew}><Icon name="plus" size={15} /> Novo imóvel</button>
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
    keyFor={(item) => item.id}
    {loading}
    settingsKey="imoveis"
    exportFilename="imoveis-rerum.csv"
    empty="Nenhum imóvel encontrado."
    actions
    onRowDoubleClick={openEdit}
  >
    <svelte:fragment slot="actions" let:row={item}>
      {#if item.ativo}
        <button class="btn small secondary" on:click={() => openEdit(item)}>Editar</button>
        <button class="btn small secondary" on:click={() => duplicate(item)}>Duplicar</button>
        <button class="btn small secondary" disabled={busy} on:click={() => archive(item)}>Arquivar</button>
      {:else}
        <button class="btn small" disabled={busy} on:click={() => reactivate(item)}>Reativar</button>
      {/if}
      <button class="btn small destructive" disabled={busy} on:click={() => remove(item)}>Excluir</button>
    </svelte:fragment>
  </DataTable>
</div>

{#if dialog}
  <PropertyDialog
    property={dialog.property}
    projects={projects}
    onClose={() => (dialog = null)}
    onSaved={() => {
      dialog = null
      void load()
    }}
  />
{/if}

{#if batchOpen}
  <PropertyBatchDialog
    projects={projects}
    onClose={() => (batchOpen = false)}
    onSaved={() => {
      batchOpen = false
      void load()
    }}
  />
{/if}
