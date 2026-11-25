// Strict TypeScript mirror of backend/pkg/domain, backend/pkg/storage/sqlite,
// backend/pkg/importer, and backend/pkg/httpapi JSON contracts.

export type PageRoute =
  | 'dashboard'
  | 'empreendimentos'
  | 'imoveis'
  | 'inquilinos'
  | 'locacoes'
  | 'financeiro'
  | 'estatisticas'
  | 'configuracoes'

export type IconName =
  | 'dashboard'
  | 'projects'
  | 'properties'
  | 'tenants'
  | 'leases'
  | 'ledger'
  | 'statistics'
  | 'settings'
  | 'plus'
  | 'search'
  | 'close'
  | 'cash'
  | 'debts'
  | 'download'
  | 'upload'
  | 'refresh'
  | 'check'
  | 'edit'
  | 'archive'
  | 'trash'
  | 'whatsapp'
  | 'copy'
  | 'fileName'
  | 'chart'
  | 'arrowUp'
  | 'arrowDown'
  | 'info'
  | 'warning'
  | 'dot'

export type BadgeTone = 'green' | 'amber' | 'red' | 'blue' | 'gray'

export interface BadgeSpec {
  text?: string
  label?: string
  tone?: BadgeTone
}

export interface TableColumn<T> {
  key: string
  label: string
  width?: number | string
  align?: 'left' | 'right' | 'center'
  sortable?: boolean
  title?: string
  value?: (row: T) => string | number | boolean | null | undefined
  text?: (row: T) => string | number | null | undefined
  sortValue?: (row: T) => string | number
  badge?: (row: T) => BadgeSpec | null
  html?: (row: T) => string
  class?: (row: T) => string
}

export interface TableAction<T> {
  label: string
  onClick: (row: T) => void | Promise<void>
  when?: (row: T) => boolean
  danger?: boolean
}

export type PropertyStatus = 'disponivel' | 'manutencao' | 'ocupado' | string
export type PropertySituacao = 'proprio' | 'vendido' | string
export type PropertyKind = 'casa' | 'apartamento' | 'sala' | 'galpao' | 'lote' | 'outros' | string

export interface Imovel {
  id: number
  apelido: string
  endereco: string
  tipo: PropertyKind
  status: PropertyStatus
  valor_compra_c: number
  reforma_c: number
  itbi_c: number
  corretagem_c: number
  outros_custos_c: number
  data_aquisicao: string | null
  observacao: string
  ativo: boolean
  matricula_cartorio: string
  inscricao_iptu: string
  unidade_agua: string
  unidade_energia: string
  empreendimento_id: number | null
  bairro: string
  situacao_patrimonial: PropertySituacao
  investimento_importado_c: number
  investimento_total_c: number
  destinado_locacao: boolean
  matricula_compartilhada: boolean
  data_venda: string | null
  valor_venda_c: number
  numero: string
  unidade: string
}

export type ImovelInput = Omit<Imovel, 'investimento_total_c'> & {
  investimento_total_c?: number
}

export interface PropertyBatch {
  imoveis: Array<Partial<Imovel> & Pick<Imovel, 'apelido' | 'status'>>
  modo_custo: 'dividir' | 'fixo'
  valor_compra_c: number
  reforma_c: number
  outros_custos_c: number
}

export interface Inquilino {
  id: number
  nome: string
  cpf: string
  telefone: string
  email: string
  observacao: string
  ativo: boolean
  rg: string
  renda_mensal_c: number
  local_trabalho: string
  endereco_atual: string
}

export type LeaseStatus = 'ativa' | 'encerrada' | string

export interface Locacao {
  id: number
  imovel_id: number
  inquilino_id: number
  inicio: string
  fim: string
  dia_vencimento: number
  valor_mensal_c: number
  multa_pct: number
  juros_dia_pct: number
  reajuste_indice: string
  status: LeaseStatus
  intermediario: string
  observacao: string
  primeiro_vencimento: string | null
}

export type LocacaoCreateInput = Omit<Locacao, 'id' | 'status'> & {
  id?: number
  status?: LeaseStatus
}

export interface LocacaoUpdateInput {
  intermediario?: string
  observacao?: string
  dia_vencimento?: number
  valor_mensal_c?: number
  inicio?: string
  fim?: string
  multa_pct?: number
  juros_dia_pct?: number
  reajuste_indice?: string
}

export interface LocacaoRenewInput {
  novo_fim: string
  novo_valor_mensal_c: number
}

export interface LeaseSummary extends Locacao {
  imovel: string
  inquilino: string
  parcelas: number
  periodo_inicio: string | null
  periodo_fim: string | null
}

export interface CreateLeaseResponse {
  locacao: Locacao
  parcelas: Parcela[]
  parcelas_geradas: number
}

export interface RenewLeaseResponse {
  locacao: Locacao
  parcelas: Parcela[]
  novas_parcelas: number
}

export type ParcelaStatus = 'aberta' | 'parcial' | 'paga' | 'cancelada' | string
export type ParcelaSituacao =
  | 'aberta'
  | 'vencida'
  | 'parcial'
  | 'parcial-vencida'
  | 'paga'
  | 'cancelada'
  | string

export interface Parcela {
  id: number
  locacao_id: number
  competencia: string
  vencimento: string
  valor_base_c: number
  multa_c: number
  juros_c: number
  desconto_c: number
  valor_pago_c: number
  data_pagamento: string | null
  forma: string | null
  status: ParcelaStatus
  recibo_path: string | null
  numero: number
  total_parcelas: number
  periodo_inicio?: string | null
  periodo_fim?: string | null
  tipo: string
  observacao: string
  imovel?: string
  inquilino?: string
  total_devido_c?: number
  saldo_c?: number
  situacao?: ParcelaSituacao
  tem_recibo?: boolean
}

