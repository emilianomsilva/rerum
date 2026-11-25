<script lang="ts">
  import { onMount } from 'svelte'
  import DataTable from '../components/DataTable.svelte'
  import Icon from '../components/Icon.svelte'
  import BatchPaymentDialog from '../dialogs/BatchPaymentDialog.svelte'
  import DebtDialog from '../dialogs/DebtDialog.svelte'
  import DebtMovementDialog from '../dialogs/DebtMovementDialog.svelte'
  import InstallmentDialog from '../dialogs/InstallmentDialog.svelte'
  import MovementDialog from '../dialogs/MovementDialog.svelte'
  import PaymentDialog from '../dialogs/PaymentDialog.svelte'
  import ProjectDialog from '../dialogs/ProjectDialog.svelte'
  import {
    buscarParcela,
    cancelarParcela,
    estornarParcela,
    excluirDivida,
    excluirMovimentacao,
    excluirMovimentoDivida,
    excluirParcela,
    listarDividas,
    listarEmpreendimentos,
    listarImoveis,
    listarInquilinos,
    listarLocacoes,
    listarMovimentacoes,
    listarParcelas,
    rateiosMovimentacao,
    salvarRecibo,
    saveTextFile,
    simularParcela,
  } from '../lib/api'
  import { confirmAction } from '../lib/confirm'
  import { csvContent, dateBR, money, periodLabel, situacaoParcela, todayISO } from '../lib/format'
  import { getSetting, setSetting } from '../lib/settings'
  import { error as notifyError, errorMessage, info, success } from '../lib/toast'
  import type {
    BatchPaymentItem,
    DebtSummary,
    Divida,
    Empreendimento,
    Imovel,
    Inquilino,
    LeaseSummary,
    Movimento,
    MovimentoDivida,
    Parcela,
    Rateio,
    Simulation,
    TableColumn,
  } from '../lib/types'

  export let refreshToken = 0

  type FinancialTab = 'movimentacoes' | 'cobrancas' | 'dividas'
  type CobStatus = 'todas' | 'a_receber' | 'vencidas' | 'a_vencer' | 'pagas' | 'canceladas'
  type MovNatureFilter = 'todas' | 'receita' | 'despesa_operacional' | 'investimento'

  let activeTab: FinancialTab = getSetting<FinancialTab>('financeiro/tab', 'cobrancas')
  let loading = true
  let loadError = ''
  let busy = false

  let properties: Imovel[] = []
  let projects: Empreendimento[] = []
  let tenants: Inquilino[] = []
  let leases: LeaseSummary[] = []
  let installments: Parcela[] = []
  let movements: Movimento[] = []
  let debts: DebtSummary[] = []

  // Cobranças filters
  let cobStatus: CobStatus = normalizeCobStatus(getSetting<string>('cobrancas/status', 'a_receber'))
  let cobPropertyId = Number(getSetting('cobrancas/property_id', 0)) || 0
  let cobSearch = getSetting<string>('cobrancas/search', '')
  let selectedIds: Array<string | number> = []

  // Movimentações filters
  let movNature: MovNatureFilter = getSetting<MovNatureFilter>('movimentacoes/nature', 'todas')
  let movPropertyId = Number(getSetting('movimentacoes/property_id', 0)) || 0
  let movProjectId = Number(getSetting('movimentacoes/project_id', 0)) || 0
  let movSearch = getSetting<string>('movimentacoes/search', '')

  // Dívidas master-detail selection
  let selectedDebtId: number | null = null

  // Dialogs
  let paymentDialog: { parcela: Parcela; simulation: Simulation } | null = null
  let batchItems: BatchPaymentItem[] | null = null
  let installmentDialog: { parcela: Parcela | null } | null = null
  let movementDialog: { movement: Movimento | null; rateios: Rateio[] } | null = null
  let projectDialogOpen = false
  let debtDialog: { debt: Divida | null } | null = null
  let debtMovDialog: { debtId: number } | null = null

  function normalizeCobStatus(val: string): CobStatus {
    if (val === 'abertas') return 'a_receber'
    if (
      val === 'todas' ||
      val === 'a_receber' ||
      val === 'vencidas' ||
      val === 'a_vencer' ||
      val === 'pagas' ||
      val === 'canceladas'
    ) {
      return val
    }
    return 'a_receber'
  }

  $: propertyMap = new Map(properties.map((p) => [p.id, p]))
  $: projectMap = new Map(projects.map((p) => [p.id, p]))
  $: tenantMap = new Map(tenants.map((t) => [t.id, t]))
  $: leaseMap = new Map(leases.map((l) => [l.id, l]))

  function setTab(tab: FinancialTab): void {
    activeTab = tab
    setSetting('financeiro/tab', tab)
  }

  async function refresh(): Promise<void> {
    loading = true
    loadError = ''
    try {
      const [p, proj, t, l, parc, mov, div] = await Promise.all([
        listarImoveis(true),
        listarEmpreendimentos(true),
        listarInquilinos(true),
        listarLocacoes(),
        listarParcelas({}),
        listarMovimentacoes({}),
        listarDividas(),
      ])
      properties = p || []
      projects = proj || []
      tenants = t || []
      leases = l || []
      installments = parc || []
      movements = mov || []
      debts = div || []
      selectedIds = []
      if (debts.length > 0 && (!selectedDebtId || !debts.some((d) => d.divida.id === selectedDebtId))) {
        selectedDebtId = debts[0].divida.id
      } else if (debts.length === 0) {
        selectedDebtId = null
      }
    } catch (err) {
      loadError = errorMessage(err)
    } finally {
      loading = false
    }
  }

  onMount(refresh)

  let previousToken = refreshToken
  $: if (refreshToken !== previousToken) {
    activeTab = getSetting<FinancialTab>('financeiro/tab', activeTab)
    previousToken = refreshToken
    void refresh()
  }

  function installmentSituation(r: Parcela): string {
    return r.situacao || situacaoParcela(r)
  }

  function installmentBalance(r: Parcela): number {
    if (r.status === 'paga' || r.status === 'cancelada') return 0
    if (typeof r.saldo_c === 'number') return Math.max(0, r.saldo_c)
    return Math.max(
      0,
      (r.valor_base_c || 0) + (r.multa_c || 0) + (r.juros_c || 0) - (r.desconto_c || 0) - (r.valor_pago_c || 0),
    )
  }

  $: visibleInstallments = installments.filter((r) => {
    const lease = leaseMap.get(r.locacao_id)
    if (cobPropertyId && lease?.imovel_id !== cobPropertyId) return false
    const sit = installmentSituation(r)
    if (cobStatus === 'a_receber' && sit !== 'aberta' && sit !== 'vencida' && sit !== 'parcial' && sit !== 'parcial-vencida') {
      return false
    }
    if (cobStatus === 'vencidas' && sit !== 'vencida' && sit !== 'parcial-vencida') return false
    if (cobStatus === 'a_vencer' && sit !== 'aberta' && sit !== 'parcial') return false
    if (cobStatus === 'pagas' && r.status !== 'paga') return false
    if (cobStatus === 'canceladas' && r.status !== 'cancelada') return false

    if (cobSearch.trim()) {
      const term = cobSearch.trim().toLocaleLowerCase('pt-BR')
      const prop = lease ? propertyMap.get(lease.imovel_id) : undefined
      const tenant = lease ? tenantMap.get(lease.inquilino_id) : undefined
      const blob = `${r.imovel || prop?.apelido || ''} ${r.inquilino || tenant?.nome || ''} ${r.competencia || ''} ${r.observacao || ''}`.toLocaleLowerCase('pt-BR')
      if (!blob.includes(term)) return false
    }
    return true
  })

  $: totalAReceberFiltrado = visibleInstallments.reduce((sum, r) => sum + installmentBalance(r), 0)
  $: totalVencidoFiltrado = visibleInstallments.reduce((sum, r) => {
    const sit = installmentSituation(r)
    return sit === 'vencida' || sit === 'parcial-vencida' ? sum + installmentBalance(r) : sum
  }, 0)

  $: visibleMovements = movements.filter((m) => {
    if (movNature === 'receita' && m.tipo !== 'receita') return false
    if (movNature === 'despesa_operacional' && !(m.tipo === 'despesa' && m.natureza !== 'investimento')) {
      return false
    }
    if (movNature === 'investimento' && !(m.tipo === 'despesa' && m.natureza === 'investimento')) {
      return false
    }
    if (movPropertyId && m.imovel_id !== movPropertyId) return false
    if (movProjectId && m.empreendimento_id !== movProjectId) return false
    if (movSearch.trim()) {
      const term = movSearch.trim().toLocaleLowerCase('pt-BR')
      const propName = m.imovel || (m.imovel_id ? propertyMap.get(m.imovel_id)?.apelido : '') || ''
      const projName = m.empreendimento_id ? projectMap.get(m.empreendimento_id)?.nome || '' : ''
      const tenName = m.inquilino || (m.inquilino_id ? tenantMap.get(m.inquilino_id)?.nome : '') || ''
      const blob = `${m.descricao || ''} ${m.categoria || ''} ${m.referencia || ''} ${m.fornecedor || ''} ${m.origem || ''} ${propName} ${projName} ${tenName}`.toLocaleLowerCase('pt-BR')
      if (!blob.includes(term)) return false
    }
    return true
  })

  // Movimentações KPI totals (respecting current filter, matching PySide MovimentacoesPage)
  $: movKpis = (() => {
    let receita = 0
    let opex = 0
    let capex = 0
    for (const m of visibleMovements) {
      const sign = m.reversao ? -1 : 1
      if (m.tipo === 'receita') receita += m.valor_c * sign
      else if (m.natureza === 'investimento') capex += m.valor_c * sign
      else opex += m.valor_c * sign
    }
    return {
      receita,
      opex,
      capex,
      liquido: receita - opex - capex,
    }
  })()

  // Dívidas KPI totals & selected debt detail (matching PySide DividasPage)
  $: debtKpis = (() => {
    let orig = 0
    let liberado = 0
    let amort = 0
    let juros = 0
    let saldo = 0
    for (const d of debts) {
      orig += d.divida.valor_original_c
      const addRel = (d.movimentos || [])
        .filter((m) => m.tipo === 'liberacao')
        .reduce((s, m) => s + m.valor_c, 0)
      liberado += d.divida.valor_original_c + addRel
      for (const m of d.movimentos || []) {
        if (m.tipo === 'pagamento') {
          juros += m.juros_c
          amort += m.valor_c - m.juros_c
        }
      }
      saldo += d.saldo_c
    }
    return { orig, liberado, amort, juros, saldo }
  })()

  $: selectedDebt = debts.find((d) => d.divida.id === selectedDebtId) || null

  let installmentColumns: TableColumn<Parcela>[] = []
  $: installmentColumns = [
    {
      key: 'numero',
      label: 'Parcela',
      width: 78,
      align: 'center',
      sortValue: (r) => r.numero || 0,
      text: (r) =>
        r.total_parcelas
          ? `${String(r.numero).padStart(2, '0')}/${String(r.total_parcelas).padStart(2, '0')}`
          : 'Extra',
    },
    {
      key: 'vencimento',
      label: 'Vencimento',
      width: 105,
      sortValue: (r) => r.vencimento,
      text: (r) => dateBR(r.vencimento),
    },
    {
      key: 'periodo',
      label: 'Período / Comp.',
      width: 165,
      sortValue: (r) => r.periodo_inicio || r.competencia,
      text: (r) =>
        r.periodo_inicio && r.periodo_fim
          ? periodLabel(r.periodo_inicio, r.periodo_fim)
          : r.competencia,
    },
    {
      key: 'imovel',
      label: 'Imóvel',
      width: 155,
      text: (r) => {
        const lease = leaseMap.get(r.locacao_id)
        return r.imovel || (lease ? propertyMap.get(lease.imovel_id)?.apelido : undefined) || '—'
      },
    },
    {
      key: 'inquilino',
      label: 'Inquilino',
      width: 195,
      text: (r) => {
        const lease = leaseMap.get(r.locacao_id)
        return r.inquilino || (lease ? tenantMap.get(lease.inquilino_id)?.nome : undefined) || '—'
      },
    },
    {
      key: 'total_devido_c',
      label: 'Devido',
      align: 'right',
      width: 112,
      value: (r) => r.total_devido_c ?? r.valor_base_c,
      text: (r) => money(r.total_devido_c ?? r.valor_base_c),
    },
    {
      key: 'saldo_c',
      label: 'Saldo / Pago',
      align: 'right',
      width: 118,
      value: (r) => (r.status === 'paga' ? r.valor_pago_c : installmentBalance(r)),
      text: (r) => (r.status === 'paga' ? money(r.valor_pago_c) : money(installmentBalance(r))),
    },
    {
      key: 'situacao',
      label: 'Situação',
      width: 115,
      sortValue: (r) => installmentSituation(r),
      badge: (r) => {
        const sit = installmentSituation(r)
        if (sit === 'vencida' || sit === 'parcial-vencida') {
          return { label: sit === 'parcial-vencida' ? 'Parcial venc.' : 'Vencida', tone: 'red' }
        }
        if (r.status === 'paga') return { label: 'Paga', tone: 'green' }
        if (r.status === 'parcial') return { label: 'Parcial', tone: 'amber' }
        if (r.status === 'cancelada') return { label: 'Cancelada', tone: 'gray' }
        return { label: 'Aberta', tone: 'blue' }
      },
    },
  ]

  let movementColumns: TableColumn<Movimento>[] = []
  $: movementColumns = [
    { key: 'data', label: 'Data', width: 100, sortValue: (r) => r.data, text: (r) => dateBR(r.data) },
    {
      key: 'tipo',
      label: 'Tipo',
      width: 90,
      text: (r) => (r.tipo === 'receita' ? 'Receita' : 'Despesa'),
      badge: (r) =>
        r.tipo === 'receita' ? { label: 'Receita', tone: 'green' } : { label: 'Despesa', tone: 'red' },
    },
    {
      key: 'natureza',
      label: 'Natureza',
      width: 105,
      text: (r) => (r.natureza === 'investimento' ? 'Investimento' : 'Operacional'),
    },
    { key: 'categoria', label: 'Categoria', width: 120, text: (r) => r.categoria || '—' },
    {
      key: 'alvo',
      label: 'Unidade / Empreend.',
      width: 165,
      text: (r) =>
        r.imovel ||
        (r.imovel_id ? propertyMap.get(r.imovel_id)?.apelido : undefined) ||
        (r.empreendimento_id ? projectMap.get(r.empreendimento_id)?.nome : undefined) ||
        'Geral',
    },
    {
      key: 'inquilino',
      label: 'Inquilino',
      width: 150,
      text: (r) =>
        r.inquilino ||
        (r.inquilino_id ? tenantMap.get(r.inquilino_id)?.nome : undefined) ||
        '—',
    },
    { key: 'referencia', label: 'Referência', width: 110, text: (r) => r.referencia || '—' },
    {
      key: 'valor_c',
      label: 'Valor',
      align: 'right',
      width: 115,
      value: (r) => (r.tipo === 'receita' ? r.valor_c : -r.valor_c),
      text: (r) => money(r.valor_c),
    },
    { key: 'forma_pagamento', label: 'Forma', width: 85, text: (r) => r.forma_pagamento || '—' },
    {
      key: 'fornecedor',
      label: 'Origem / Fornecedor',
      width: 140,
      text: (r) => r.fornecedor || r.origem || '—',
    },
    { key: 'descricao', label: 'Descrição', width: 220, text: (r) => r.descricao || '—' },
  ]

  const debtColumns: TableColumn<DebtSummary>[] = [
    { key: 'credor', label: 'Credor', width: 160, text: (r) => r.divida.credor },
    { key: 'descricao', label: 'Descrição', width: 200, text: (r) => r.divida.descricao || '—' },
    {
      key: 'data_contratacao',
      label: 'Contratação',
      width: 105,
      sortValue: (r) => r.divida.data_contratacao,
      text: (r) => dateBR(r.divida.data_contratacao),
    },
    {
      key: 'vencimento',
      label: 'Vencimento',
      width: 105,
      sortValue: (r) => r.divida.vencimento || '',
      text: (r) => (r.divida.vencimento ? dateBR(r.divida.vencimento) : '—'),
    },
    {
      key: 'liberado',
      label: 'Total liberado',
      align: 'right',
      width: 125,
      value: (r) =>
        r.divida.valor_original_c +
        (r.movimentos || []).filter((m) => m.tipo === 'liberacao').reduce((s, m) => s + m.valor_c, 0),
      text: (r) =>
        money(
          r.divida.valor_original_c +
            (r.movimentos || []).filter((m) => m.tipo === 'liberacao').reduce((s, m) => s + m.valor_c, 0),
        ),
    },
    {
      key: 'amortizado',
      label: 'Amortizado',
      align: 'right',
      width: 120,
      value: (r) =>
        (r.movimentos || []).filter((m) => m.tipo === 'pagamento').reduce((s, m) => s + (m.valor_c - m.juros_c), 0),
      text: (r) =>
        money(
          (r.movimentos || []).filter((m) => m.tipo === 'pagamento').reduce((s, m) => s + (m.valor_c - m.juros_c), 0),
        ),
    },
    {
      key: 'juros',
      label: 'Juros pagos',
      align: 'right',
      width: 115,
      value: (r) =>
        (r.movimentos || []).filter((m) => m.tipo === 'pagamento').reduce((s, m) => s + m.juros_c, 0),
      text: (r) =>
        money(
          (r.movimentos || []).filter((m) => m.tipo === 'pagamento').reduce((s, m) => s + m.juros_c, 0),
        ),
    },
    {
      key: 'saldo_c',
      label: 'Saldo devedor',
      align: 'right',
      width: 125,
      value: (r) => r.saldo_c,
      text: (r) => money(r.saldo_c),
    },
    {
      key: 'status',
      label: 'Status',
      width: 100,
      sortValue: (r) => (r.saldo_c === 0 ? 1 : 0),
      badge: (r) =>
        r.saldo_c === 0
          ? { label: 'Quitado', tone: 'green' }
          : { label: 'Em aberto', tone: 'amber' },
    },
  ]

  const debtMoveColumns: TableColumn<MovimentoDivida>[] = [
    { key: 'data', label: 'Data', width: 105, sortValue: (m) => m.data, text: (m) => dateBR(m.data) },
    {
      key: 'tipo',
      label: 'Tipo de operação',
      width: 170,
      badge: (m) =>
        m.tipo === 'pagamento'
          ? { label: 'Pagamento (Amortização)', tone: 'green' }
          : { label: 'Liberação (Aporte)', tone: 'blue' },
    },
    {
      key: 'valor_c',
      label: 'Valor total',
      align: 'right',
      width: 125,
      value: (m) => m.valor_c,
      text: (m) => money(m.valor_c),
    },
    {
      key: 'juros_c',
      label: 'Juros / Encargos',
      align: 'right',
      width: 125,
      value: (m) => (m.tipo === 'pagamento' ? m.juros_c : 0),
      text: (m) => (m.tipo === 'pagamento' ? money(m.juros_c) : '—'),
    },
    {
      key: 'amort_c',
      label: 'Amortização principal',
      align: 'right',
      width: 145,
      value: (m) => (m.tipo === 'pagamento' ? m.valor_c - m.juros_c : 0),
      text: (m) => (m.tipo === 'pagamento' ? money(m.valor_c - m.juros_c) : '—'),
    },
    { key: 'forma_pagamento', label: 'Forma', width: 95, text: (m) => m.forma_pagamento || '—' },
    { key: 'descricao', label: 'Descrição', width: 220, text: (m) => m.descricao || '—' },
  ]

  async function exportMovementsCsv(): Promise<void> {
    const headers = movementColumns.map((c) => c.label)
    const rows = visibleMovements.map((m) =>
      movementColumns.map((c) => (c.text ? String(c.text(m) ?? '') : String((m as unknown as Record<string, unknown>)[c.key] ?? ''))),
    )
    try {
      const saved = await saveTextFile('livro_financeiro.csv', csvContent(headers, rows))
      if (saved) success(`Livro financeiro exportado para: ${saved}`)
    } catch (err) {
      notifyError(errorMessage(err))
    }
  }

  async function openPayment(row: Parcela): Promise<void> {
    try {
      const [parcela, sim] = await Promise.all([
        buscarParcela(row.id),
        simularParcela(row.id, todayISO()),
      ])
      paymentDialog = { parcela: parcela || row, simulation: sim }
    } catch (err) {
      notifyError(errorMessage(err))
    }
  }

  async function openBatchPayment(): Promise<void> {
    const selected = installments.filter(
      (r) => selectedIds.includes(r.id) && (r.status === 'aberta' || r.status === 'parcial'),
    )
    if (selected.length === 0) {
      info('Selecione ao menos uma parcela em aberto ou parcial.')
      return
    }
    busy = true
    try {
      const sims = await Promise.all(selected.map((p) => simularParcela(p.id, todayISO())))
      batchItems = selected.map((parcela, index) => ({ parcela, simulation: sims[index] }))
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function handleRefund(row: Parcela): Promise<void> {
    const ok = await confirmAction({
      title: 'Estornar pagamento',
      message: `Deseja estornar o pagamento da parcela ${row.competencia}?`,
      confirmLabel: 'Estornar',
      danger: true,
    })
    if (!ok) return
    busy = true
    try {
      await estornarParcela(row.id)
      info('Pagamento estornado.')
      await refresh()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function handleCancelInstallment(row: Parcela): Promise<void> {
    const ok = await confirmAction({
      title: 'Cancelar parcela',
      message: `Deseja cancelar a parcela ${row.competencia}?`,
      confirmLabel: 'Cancelar parcela',
      danger: true,
    })
    if (!ok) return
    busy = true
    try {
      await cancelarParcela(row.id)
      info('Parcela cancelada.')
      await refresh()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function handleDeleteInstallment(row: Parcela): Promise<void> {
    const ok = await confirmAction({
      title: 'Excluir parcela',
      message: `Excluir permanentemente a parcela ${row.competencia}?`,
      confirmLabel: 'Excluir',
      danger: true,
    })
    if (!ok) return
    busy = true
    try {
      await excluirParcela(row.id)
      success('Parcela excluída.')
      await refresh()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function handleReceipt(row: Parcela): Promise<void> {
    try {
      const saved = await salvarRecibo(row.id)
      if (saved) success('Recibo salvo.')
    } catch (err) {
      notifyError(errorMessage(err))
    }
  }

  async function handleDeleteMovement(row: Movimento): Promise<void> {
    const ok = await confirmAction({
      title: 'Excluir movimentação',
      message: `Excluir "${row.descricao || row.categoria}" (${money(row.valor_c)})?`,
      confirmLabel: 'Excluir',
      danger: true,
    })
    if (!ok) return
    busy = true
    try {
      await excluirMovimentacao(row.id)
      success('Movimentação excluída.')
      await refresh()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function handleDeleteDebt(row: DebtSummary): Promise<void> {
    const ok = await confirmAction({
      title: 'Excluir dívida',
      message: `Excluir permanentemente "${row.divida.credor}" e todos os seus lançamentos?`,
      confirmLabel: 'Excluir',
      danger: true,
    })
    if (!ok) return
    busy = true
    try {
      await excluirDivida(row.divida.id)
      success('Dívida excluída.')
      await refresh()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function handleDeleteDebtMovement(mov: MovimentoDivida): Promise<void> {
    const ok = await confirmAction({
      title: 'Excluir movimento do débito',
      message: `Excluir permanentemente este lançamento de ${money(mov.valor_c)} em ${dateBR(mov.data)}?`,
      confirmLabel: 'Excluir',
      danger: true,
    })
    if (!ok) return
    busy = true
    try {
      await excluirMovimentoDivida(mov.id)
      success('Movimento excluído.')
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
      <h1 class="page-title">Central financeira</h1>
      <p class="page-subtitle">
        Gestão consolidada: cobranças de aluguéis, livro financeiro (receitas, despesas e capex) e débitos com credores.
      </p>
    </div>
    <div class="segmented" role="tablist">
      <button class:active={activeTab === 'cobrancas'} on:click={() => setTab('cobrancas')} type="button">
        Cobranças e Aluguéis
      </button>
      <button class:active={activeTab === 'movimentacoes'} on:click={() => setTab('movimentacoes')} type="button">
        Extrato e Livro financeiro
      </button>
      <button class:active={activeTab === 'dividas'} on:click={() => setTab('dividas')} type="button">
        Débitos e Financiamentos
      </button>
    </div>
  </div>

  {#if loadError}
    <div class="status-banner error">
      <Icon name="warning" size={16} /> {loadError}
      <button class="btn secondary small" style="margin-left:auto" on:click={refresh}>Tentar novamente</button>
    </div>
  {:else if loading && installments.length === 0 && movements.length === 0 && debts.length === 0}
    <div class="surface-card"><div class="empty">Carregando dados financeiros…</div></div>
  {:else if activeTab === 'cobrancas'}
    <div class="toolbar">
      <span class="toolbar-label">Situação:</span>
      <select
        class="select"
        style="width:165px"
        value={cobStatus}
        on:change={(e) => {
          cobStatus = e.currentTarget.value as CobStatus
          setSetting('cobrancas/status', cobStatus)
        }}
        aria-label="Situação das cobranças"
      >
        <option value="todas">Todas</option>
        <option value="a_receber">A receber</option>
        <option value="vencidas">Vencidas</option>
        <option value="a_vencer">A vencer</option>
        <option value="pagas">Pagas</option>
        <option value="canceladas">Canceladas</option>
      </select>

      <span class="toolbar-label">Imóvel:</span>
      <select
        class="select"
        style="width:190px"
        value={cobPropertyId}
        on:change={(e) => {
          cobPropertyId = Number(e.currentTarget.value)
          setSetting('cobrancas/property_id', cobPropertyId)
        }}
        aria-label="Filtrar cobranças por imóvel"
      >
        <option value={0}>Todos os imóveis</option>
        {#each properties as p (p.id)}
          <option value={p.id}>{p.apelido}</option>
        {/each}
      </select>

      <div class="search">
        <Icon name="search" />
        <input
          class="input"
          type="search"
          placeholder="Buscar inquilino, imóvel, competência…"
          value={cobSearch}
          on:input={(e) => {
            cobSearch = e.currentTarget.value
            setSetting('cobrancas/search', cobSearch)
          }}
          aria-label="Buscar cobrança"
        />
      </div>

      <span class="summary-pill" class:danger={totalVencidoFiltrado > 0}>
        A receber: {money(totalAReceberFiltrado)}
      </span>

      <button class="btn secondary" on:click={() => (installmentDialog = { parcela: null })} disabled={busy}>
        <Icon name="plus" size={15} /> Nova parcela
      </button>
      <button class="btn" on:click={openBatchPayment} disabled={busy || selectedIds.length === 0}>
        Quitar selecionadas{selectedIds.length > 0 ? ` (${selectedIds.length})` : ''}
      </button>
    </div>

    {#if busy}<div class="status-banner info"><span class="spinner"></span> Processando…</div>{/if}

    <DataTable
      rows={visibleInstallments}
      columns={installmentColumns}
      keyFor={(r) => r.id}
      {loading}
      settingsKey="cobrancas"
      exportFilename="cobrancas-rerum.csv"
      selectable={true}
      canSelect={(r) => r.status === 'aberta' || r.status === 'parcial'}
      rowClass={(r) => {
        const sit = installmentSituation(r)
        return sit === 'vencida' || sit === 'parcial-vencida' ? 'row-danger' : ''
      }}
      bind:selectedIds
      emptyText="Nenhuma cobrança encontrada."
      onRowDoubleClick={(r) => {
        if (r.status === 'aberta' || r.status === 'parcial') void openPayment(r)
      }}
      actions={[
        { label: 'Quitar', onClick: openPayment, when: (r) => r.status === 'aberta' || r.status === 'parcial' },
        { label: 'Recibo PDF', onClick: handleReceipt, when: (r) => Boolean(r.tem_recibo) || r.status === 'paga' || r.status === 'parcial' },
        { label: 'Editar', onClick: (r) => { installmentDialog = { parcela: r } }, when: (r) => r.status === 'aberta' },
        { label: 'Estornar', onClick: handleRefund, when: (r) => r.status === 'paga' || r.status === 'parcial', danger: true },
        { label: 'Cancelar', onClick: handleCancelInstallment, when: (r) => r.status === 'aberta', danger: true },
        { label: 'Excluir', onClick: handleDeleteInstallment, when: (r) => r.status === 'aberta', danger: true },
      ]}
    />
  {:else if activeTab === 'movimentacoes'}
    <div class="cards-grid">
      <div class="card">
        <span class="card-label">TOTAL RECEITAS</span>
        <span class="card-value">{money(movKpis.receita)}</span>
        <span class="card-hint">Aluguéis e receitas avulsas</span>
      </div>
      <div class="card">
        <span class="card-label">DESPESAS OPERACIONAIS</span>
        <span class="card-value">{money(movKpis.opex)}</span>
        <span class="card-hint">Custos de rotina dedutíveis</span>
      </div>
      <div class="card">
        <span class="card-label">INVESTIMENTOS / OBRAS</span>
        <span class="card-value">{money(movKpis.capex)}</span>
        <span class="card-hint">Reformas, projetos e capex</span>
      </div>
      <div class="card" class:positive={movKpis.liquido > 0} class:negative={movKpis.liquido < 0}>
        <span class="card-label">RESULTADO LÍQUIDO</span>
        <span class="card-value">{money(movKpis.liquido)}</span>
        <span class="card-hint">Receitas menos todas as saídas</span>
      </div>
    </div>

    <div class="toolbar">
      <span class="toolbar-label">Filtrar:</span>
      <select
        class="select"
        style="width:225px"
        value={movNature}
        on:change={(e) => {
          movNature = e.currentTarget.value as MovNatureFilter
          setSetting('movimentacoes/nature', movNature)
        }}
        aria-label="Filtrar natureza da movimentação"
      >
        <option value="todas">Todas as movimentações</option>
        <option value="receita">Receitas</option>
        <option value="despesa_operacional">Despesas operacionais (rotina)</option>
        <option value="investimento">Investimentos / Obras (capex)</option>
      </select>

      <select
        class="select"
        style="width:195px"
        value={movProjectId}
        on:change={(e) => {
          movProjectId = Number(e.currentTarget.value)
          setSetting('movimentacoes/project_id', movProjectId)
        }}
        aria-label="Filtrar movimentações por empreendimento"
      >
        <option value={0}>Todos os empreendimentos</option>
        {#each projects as pr (pr.id)}
          <option value={pr.id}>{pr.nome}</option>
        {/each}
      </select>

      <select
        class="select"
        style="width:185px"
        value={movPropertyId}
        on:change={(e) => {
          movPropertyId = Number(e.currentTarget.value)
          setSetting('movimentacoes/property_id', movPropertyId)
        }}
        aria-label="Filtrar movimentações por imóvel"
      >
        <option value={0}>Todos os imóveis</option>
        {#each properties as p (p.id)}
          <option value={p.id}>{p.apelido}</option>
        {/each}
      </select>

      <div class="search">
        <Icon name="search" />
        <input
          class="input"
          type="search"
          placeholder="Buscar categoria, descrição, referência, fornecedor…"
          value={movSearch}
          on:input={(e) => {
            movSearch = e.currentTarget.value
            setSetting('movimentacoes/search', movSearch)
          }}
          aria-label="Buscar movimentação"
        />
      </div>

      <button class="btn secondary" on:click={exportMovementsCsv} disabled={visibleMovements.length === 0}>
        <Icon name="download" size={14} /> Exportar CSV…
      </button>
      <button class="btn secondary" on:click={() => (projectDialogOpen = true)} disabled={busy}>
        + Empreendimento
      </button>
      <button class="btn" on:click={() => (movementDialog = { movement: null, rateios: [] })} disabled={busy}>
        <Icon name="plus" size={15} /> Nova movimentação
      </button>
    </div>

    {#if busy}<div class="status-banner info"><span class="spinner"></span> Processando…</div>{/if}

    <DataTable
      rows={visibleMovements}
      columns={movementColumns}
      keyFor={(r) => r.id}
      {loading}
      settingsKey="movimentacoes"
      exportFilename="livro_financeiro.csv"
      emptyText="Nenhuma movimentação encontrada."
      onRowDoubleClick={async (r) => {
        if (r.parcela_id) return
        const rateios = await rateiosMovimentacao(r.id).catch((): Rateio[] => [])
        movementDialog = { movement: r, rateios: rateios || [] }
      }}
      actions={[
        {
          label: 'Editar',
          when: (r) => !r.parcela_id,
          onClick: async (r) => {
            const rateios = await rateiosMovimentacao(r.id).catch((): Rateio[] => [])
            movementDialog = { movement: r, rateios: rateios || [] }
          },
        },
        { label: 'Excluir', when: (r) => !r.parcela_id, onClick: handleDeleteMovement, danger: true },
      ]}
    />
  {:else}
    <div class="cards-grid">
      <div class="card">
        <span class="card-label">VALOR ORIGINAL</span>
        <span class="card-value">{money(debtKpis.orig)}</span>
        <span class="card-hint">Soma dos contratos iniciais</span>
      </div>
      <div class="card">
        <span class="card-label">TOTAL LIBERADO</span>
        <span class="card-value">{money(debtKpis.liberado)}</span>
        <span class="card-hint">Original + aportes posteriores</span>
      </div>
      <div class="card">
        <span class="card-label">AMORTIZADO DO PRINCIPAL</span>
        <span class="card-value">{money(debtKpis.amort)}</span>
        <span class="card-hint">Baixas no saldo devedor</span>
      </div>
      <div class="card">
        <span class="card-label">JUROS PAGOS</span>
        <span class="card-value">{money(debtKpis.juros)}</span>
        <span class="card-hint">Encargos bancários acumulados</span>
      </div>
      <div class="card" class:negative={debtKpis.saldo > 0}>
        <span class="card-label">SALDO DEVEDOR ATUAL</span>
        <span class="card-value">{money(debtKpis.saldo)}</span>
        <span class="card-hint">Saldo principal em aberto</span>
      </div>
    </div>

    <div class="toolbar">
      <button class="btn" on:click={() => (debtDialog = { debt: null })} disabled={busy}>
        <Icon name="plus" size={15} /> Novo débito / empréstimo
      </button>
      <button
        class="btn secondary"
        disabled={busy || !selectedDebt}
        on:click={() => {
          if (selectedDebt) debtMovDialog = { debtId: selectedDebt.divida.id }
        }}
      >
        + Registrar pagamento / liberação…
      </button>
      {#if selectedDebt}
        <span class="helper" style="margin-left:8px">
          Débito selecionado: <strong>{selectedDebt.divida.credor}</strong>
        </span>
      {/if}
    </div>

    {#if busy}<div class="status-banner info"><span class="spinner"></span> Processando…</div>{/if}

    <h2 class="section-title tight">Débitos cadastrados</h2>
    <DataTable
      rows={debts}
      columns={debtColumns}
      keyFor={(r) => r.divida.id}
      {loading}
      maxHeight="310px"
      settingsKey="dividas"
      exportFilename="dividas-rerum.csv"
      emptyText="Nenhuma dívida cadastrada."
      rowClass={(r) => (r.divida.id === selectedDebtId ? 'selected' : '')}
      onRowClick={(r) => {
        selectedDebtId = r.divida.id
      }}
      onRowDoubleClick={(r) => {
        debtDialog = { debt: r.divida }
      }}
      actions={[
        { label: '+ Movimento', onClick: (r) => { selectedDebtId = r.divida.id; debtMovDialog = { debtId: r.divida.id } } },
        { label: 'Editar', onClick: (r) => { debtDialog = { debt: r.divida } } },
        { label: 'Excluir', onClick: handleDeleteDebt, danger: true },
      ]}
    />

    <h2 class="section-title">
      {#if selectedDebt}
        Extrato de movimentos: {selectedDebt.divida.credor}{selectedDebt.divida.descricao ? ` · ${selectedDebt.divida.descricao}` : ''}
      {:else}
        Extrato de movimentos (selecione um débito na tabela acima)
      {/if}
    </h2>
    <DataTable
      rows={selectedDebt?.movimentos || []}
      columns={debtMoveColumns}
      keyFor={(m) => m.id}
      maxHeight="290px"
      settingsKey="dividas_movimentacoes"
      exportFilename="extrato_divida.csv"
      emptyText={selectedDebt ? 'Nenhum lançamento neste débito.' : 'Nenhum débito selecionado.'}
      actions={[
        { label: 'Excluir', onClick: handleDeleteDebtMovement, danger: true },
      ]}
    />
  {/if}
</div>

{#if paymentDialog}
  <PaymentDialog
    parcela={paymentDialog.parcela}
    simulation={paymentDialog.simulation}
    onClose={() => (paymentDialog = null)}
    onSaved={() => {
      paymentDialog = null
      void refresh()
    }}
  />
{/if}

{#if batchItems}
  <BatchPaymentDialog
    items={batchItems}
    onClose={() => (batchItems = null)}
    onSaved={() => {
      batchItems = null
      void refresh()
    }}
  />
{/if}

{#if installmentDialog}
  <InstallmentDialog
    parcela={installmentDialog.parcela}
    {leases}
    onClose={() => (installmentDialog = null)}
    onSaved={() => {
      installmentDialog = null
      void refresh()
    }}
  />
{/if}

{#if movementDialog}
  <MovementDialog
    movement={movementDialog.movement}
    rateios={movementDialog.rateios}
    {properties}
    {projects}
    {tenants}
    onClose={() => (movementDialog = null)}
    onSaved={() => {
      movementDialog = null
      void refresh()
    }}
  />
{/if}

{#if projectDialogOpen}
  <ProjectDialog
    project={null}
    {properties}
    onClose={() => (projectDialogOpen = false)}
    onSaved={() => {
      projectDialogOpen = false
      void refresh()
    }}
  />
{/if}

{#if debtDialog}
  <DebtDialog
    debt={debtDialog.debt}
    onClose={() => (debtDialog = null)}
    onSaved={() => {
      debtDialog = null
      void refresh()
    }}
  />
{/if}

{#if debtMovDialog}
  <DebtMovementDialog
    debtId={debtMovDialog.debtId}
    onClose={() => (debtMovDialog = null)}
    onSaved={() => {
      debtMovDialog = null
      void refresh()
    }}
  />
{/if}
