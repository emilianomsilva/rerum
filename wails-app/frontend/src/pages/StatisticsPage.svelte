<script lang="ts">
  import { onMount } from 'svelte'
  import DataTable from '../components/DataTable.svelte'
  import Icon from '../components/Icon.svelte'
  import { copyToClipboard, estatisticas, relatorioAnual, saveTextFile } from '../lib/api'
  import { csvContent, money, parseISO, percent, toISO } from '../lib/format'
  import { ROI_PERIOD_OPTIONS, addDays, defaultCustomRange, resolvePeriod } from '../lib/periods'
  import { getSetting, setSetting } from '../lib/settings'
  import { error as notifyError, errorMessage, info, success } from '../lib/toast'
  import type {
    AnnualReport,
    AnnualReportMonth,
    AnnualReportTenantItem,
    Statistics,
    StatisticsLine,
    TableColumn,
  } from '../lib/types'

  export let refreshToken = 0

  type ReportTab = 'patrimonio' | 'irpf'

  const defaults = defaultCustomRange()
  let activeTab: ReportTab = getSetting<ReportTab>('estatisticas/tab', 'patrimonio')
  let loading = true
  let loadError = ''
  let onlyUnits = getSetting<boolean>('estatisticas/only_units', true) !== false
  let periodPreset = getSetting<string>('estatisticas/period', '12')
  let customStart = getSetting<string>('estatisticas/period_start', defaults.start)
  let customEnd = getSetting<string>('estatisticas/period_end', defaults.end)
  let days = Number(getSetting('estatisticas/period_days', defaults.days)) || defaults.days
  let year = Number(getSetting('estatisticas/year', new Date().getFullYear())) || new Date().getFullYear()
  let stats: Statistics | null = null
  let annual: AnnualReport | null = null

  $: resolvedPeriod = resolvePeriod(periodPreset, customStart, customEnd)

  function setTab(tab: ReportTab): void {
    activeTab = tab
    setSetting('estatisticas/tab', tab)
  }

  async function refresh(): Promise<void> {
    loading = true
    loadError = ''
    try {
      const pInfo = resolvePeriod(periodPreset, customStart, customEnd)
      const rangeParam = pInfo.allTime ? undefined : { inicio: pInfo.start, fim: pInfo.end }
      const [s, a] = await Promise.all([estatisticas(onlyUnits, rangeParam), relatorioAnual(year)])
      stats = s
      annual = a
    } catch (err) {
      loadError = errorMessage(err)
    } finally {
      loading = false
    }
  }

  async function loadPortfolioOnly(): Promise<void> {
    loading = true
    loadError = ''
    try {
      const pInfo = resolvePeriod(periodPreset, customStart, customEnd)
      const rangeParam = pInfo.allTime ? undefined : { inicio: pInfo.start, fim: pInfo.end }
      stats = await estatisticas(onlyUnits, rangeParam)
    } catch (err) {
      loadError = errorMessage(err)
    } finally {
      loading = false
    }
  }

  async function loadYearOnly(): Promise<void> {
    loading = true
    loadError = ''
    try {
      annual = await relatorioAnual(year)
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

  function changeScope(val: string): void {
    onlyUnits = val === 'unidades'
    setSetting('estatisticas/only_units', onlyUnits)
    void loadPortfolioOnly()
  }

  function changePeriod(val: string): void {
    periodPreset = val
    setSetting('estatisticas/period', val)
    void loadPortfolioOnly()
  }

  function applyCustomDates(): void {
    const s = parseISO(customStart)
    const e = parseISO(customEnd)
    if (s && e) days = Math.max(1, Math.round((e.getTime() - s.getTime()) / 86400000) + 1)
    setSetting('estatisticas/period_start', customStart)
    setSetting('estatisticas/period_end', customEnd)
    setSetting('estatisticas/period_days', days)
    void loadPortfolioOnly()
  }

  function applyCustomDays(): void {
    const end = parseISO(customEnd) || new Date()
    customStart = toISO(addDays(end, -(Math.max(1, Number(days) || 1) - 1)))
    setSetting('estatisticas/period_start', customStart)
    setSetting('estatisticas/period_end', customEnd)
    setSetting('estatisticas/period_days', days)
    void loadPortfolioOnly()
  }

  function changeYear(val: string): void {
    const parsed = Number(val)
    if (parsed >= 2000 && parsed <= 2100) {
      year = parsed
      setSetting('estatisticas/year', year)
      void loadYearOnly()
    }
  }

  let propertyColumns: TableColumn<StatisticsLine>[] = []
  $: propertyColumns = [
    { key: 'apelido', label: 'Imóvel / Unidade', width: 170 },
    {
      key: 'destinado_locacao',
      label: 'Destinado à',
      width: 110,
      text: (r) => (r.destinado_locacao ? 'Locação' : 'Patrimônio'),
      badge: (r) =>
        r.destinado_locacao
          ? { label: 'Locação', tone: 'blue' }
          : { label: 'Patrimônio', tone: 'gray' },
    },
    {
      key: 'custo_c',
      label: 'Capital investido',
      align: 'right',
      width: 135,
      value: (r) => r.custo_c,
      text: (r) => money(r.custo_c),
    },
    {
      key: 'rendimento_c',
      label: `Receitas (${resolvedPeriod.shortLabel})`,
      align: 'right',
      width: 140,
      value: (r) => r.rendimento_c,
      text: (r) => money(r.rendimento_c),
    },
    {
      key: 'despesas_operacionais_c',
      label: `Despesas op. (${resolvedPeriod.shortLabel})`,
      align: 'right',
      width: 150,
      value: (r) => r.despesas_operacionais_c,
      text: (r) => money(r.despesas_operacionais_c),
    },
    {
      key: 'investimentos_c',
      label: 'Investimentos / Obras',
      align: 'right',
      width: 145,
      value: (r) => r.investimentos_c,
      text: (r) => money(r.investimentos_c),
    },
    {
      key: 'resultado_liquido_c',
      label: 'Resultado líquido',
      align: 'right',
      width: 135,
      value: (r) => r.resultado_liquido_c,
      text: (r) => money(r.resultado_liquido_c),
    },
    {
      key: 'roi_pct',
      label: `ROI bruto (${resolvedPeriod.shortLabel})`,
      align: 'right',
      width: 120,
      value: (r) => (r.custo_c > 0 ? r.roi_pct : -999999),
      text: (r) => (r.custo_c > 0 ? percent(r.roi_pct) : '—'),
    },
    {
      key: 'roi_liquido_pct',
      label: `ROI líquido (${resolvedPeriod.shortLabel})`,
      align: 'right',
      width: 125,
      value: (r) => (r.custo_c > 0 ? r.roi_liquido_pct : -999999),
      text: (r) => (r.custo_c > 0 ? percent(r.roi_liquido_pct) : '—'),
    },
  ]

  $: portfolioFooter = stats
    ? {
        apelido: 'TOTAL CONSOLIDADO',
        destinado_locacao: '—',
        custo_c: money(stats.custo_total_c),
        rendimento_c: money(stats.rendimento_total_c),
        despesas_operacionais_c: money(stats.despesas_operacionais_total_c),
        investimentos_c: money(stats.investimentos_total_c),
        resultado_liquido_c: money(stats.resultado_liquido_total_c),
        roi_pct: stats.custo_total_c > 0 ? percent(stats.roi_total_pct) : '—',
        roi_liquido_pct: stats.custo_total_c > 0 ? percent(stats.roi_liquido_total_pct) : '—',
      }
    : null

  const monthColumns: TableColumn<AnnualReportMonth>[] = [
    {
      key: 'mes',
      label: 'Mês',
      width: 95,
      sortValue: (r) => r.mes,
      text: (r) => `${r.mes.slice(5, 7)}/${r.mes.slice(0, 4)}`,
    },
    {
      key: 'receitas_c',
      label: 'Receita bruta',
      align: 'right',
      width: 125,
      value: (r) => r.receitas_c,
      text: (r) => money(r.receitas_c),
    },
    {
      key: 'despesas_operacionais_c',
      label: 'Despesas operacionais',
      align: 'right',
      width: 145,
      value: (r) => r.despesas_operacionais_c,
      text: (r) => money(r.despesas_operacionais_c),
    },
    {
      key: 'investimentos_c',
      label: 'Investimentos / Obras',
      align: 'right',
      width: 145,
      value: (r) => r.investimentos_c,
      text: (r) => money(r.investimentos_c),
    },
    {
      key: 'resultado_liquido_c',
      label: 'Resultado líquido',
      align: 'right',
      width: 135,
      value: (r) => r.resultado_liquido_c,
      text: (r) => money(r.resultado_liquido_c),
    },
  ]

  $: annualFooter = annual
    ? {
        mes: 'TOTAL DO ANO',
        receitas_c: money(annual.total_receitas_c),
        despesas_operacionais_c: money(annual.total_despesas_opex_c),
        investimentos_c: money(annual.total_investimentos_c),
        resultado_liquido_c: money(annual.resultado_liquido_c),
      }
    : null

  const tenantColumns: TableColumn<AnnualReportTenantItem>[] = [
    { key: 'inquilino_nome', label: 'Inquilino', width: 220 },
    { key: 'inquilino_cpf', label: 'CPF', width: 130, text: (r) => r.inquilino_cpf || '—' },
    { key: 'imovel_apelido', label: 'Imóvel', width: 125 },
    {
      key: 'total_recebido_c',
      label: 'Total anual recebido',
      align: 'right',
      width: 140,
      value: (r) => r.total_recebido_c,
      text: (r) => money(r.total_recebido_c),
    },
  ]

  async function exportPortfolioCsv(): Promise<void> {
    if (!stats) return
    const headers = propertyColumns.map((c) => c.label)
    const rows = (stats.linhas || []).map((line) =>
      propertyColumns.map((c) => (c.text ? String(c.text(line) ?? '') : String((line as unknown as Record<string, unknown>)[c.key] ?? ''))),
    )
    if (portfolioFooter) {
      rows.push(propertyColumns.map((c) => String((portfolioFooter as Record<string, string>)[c.key] ?? '')))
    }
    try {
      const saved = await saveTextFile('estatisticas-rerum.csv', csvContent(headers, rows))
      if (saved) success(`Estatísticas exportadas para: ${saved}`)
    } catch (err) {
      notifyError(errorMessage(err))
    }
  }

  async function copyIrpfRow(row: AnnualReportTenantItem): Promise<void> {
    const line = `${row.inquilino_nome} | CPF: ${row.inquilino_cpf || 'não informado'} | Imóvel: ${row.imovel_apelido} | Total ${year}: ${money(row.total_recebido_c)}`
    await copyToClipboard(line)
    info(`Copiado para declaração:\n${line}`)
  }
</script>

<div class="page">
  <div class="page-header">
    <div>
      <h1 class="page-title">Relatórios e IRPF</h1>
      <p class="page-subtitle">
        Análise patrimonial consolidada, retorno sobre investimento por período e relatório para declaração do imposto de renda.
      </p>
    </div>
    <div class="segmented" role="tablist">
      <button class:active={activeTab === 'patrimonio'} on:click={() => setTab('patrimonio')} type="button">
        Patrimônio e rentabilidade
      </button>
      <button class:active={activeTab === 'irpf'} on:click={() => setTab('irpf')} type="button">
        Informe anual / IRPF
      </button>
    </div>
  </div>

  {#if loadError}
    <div class="status-banner error">
      <Icon name="warning" size={16} /> {loadError}
      <button class="btn secondary small" style="margin-left:auto" on:click={refresh}>Tentar novamente</button>
    </div>
  {:else if loading && !stats && !annual}
    <div class="surface-card"><div class="empty">Carregando estatísticas patrimoniais…</div></div>
  {:else if activeTab === 'patrimonio'}
    {#if stats}
      <div class="cards-grid">
        <div class="card">
          <span class="card-label">CAPITAL INVESTIDO</span>
          <span class="card-value">{money(stats.custo_total_c)}</span>
          <span class="card-hint">Base de investimento no patrimônio</span>
        </div>
        <div class="card">
          <span class="card-label">{resolvedPeriod.receiptTitle}</span>
          <span class="card-value">{money(stats.rendimento_total_c)}</span>
          <span class="card-hint">{resolvedPeriod.hint}</span>
        </div>
        <div class="card">
          <span class="card-label">{resolvedPeriod.expenseTitle}</span>
          <span class="card-value">{money(stats.despesas_operacionais_total_c)}</span>
          <span class="card-hint">Custos operacionais de rotina</span>
        </div>
        <div class="card" class:positive={stats.resultado_liquido_total_c > 0} class:negative={stats.resultado_liquido_total_c < 0}>
          <span class="card-label">RESULTADO LÍQUIDO</span>
          <span class="card-value">{money(stats.resultado_liquido_total_c)}</span>
          <span class="card-hint">Receitas menos despesas no período</span>
        </div>
        <div class="card">
          <span class="card-label">ROI BRUTO ({resolvedPeriod.shortLabel.toUpperCase()})</span>
          <span class="card-value">{stats.custo_total_c > 0 ? percent(stats.roi_total_pct) : '—'}</span>
          <span class="card-hint">Receita ÷ Capital investido</span>
        </div>
        <div class="card">
          <span class="card-label">ROI LÍQUIDO ({resolvedPeriod.shortLabel.toUpperCase()})</span>
          <span class="card-value">{stats.custo_total_c > 0 ? percent(stats.roi_liquido_total_pct) : '—'}</span>
          <span class="card-hint">Resultado líquido ÷ Capital</span>
        </div>
      </div>

      <div class="toolbar">
        <span class="toolbar-label">Período:</span>
        <select
          class="select"
          style="width:210px"
          value={periodPreset}
          on:change={(e) => changePeriod(e.currentTarget.value)}
          aria-label="Período de apuração do ROI"
        >
          {#each ROI_PERIOD_OPTIONS as [value, label]}
            <option {value}>{label}</option>
          {/each}
        </select>

        {#if periodPreset === 'custom'}
          <span class="toolbar-label">De:</span>
          <input class="input" type="date" bind:value={customStart} on:change={applyCustomDates} aria-label="Data inicial" />
          <span class="toolbar-label">Até:</span>
          <input class="input" type="date" bind:value={customEnd} on:change={applyCustomDates} aria-label="Data final" />
          <input class="input" type="number" min="1" max="3650" bind:value={days} on:change={applyCustomDays} aria-label="Dias" />
          <span class="helper">dias</span>
        {/if}

        <span class="toolbar-label">Escopo:</span>
        <select
          class="select"
          style="width:255px"
          value={onlyUnits ? 'unidades' : 'todos'}
          on:change={(e) => changeScope(e.currentTarget.value)}
          aria-label="Escopo patrimonial"
        >
          <option value="unidades">Unidades destinadas à locação</option>
          <option value="todos">Todos os imóveis do patrimônio</option>
        </select>

        <div class="spacer"></div>
        <button class="btn secondary" on:click={exportPortfolioCsv}>
          <Icon name="download" size={14} /> Exportar CSV…
        </button>
      </div>

      <h2 class="section-title tight">Detalhamento por imóvel · {resolvedPeriod.hint}</h2>
      <DataTable
        rows={stats.linhas || []}
        columns={propertyColumns}
        keyFor={(r) => r.imovel_id}
        footer={portfolioFooter}
        settingsKey="estatisticas_detalhamento"
        exportFilename="estatisticas-rerum.csv"
        emptyText="Nenhum imóvel no escopo selecionado."
      />
    {/if}
  {:else}
    <div class="toolbar">
      <span class="toolbar-label">Ano-calendário:</span>
      <input
        class="input"
        type="number"
        min="2000"
        max="2100"
        value={year}
        on:change={(e) => changeYear(e.currentTarget.value)}
        aria-label="Ano-calendário"
      />
      <button class="btn secondary" on:click={loadYearOnly} disabled={loading}>
        <Icon name="refresh" size={14} /> Carregar ano
      </button>
      {#if annual}
        <span class="summary-pill">
          Receitas {year}: {money(annual.total_receitas_c)} · Líquido: {money(annual.resultado_liquido_c)}
        </span>
      {/if}
    </div>

    <div class="split-two" style="grid-template-columns: minmax(420px, 1.25fr) minmax(380px, 1fr)">
      <div>
        <h2 class="section-title tight">Consolidação mensal (Carnê-Leão: Receitas vs Despesas dedutíveis)</h2>
        <DataTable
          rows={annual?.meses || []}
          columns={monthColumns}
          keyFor={(r) => r.mes}
          footer={annualFooter}
          maxHeight="540px"
          settingsKey="estatisticas_mensal"
          exportFilename={`carne_leao_${year}.csv`}
          emptyText="Sem lançamentos para o ano selecionado."
        />
      </div>

      <div>
        <h2 class="section-title tight">Informe de rendimentos por inquilino (CPF)</h2>
        <DataTable
          rows={annual?.inquilinos || []}
          columns={tenantColumns}
          keyFor={(r) => `${r.inquilino_nome}|${r.imovel_apelido}`}
          maxHeight="540px"
          settingsKey="estatisticas_inquilinos"
          exportFilename={`irpf_inquilinos_${year}.csv`}
          emptyText="Sem recebimentos registrados neste ano."
          actions={[
            { label: 'Copiar', onClick: copyIrpfRow },
          ]}
        />
      </div>
    </div>
  {/if}
</div>
