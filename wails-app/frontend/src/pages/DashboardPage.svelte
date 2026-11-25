<script lang="ts">
  import { onMount } from 'svelte'
  import DataTable from '../components/DataTable.svelte'
  import CashflowChart from '../components/CashflowChart.svelte'
  import RoiChart from '../components/RoiChart.svelte'
  import Icon from '../components/Icon.svelte'
  import PaymentDialog from '../dialogs/PaymentDialog.svelte'
  import LeaseEditDialog from '../dialogs/LeaseEditDialog.svelte'
  import {
    analyticsSeries,
    buscarParcela,
    copyToClipboard,
    dashboard as loadDashboard,
    listarImoveis,
    listarInquilinos,
    listarLocacoes,
    listarParcelas,
    openExternal,
    simularParcela,
  } from '../lib/api'
  import { dateBR, daysBetween, money, parseISO, percent, todayISO, toISO } from '../lib/format'
  import {
    DASHBOARD_PERIOD_OPTIONS,
    addDays,
    defaultCustomRange,
    resolvePeriod,
    type ResolvedPeriod,
  } from '../lib/periods'
  import { getSetting, setSetting } from '../lib/settings'
  import { error as notifyError, errorMessage, info, success } from '../lib/toast'
  import type {
    BadgeTone,
    Dashboard,
    DashboardPayment,
    Imovel,
    Inquilino,
    LeaseSummary,
    MonthlyCashflow,
    PageRoute,
    Parcela,
    Roi,
    Simulation,
    TableColumn,
  } from '../lib/types'

  export let onNavigate: (page: PageRoute) => void = () => {}
  export let refreshToken = 0

  type ChartView = 'both' | 'income' | 'roi'
  type ChargeKind = 'vencida' | 'mes_atual' | 'prox_mes'
  type CardKey =
    | 'recebido_mes'
    | 'a_receber_mes'
    | 'vencidas'
    | 'ocupacao'
    | 'resultado_mes'
    | 'despesas_mes'
    | 'capital_investido'
    | 'roi_12m'

  interface ChargeRow {
    kind: ChargeKind
    badge: string
    tone: BadgeTone
    item: DashboardPayment
  }

  interface RenewalRow {
    lease: LeaseSummary
    days: number
    tenant: Inquilino | undefined
  }

  let loading = true
  let filtering = false
  let loadError = ''
  let dashboardData: Dashboard | null = null
  let leases: LeaseSummary[] = []
  let tenants: Inquilino[] = []
  let properties: Imovel[] = []
  let installments: Parcela[] = []

  const defaults = defaultCustomRange()
  let range = String(getSetting('dashboard/chart_range', 'mes_atual'))
  let chartView: ChartView = getSetting<ChartView>('dashboard/chart_view', 'both')
  let stacked = getSetting<boolean>('dashboard/bar_type', false) === true
  let customStart = getSetting<string>('dashboard/chart_start', defaults.start)
  let customEnd = getSetting<string>('dashboard/chart_end', defaults.end)
  let days = Number(getSetting('dashboard/days_spin', defaults.days)) || defaults.days
  let propertyId = Number(getSetting('dashboard/property_id', 0)) || 0
  let series: MonthlyCashflow[] = []
  let rois: Roi[] = []
  let quitting = false
  let paymentDialog: { simulation: Simulation; parcela: Parcela } | null = null
  let renewingLease: LeaseSummary | null = null

  const DEFAULT_CARDS: CardKey[] = ['recebido_mes', 'a_receber_mes', 'vencidas', 'ocupacao']
  const CARD_ORDER: CardKey[] = [
    'recebido_mes',
    'a_receber_mes',
    'vencidas',
    'ocupacao',
    'resultado_mes',
    'despesas_mes',
    'capital_investido',
    'roi_12m',
  ]
  let visibleCards: CardKey[] = normalizeCards(getSetting('dashboard/visible_cards', DEFAULT_CARDS))
  let renewalNoticeDays = Number(getSetting('dashboard/renewal_notice_days', 60)) || 60

  function normalizeCards(value: unknown): CardKey[] {
    const list = Array.isArray(value) ? value : String(value || '').split(',')
    const clean = list
      .map((item) => String(item).trim())
      .filter((item): item is CardKey => CARD_ORDER.includes(item as CardKey))
    return clean.length ? clean : [...DEFAULT_CARDS]
  }

  $: selectedProperty = properties.find((item) => item.id === Number(propertyId)) || null
  $: rangeIsCustom = range === 'custom'
  $: currentPeriod = resolvePeriod(range, customStart, customEnd)
  $: filteredRois = selectedProperty ? rois.filter((item) => item.imovel_id === selectedProperty.id) : rois
  $: cardEntries = buildCards(currentPeriod, dashboardData, rois, series, installments, selectedProperty, visibleCards)

  function buildCards(
    periodInfo: ResolvedPeriod,
    data: Dashboard | null,
    roiList: Roi[],
    seriesList: MonthlyCashflow[],
    installmentList: Parcela[],
    prop: Imovel | null,
    cards: CardKey[],
  ): Array<[string, string, string]> {
    const recTotal = seriesList.reduce((total, item) => total + (item.receitas_c || 0), 0)
    const despTotal = seriesList.reduce((total, item) => total + (item.despesas_operacionais_c || 0), 0)
    const start = periodInfo.start
    const end = periodInfo.end
    const todayText = todayISO()
    let receivable = 0
    let overdue = 0
    let overdueCount = 0
    for (const item of installmentList) {
      if (!isOpen(item)) continue
      if (prop && item.imovel !== prop.apelido) continue
      const balance = Math.max(0, item.saldo_c ?? 0)
      if (balance <= 0) continue
      const due = String(item.vencimento).slice(0, 10)
      if (
        periodInfo.allTime ||
        (due >= start && due <= end) ||
        (periodInfo.monthly && item.competencia === start.slice(0, 7))
      ) {
        receivable += balance
      }
      if (due < todayText) {
        overdue += balance
        overdueCount += 1
      }
    }

    const occupancy = data ? `${data.ocupados} · ${percent(data.taxa_ocupacao_pct)}` : '—'

    // Compute portfolio or selected-property ROI strictly over the chosen period
    const scopedRois = prop ? roiList.filter((r) => r.imovel_id === prop.id) : roiList
    const capitalBase = scopedRois.reduce((sum, r) => sum + (r.investimento_c || 0), 0)
    const periodNet = scopedRois.reduce(
      (sum, r) =>
        sum +
        (periodInfo.allTime
          ? (r.recebido_total_c || 0) - (r.despesas_total_c || 0)
          : (r.recebido_12m_c || 0) - (r.despesas_12m_c || 0)),
      0,
    )
    const roiPct =
      capitalBase > 0 ? Math.round((periodNet * 10000) / capitalBase) / 100 : null
    const roiText = roiPct !== null ? percent(roiPct) : '—'

    const catalog: Record<CardKey, [string, string, string]> = {
      recebido_mes: [periodInfo.receiptTitle, money(recTotal), `Receitas e quitações · ${periodInfo.hint}`],
      a_receber_mes: [
        periodInfo.receivableTitle,
        money(receivable),
        periodInfo.allTime
          ? 'Todas as parcelas em aberto'
          : `Parcelas com vencimento no período (${dateBR(start)} a ${dateBR(end)})`,
      ],
      vencidas: ['VENCIDAS', money(overdue), `${overdueCount} parcelas vencidas com saldo em aberto`],
      ocupacao: ['OCUPAÇÃO', occupancy, 'Locações ativas ÷ imóveis'],
      resultado_mes: ['RESULTADO LÍQUIDO', money(recTotal - despTotal), `Receitas menos despesas operacionais (${periodInfo.hint})`],
      despesas_mes: [periodInfo.expenseTitle, money(despTotal), `Despesas operacionais no período (${periodInfo.hint})`],
      capital_investido: ['PATRIMÔNIO INVESTIDO', capitalBase > 0 ? money(capitalBase) : data ? money(data.capital_investido_c) : '—', 'Base de investimento no patrimônio'],
      roi_12m: [periodInfo.roiTitle, roiText, `Retorno líquido ÷ Capital (${periodInfo.hint})`],
    }
    const chosen = CARD_ORDER.filter((key) => cards.includes(key)).map((key) => catalog[key])
    return chosen.length ? chosen : DEFAULT_CARDS.map((key) => catalog[key])
  }

  function isOpen(item: Parcela): boolean {
    return item.status !== 'paga' && item.status !== 'cancelada'
  }

  function openCharges(data: Dashboard | null): ChargeRow[] {
    const seen = new Set<number>()
    const combined: ChargeRow[] = []
    for (const item of data?.vencidas || []) {
      if (!seen.has(item.id)) {
        seen.add(item.id)
        combined.push({ kind: 'vencida', badge: 'Vencida', tone: 'red', item })
      }
    }
    for (const item of data?.proximas || []) {
      if (seen.has(item.id)) continue
      seen.add(item.id)
      const due = parseISO(item.vencimento)
      const now = new Date()
      if (due && due.getFullYear() === now.getFullYear() && due.getMonth() === now.getMonth()) {
        combined.push({ kind: 'mes_atual', badge: 'Mês atual', tone: 'amber', item })
      } else {
        combined.push({ kind: 'prox_mes', badge: 'Próx. mês', tone: 'green', item })
      }
    }
    const weight: Record<ChargeKind, number> = { vencida: 0, mes_atual: 1, prox_mes: 2 }
    return combined
      .sort(
        (a, b) =>
          weight[a.kind] - weight[b.kind] ||
          String(a.item.vencimento).localeCompare(String(b.item.vencimento)),
      )
      .slice(0, 8)
  }

  function upcomingRenewals(
    leaseList: LeaseSummary[],
    tenantList: Inquilino[],
    noticeDays: number,
  ): RenewalRow[] {
    const now = new Date()
    const map = new Map(tenantList.map((item) => [item.id, item]))
    return leaseList
      .filter((lease) => lease.status === 'ativa')
      .map((lease) => ({ lease, days: daysBetween(now, lease.fim), tenant: map.get(lease.inquilino_id) }))
      .filter((entry) => entry.days <= noticeDays)
      .sort((a, b) => a.days - b.days)
      .slice(0, 6)
  }

  $: chargesRows = openCharges(dashboardData)
  $: renewalRows = upcomingRenewals(leases, tenants, renewalNoticeDays)

  const chargeWeight: Record<ChargeKind, number> = { vencida: 0, mes_atual: 1, prox_mes: 2 }
  const chargeColumns: TableColumn<ChargeRow>[] = [
    {
      key: 'badge',
      label: 'Status',
      badge: (row) => ({ text: row.badge, tone: row.tone }),
      sortValue: (row) => chargeWeight[row.kind],
    },
    {
      key: 'vencimento',
      label: 'Vencimento',
      value: (row) => dateBR(row.item.vencimento),
      sortValue: (row) => String(row.item.vencimento),
    },
    { key: 'imovel', label: 'Imóvel', value: (row) => row.item.imovel || '—' },
    { key: 'inquilino', label: 'Inquilino', value: (row) => row.item.inquilino || '—' },
    {
      key: 'saldo',
      label: 'Saldo',
      align: 'right',
      value: (row) => money(row.item.saldo_c),
      sortValue: (row) => row.item.saldo_c,
    },
  ]

  function roiRangeFor(periodInfo: ResolvedPeriod): { inicio?: string; fim?: string } {
    if (periodInfo.allTime) return {}
    return { inicio: periodInfo.start, fim: periodInfo.end }
  }

  async function loadAll(): Promise<void> {
    loading = true
    loadError = ''
    try {
      const periodInfo = resolvePeriod(range, customStart, customEnd)
      const [data, leaseItems, tenantItems, propertyItems, installmentItems] = await Promise.all([
        loadDashboard(todayISO(), roiRangeFor(periodInfo)),
        listarLocacoes(),
        listarInquilinos(true),
        listarImoveis(true),
        listarParcelas(),
      ])
      dashboardData = data
      leases = leaseItems || []
      tenants = tenantItems || []
      properties = propertyItems || []
      installments = installmentItems || []
      rois = data?.rois || []
      visibleCards = normalizeCards(getSetting('dashboard/visible_cards', DEFAULT_CARDS))
      renewalNoticeDays = Number(getSetting('dashboard/renewal_notice_days', 60)) || 60
      if (propertyId && !properties.some((item) => item.id === Number(propertyId))) propertyId = 0
      await loadSeriesAndRois(false)
    } catch (err) {
      loadError = errorMessage(err)
      dashboardData = null
      series = []
    } finally {
      loading = false
    }
  }

  async function loadSeriesAndRois(reloadDashboardRoi = true): Promise<void> {
    const periodInfo = resolvePeriod(range, customStart, customEnd)
    filtering = true
    try {
      const analyticsStart = periodInfo.allTime ? '2020-01-01' : periodInfo.start
      const analyticsEnd = periodInfo.allTime ? todayISO() : periodInfo.end
      const [result, updatedDash] = await Promise.all([
        analyticsSeries({
          inicio: analyticsStart,
          fim: analyticsEnd,
          eixo: 'tempo',
          series: 'tipo',
          metrica: 'ambos',
          granularidade: periodInfo.granularidade,
          imoveis: selectedProperty ? selectedProperty.id : '',
        }),
        reloadDashboardRoi
          ? loadDashboard(todayISO(), roiRangeFor(periodInfo))
          : Promise.resolve(null),
      ])
      if (updatedDash) {
        dashboardData = updatedDash
        rois = updatedDash.rois || []
      }
      const revenues = result?.series?.[0]?.valores_c || []
      const expenses = result?.series?.[1]?.valores_c || []
      series = (result?.categorias || []).map((category, index) => ({
        mes: category.chave,
        receitas_c: revenues[index] || 0,
        despesas_operacionais_c: expenses[index] || 0,
        resultado_operacional_c: (revenues[index] || 0) - (expenses[index] || 0),
      }))
    } finally {
      filtering = false
    }
  }

  function persistRange(): void {
    setSetting('dashboard/chart_range', range)
    setSetting('dashboard/chart_view', chartView)
    setSetting('dashboard/bar_type', stacked)
    setSetting('dashboard/chart_start', customStart)
    setSetting('dashboard/chart_end', customEnd)
    setSetting('dashboard/days_spin', days)
    setSetting('dashboard/property_id', selectedProperty ? selectedProperty.id : '')
  }

  let lastRange = range
  let lastProperty = propertyId
  $: if (range !== lastRange || Number(propertyId) !== lastProperty) {
    const rangeChanged = range !== lastRange
    lastRange = range
    lastProperty = Number(propertyId)
    persistRange()
    if (!loading) {
      loadSeriesAndRois(rangeChanged).catch((err) => notifyError(errorMessage(err)))
    }
  }

  function updateCustomDates(): void {
    const start = parseISO(customStart)
    const end = parseISO(customEnd)
    if (start && end) days = Math.max(1, Math.round((end.getTime() - start.getTime()) / 86400000) + 1)
  }

  function applyDays(): void {
    const end = parseISO(customEnd) || new Date()
    customStart = toISO(addDays(end, -(Math.max(1, Number(days) || 1) - 1)))
    persistRange()
    if (!loading) loadSeriesAndRois(true).catch((err) => notifyError(errorMessage(err)))
  }

  function applyCustom(): void {
    updateCustomDates()
    persistRange()
    if (!loading) loadSeriesAndRois(true).catch((err) => notifyError(errorMessage(err)))
  }

  function toggleStacked(): void {
    stacked = !stacked
    persistRange()
  }

  async function quitCharge(parcelaId: number): Promise<void> {
    if (quitting) return
    quitting = true
    try {
      const [simulation, parcela] = await Promise.all([
        simularParcela(parcelaId, todayISO()),
        buscarParcela(parcelaId),
      ])
      paymentDialog = { simulation, parcela }
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      quitting = false
    }
  }

  function paymentSaved(customMessage?: string): void {
    success(customMessage || 'Pagamento registrado com sucesso.')
    paymentDialog = null
    void loadAll()
  }

  function whatsapp(row: RenewalRow): void {
    const phone = String(row.tenant?.telefone || '').replace(/\D/g, '')
    if (!phone) return
    const digits = phone.length <= 11 && !phone.startsWith('55') ? `55${phone}` : phone
    const message = `Olá ${row.tenant?.nome || row.lease.inquilino}, tudo bem? Gostaria de conversar sobre a renovação do contrato de locação do imóvel ${row.lease.imovel}, que tem término previsto para ${dateBR(row.lease.fim)}.`
    void openExternal(`https://wa.me/${digits}?text=${encodeURIComponent(message)}`)
  }

  async function copyContact(row: RenewalRow): Promise<void> {
    const parts: string[] = []
    if (row.tenant?.telefone) parts.push(`Tel: ${row.tenant.telefone}`)
    if (row.tenant?.email) parts.push(`E-mail: ${row.tenant.email}`)
    const text = parts.length ? parts.join(' · ') : 'Sem contato cadastrado'
    await copyToClipboard(text)
    info(`Contato copiado para a área de transferência:\n${text}`)
  }

  function renew(row: RenewalRow): void {
    renewingLease = row.lease
  }

  function goToOpenInstallments(): void {
    setSetting('financeiro/tab', 'cobrancas')
    onNavigate('financeiro')
  }

  onMount(loadAll)

  let previousToken = refreshToken
  $: if (refreshToken !== previousToken) {
    previousToken = refreshToken
    void loadAll()
  }