export interface InstallmentUpdate {
  vencimento?: string
  valor_base_c?: number
  multa_c?: number
  juros_c?: number
  desconto_c?: number
  tipo?: string
  observacao?: string
}

export interface InstallmentPayment {
  parcela_id?: number
  data: string
  forma: string
  desconto_c?: number | null
  valor_pago_c?: number | null
  multa_c?: number | null
  juros_c?: number | null
}

export interface Simulation {
  base_c: number
  multa_c: number
  juros_c: number
  desconto_c: number
  total_c: number
  dias_atraso: number
}

export type MovimentoTipo = 'receita' | 'despesa' | string
export type MovimentoNatureza = 'operacional' | 'investimento' | string

export interface Movimento {
  id: number
  tipo: MovimentoTipo
  categoria: string
  data: string
  valor_c: number
  descricao: string
  imovel_id: number | null
  empreendimento_id: number | null
  inquilino_id: number | null
  locacao_id?: number | null
  parcela_id?: number | null
  referencia: string
  forma_pagamento: string
  origem: string
  fornecedor: string
  observacao?: string
  reversao: boolean
  natureza: MovimentoNatureza
  imovel?: string
  inquilino?: string
}

export interface Rateio {
  imovel_id: number
  valor_c: number
}

export interface Empreendimento {
  id: number
  nome: string
  endereco: string
  observacao: string
  ativo: boolean
  unidade_ids: number[]
}

export interface Divida {
  id: number
  credor: string
  descricao: string
  valor_original_c: number
  data_contratacao: string
  vencimento: string | null
  observacao: string
  ativa: boolean
}

export type DebtMovementKind = 'pagamento' | 'liberacao' | string

export interface MovimentoDivida {
  id: number
  divida_id: number
  tipo: DebtMovementKind
  data: string
  valor_c: number
  juros_c: number
  forma_pagamento: string
  descricao: string
}

export interface DebtSummary {
  divida: Divida
  saldo_c: number
  movimentos: MovimentoDivida[]
}

export interface Roi {
  imovel_id: number
  apelido: string
  investimento_c: number
  recebido_total_c: number
  despesas_total_c: number
  recebido_12m_c: number
  despesas_12m_c: number
  roi_anual_pct: number
  roi_acumulado_pct: number
}

export interface DashboardPayment {
  id: number
  numero: number
  total_parcelas: number
  vencimento: string
  imovel: string
  inquilino: string
  saldo_c: number
}

export interface MonthlyCashflow {
  mes: string
  receitas_c: number
  despesas_operacionais_c: number
  resultado_operacional_c: number
  receitas_sem_detalhamento_c?: number
}

export interface Dashboard {
  serie_mensal: MonthlyCashflow[]
  total_imoveis: number
  ocupados: number
  disponiveis: number
  manutencao: number
  taxa_ocupacao_pct: number
  receita_mes_c: number
  a_receber_mes_c: number
  despesas_operacionais_mes_c: number
  resultado_operacional_mes_c: number
  capital_investido_c: number
  roi_carteira_12m_pct: number | null
  vencidas_qtd: number
  vencidas_c: number
  vencidas: DashboardPayment[]
  proximas: DashboardPayment[]
  rois: Roi[]
}

export interface StatisticsLine {
  investimentos_c: number
  imovel_id: number
  apelido: string
  custo_c: number
  rendimento_c: number
  despesas_operacionais_c: number
  resultado_liquido_c: number
  roi_pct: number
  roi_liquido_pct: number
  destinado_locacao: boolean
}

export interface Statistics {
  investimentos_total_c: number
  linhas: StatisticsLine[]
  custo_total_c: number
  rendimento_total_c: number
  despesas_operacionais_total_c: number
  resultado_liquido_total_c: number
  roi_total_pct: number
  roi_liquido_total_pct: number
}

export interface AnnualReportMonth {
  mes: string
  receitas_c: number
  despesas_operacionais_c: number
  investimentos_c: number
  resultado_liquido_c: number
}

export interface AnnualReportTenantItem {
  inquilino_nome: string
  inquilino_cpf: string
  imovel_apelido: string
  total_recebido_c: number
}

export interface AnnualReport {
  ano: number
  meses: AnnualReportMonth[]
  inquilinos: AnnualReportTenantItem[]
  total_receitas_c: number
  total_despesas_opex_c: number
  total_investimentos_c: number
  resultado_liquido_c: number
}

export interface AnalyticsCategory {
  chave: string
  rotulo: string
}

export interface AnalyticsSeries {
  chave: string
  rotulo: string
  valores_c: number[]
}

export interface AnalyticsResult {
  categorias: AnalyticsCategory[]
  series: AnalyticsSeries[]
  avisos: string[]
}

export interface AnalyticsParams {
  inicio?: string
  fim?: string
  eixo?: string
  series?: string
  metrica?: string
  granularidade?: string
  imoveis?: string | number
}

export interface ImportPreview {
  source: string
  sha256: string
  sheets: string[]
  properties: number
  receipts: number
  receipts_total_c: number
  expenses: number
  expenses_total_c: number
  debts: number
  debt_received_c: number
  debt_paid_c: number
  formula_rows: number
  warnings: string[]
  already_imported: boolean
}

export interface ImportExecuteResponse {
  preview: ImportPreview
  backup: string
}

export interface BackendDiagnostics {
  base_url: string
  db_path: string
  managed: boolean
  root?: string
}

export interface CascadeDeleteResponse {
  backup_path: string
}

export type EditableLease = Locacao & { imovel?: string; inquilino?: string }

export interface BatchPaymentItem {
  parcela: Parcela
  simulation: Simulation
}