</script>

<div class="page">
  <div class="page-header">
    <div>
      <h1 class="page-title">Visão geral</h1>
      <p class="page-subtitle">Indicadores consolidados de caixa, patrimônio, contratos e rentabilidade.</p>
    </div>
    <div class="flex">
      <button class="btn secondary" on:click={loadAll} disabled={loading}>
        <Icon name="refresh" size={15} /> Atualizar
      </button>
    </div>
  </div>

  {#if loadError}
    <div class="status-banner error">
      <Icon name="warning" size={16} />
      <span>Não foi possível carregar os indicadores: {loadError}</span>
      <button class="btn secondary small" style="margin-left:auto" on:click={loadAll}>Tentar novamente</button>
    </div>
  {/if}

  <div class="toolbar" style="margin-bottom:12px">
    <span class="toolbar-label">Período:</span>
    <select class="select" style="width:195px" bind:value={range} aria-label="Período">
      {#each DASHBOARD_PERIOD_OPTIONS as [value, label]}
        <option {value}>{label}</option>
      {/each}
    </select>

    {#if rangeIsCustom}
      <span class="toolbar-label">De:</span>
      <input class="input" type="date" bind:value={customStart} on:change={applyCustom} aria-label="Data inicial" />
      <span class="toolbar-label">Até:</span>
      <input class="input" type="date" bind:value={customEnd} on:change={applyCustom} aria-label="Data final" />
      <input class="input" type="number" min="1" max="3650" bind:value={days} on:change={applyDays} aria-label="Quantidade de dias" />
      <span class="helper">dias</span>
    {/if}

    <span class="toolbar-label">Imóvel:</span>
    <select class="select" style="width:190px" bind:value={propertyId} aria-label="Imóvel">
      <option value={0}>Todos os imóveis</option>
      {#each properties as property (property.id)}
        <option value={property.id}>{property.apelido}</option>
      {/each}
    </select>

    <span class="toolbar-label">Visão:</span>
    <select class="select" style="width:210px" bind:value={chartView} aria-label="Visão dos gráficos">
      <option value="both">Ambos lado a lado</option>
      <option value="income">Receitas × Despesas</option>
      <option value="roi">Rentabilidade por Imóvel (ROI)</option>
    </select>

    {#if chartView !== 'roi'}
      <span class="toolbar-label">Barras:</span>
      <button class="btn secondary small" on:click={toggleStacked}>{stacked ? 'Empilhadas' : 'Agrupadas'}</button>
    {/if}
    {#if filtering}<span class="spinner"></span>{/if}
  </div>

  {#if loading && !dashboardData}
    <div class="cards-grid">
      {#each Array(4) as _}<div class="card"><span class="card-label">Carregando…</span><span class="card-value">—</span></div>{/each}
    </div>
    <div class="empty-state">Carregando indicadores…</div>
  {:else}
    <div class="cards-grid">
      {#each cardEntries as [label, value, hint]}
        <div class="card" title={hint}>
          <span class="card-label">{label}</span>
          <span class="card-value">{value}</span>
          <span class="card-hint">{hint}</span>
        </div>
      {/each}
    </div>

    <h2 class="section-title">Desempenho financeiro e patrimonial</h2>
    <div class="chart-grid">
      {#if chartView !== 'roi'}
        <CashflowChart items={series} {stacked} subtitle={`Regime de caixa · ${currentPeriod.hint}`} />
      {/if}
      {#if chartView !== 'income'}
        <RoiChart
          items={filteredRois.length ? filteredRois : rois}
          periodLabel={currentPeriod.shortLabel}
          allTime={currentPeriod.allTime}
          selectedPropertyId={Number(propertyId) || 0}
          onSelectProperty={(id) => {
            propertyId = id
          }}
        />
      {/if}
    </div>

    <div class="split-two" style="margin-top:16px">
      <div>
        <div class="panel-header">
          <h2 class="section-title" style="margin:0">Cobranças em aberto recentes</h2>
          <button class="btn secondary small" on:click={goToOpenInstallments}>Ver todas as cobranças</button>
        </div>
        <DataTable
          columns={chargeColumns}
          rows={chargesRows}
          keyFor={(row) => row.item.id}
          empty="Nenhuma cobrança em aberto com saldo pendente."
          compact
          showToolbar={false}
          actions
        >
          <svelte:fragment slot="actions" let:row>
            <button class="btn small" disabled={quitting} on:click={() => quitCharge(row.item.id)}>Quitar</button>
          </svelte:fragment>
        </DataTable>
      </div>

      <div>
        <div class="panel-header">
          <h2 class="section-title" style="margin:0">Próximas renovações de contrato</h2>
        </div>
        <div class="table-wrap">
          <table class="data">
            <thead>
              <tr>
                <th>Urgência</th><th>Imóvel</th><th>Inquilino</th><th>Término</th><th>Ações</th>
              </tr>
            </thead>
            <tbody>
              {#if renewalRows.length === 0}
                <tr><td colspan="5" class="empty-row">Nenhum contrato com vencimento nos próximos {renewalNoticeDays} dias.</td></tr>
              {:else}
                {#each renewalRows as row (row.lease.id)}
                  <tr>
                    <td>
                      <span class="badge {row.days < 0 ? 'red' : row.days <= 30 ? 'amber' : 'blue'}">
                        {row.days < 0 ? `Venceu há ${Math.abs(row.days)}d` : `Vence em ${row.days}d`}
                      </span>
                    </td>
                    <td>{row.lease.imovel || '—'}</td>
                    <td>{row.lease.inquilino || '—'}</td>
                    <td class="nowrap">{dateBR(row.lease.fim)}</td>
                    <td>
                      <div class="actions-cell">
                        {#if row.tenant?.telefone}
                          <button class="btn icon ghost" title={`WhatsApp com ${row.tenant.nome}`} on:click={() => whatsapp(row)}>
                            <Icon name="whatsapp" size={16} />
                          </button>
                        {/if}
                        <button class="btn icon ghost" title="Copiar contato" on:click={() => copyContact(row)}>
                          <Icon name="copy" size={15} />
                        </button>
                        <button class="btn small secondary" on:click={() => renew(row)}>Renovar</button>
                      </div>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  {/if}
</div>

{#if paymentDialog}
  <PaymentDialog
    parcela={paymentDialog.parcela}
    simulation={paymentDialog.simulation}
    onClose={() => (paymentDialog = null)}
    onSaved={() => paymentSaved()}
  />
{/if}

{#if renewingLease}
  <LeaseEditDialog
    lease={renewingLease}
    mode="renew"
    onClose={() => (renewingLease = null)}
    onSaved={() => {
      renewingLease = null
      void loadAll()
    }}
  />
{/if}
