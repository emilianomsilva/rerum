// Strictly typed client for the in-process Rerum Go backend. The Svelte layer never
// owns financial rules: every write goes through the same REST API used by the
// PySide desktop app, dispatched in-process by the Wails bridge.
import {
  BackendInfo,
  BackendReady,
  BackendRequest,
  Clipboard,
  OpenExternal,
  OpenFile,
  RestoreBackup,
  SaveBackendFile,
  SaveTextFile,
} from '../../wailsjs/go/main/App.js'
import type {
  AnalyticsParams,
  AnalyticsResult,
  AnnualReport,
  BackendDiagnostics,
  CascadeDeleteResponse,
  CreateLeaseResponse,
  Dashboard,
  DebtSummary,
  Divida,
  Empreendimento,
  Imovel,
  ImovelInput,
  ImportExecuteResponse,
  ImportPreview,
  Inquilino,
  InstallmentPayment,
  InstallmentUpdate,
  LeaseSummary,
  Locacao,
  LocacaoCreateInput,
  LocacaoRenewInput,
  LocacaoUpdateInput,
  Movimento,
  MovimentoDivida,
  Parcela,
  PropertyBatch,
  Rateio,
  RenewLeaseResponse,
  Roi,
  Simulation,
  Statistics,
} from './types'

export class ApiError extends Error {
  code: string
  status: number

  constructor(message: string, code = 'ERRO', status = 0) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

function normalizeError(error: unknown): ApiError {
  if (error instanceof ApiError) return error
  const raw =
    typeof error === 'string'
      ? error
      : error instanceof Error
        ? error.message
        : String(error)
  const match = raw.match(/HTTP\s+(\d+)\s*:\s*([\s\S]*)/)
  const status = match ? Number(match[1]) : 0
  const message = match ? match[2].trim() : raw.trim()
  let code = 'ERRO'
  if (status === 404) code = 'NAO_ENCONTRADO'
  else if (status === 409) code = 'IMOVEL_OCUPADO'
  else if (status === 422) code = 'VALIDACAO'
  if (/bloquead/i.test(message)) code = 'PARCELA_BLOQUEADA'
  return new ApiError(message || 'Falha inesperada na comunicação com o backend.', code, status)
}

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const payload = body === undefined || body === null ? '' : JSON.stringify(body)
  try {
    const text = await BackendRequest(method, path, payload)
    if (!text) return null as T
    return JSON.parse(text) as T
  } catch (error) {
    throw normalizeError(error)
  }
}

const get = <T>(path: string): Promise<T> => request<T>('GET', path)
const post = <T>(path: string, body?: unknown): Promise<T> => request<T>('POST', path, body ?? {})
const patch = <T>(path: string, body?: unknown): Promise<T> => request<T>('PATCH', path, body ?? {})
const del = <T = void>(path: string): Promise<T> => request<T>('DELETE', path)

type QueryParams = Record<string, string | number | boolean | null | undefined>

function query(params?: QueryParams): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params || {})) {
    if (value === undefined || value === null || value === '') continue
    search.set(key, String(value))
  }
  const text = search.toString()
  return text ? `?${text}` : ''
}

export async function waitForBackend(): Promise<string> {
  try {
    return await BackendReady()
  } catch (error) {
    throw normalizeError(error)
  }
}

export const backendInfo = async (): Promise<BackendDiagnostics> => {
  const raw = await BackendInfo()
  return {
    base_url: String(raw?.base_url ?? ''),
    db_path: String(raw?.db_path ?? ''),
    managed: Boolean(raw?.managed),
    root: raw?.root ? String(raw.root) : undefined,
  }
}
export const openExternal = (url: string): Promise<void> => OpenExternal(url)
export const copyToClipboard = (text: string): Promise<void> => Clipboard(text)
export const saveTextFile = (name: string, content: string): Promise<string> =>
  SaveTextFile(name, content)
export const saveBackendFile = (path: string, suggestedName: string): Promise<string> =>
  SaveBackendFile(path, suggestedName)
export const openFileDialog = (title: string, pattern: string): Promise<string> =>
  OpenFile(title, pattern)
export const restoreBackup = (archive: string): Promise<string> => RestoreBackup(archive)

// ---------------------------------------------------------------- properties
export const listarImoveis = (incluirInativos = false): Promise<Imovel[]> =>
  get<Imovel[]>(`/api/imoveis${incluirInativos ? '?incluir_inativos=true' : ''}`)
export const buscarImovel = (id: number): Promise<Imovel> => get<Imovel>(`/api/imoveis/${id}`)
export const criarImovel = (item: ImovelInput): Promise<Imovel> =>
  post<Imovel>('/api/imoveis', item)
export const atualizarImovel = (item: ImovelInput): Promise<Imovel> =>
  patch<Imovel>(`/api/imoveis/${item.id}`, item)
export const arquivarImovel = (id: number): Promise<void> => del(`/api/imoveis/${id}`)
export const reativarImovel = (id: number): Promise<Imovel> =>
  post<Imovel>(`/api/imoveis/${id}/reativar`)
export const excluirImovelCascata = (id: number): Promise<CascadeDeleteResponse> =>
  del<CascadeDeleteResponse>(`/api/imoveis/${id}/cascade`)
export const loteImoveis = (payload: PropertyBatch): Promise<Imovel[]> =>
  post<Imovel[]>('/api/imoveis/lote', payload)
export const previaLoteImoveis = (payload: PropertyBatch): Promise<Imovel[]> =>
  post<Imovel[]>('/api/imoveis/lote/previa', payload)

// ------------------------------------------------------------------- tenants
export const listarInquilinos = (incluirInativos = false): Promise<Inquilino[]> =>
  get<Inquilino[]>(`/api/inquilinos${incluirInativos ? '?incluir_inativos=true' : ''}`)
export const buscarInquilino = (id: number): Promise<Inquilino> =>
  get<Inquilino>(`/api/inquilinos/${id}`)
export const criarInquilino = (item: Inquilino): Promise<Inquilino> =>
  post<Inquilino>('/api/inquilinos', item)
export const atualizarInquilino = (item: Inquilino): Promise<Inquilino> =>
  patch<Inquilino>(`/api/inquilinos/${item.id}`, item)
export const arquivarInquilino = (id: number): Promise<void> => del(`/api/inquilinos/${id}`)
export const reativarInquilino = (id: number): Promise<Inquilino> =>
  post<Inquilino>(`/api/inquilinos/${id}/reativar`)
export const excluirInquilinoCascata = (id: number): Promise<CascadeDeleteResponse> =>
  del<CascadeDeleteResponse>(`/api/inquilinos/${id}/cascade`)

// ------------------------------------------------------------------- leases
export interface LeaseFilterParams {
  imovel_id?: number
  ativas?: boolean
}

export const listarLocacoes = (params: LeaseFilterParams = {}): Promise<LeaseSummary[]> =>
  get<LeaseSummary[]>(`/api/locacoes${query({ ...params })}`)
export const buscarLocacao = (id: number): Promise<Locacao> => get<Locacao>(`/api/locacoes/${id}`)
export async function locacaoAtivaDoImovel(imovelId: number): Promise<LeaseSummary | null> {
  const items = await listarLocacoes({ imovel_id: imovelId, ativas: true })
  return items?.[0] || null
}
export const criarLocacao = (item: LocacaoCreateInput): Promise<CreateLeaseResponse> =>
  post<CreateLeaseResponse>('/api/locacoes', item)
export const atualizarLocacao = (id: number, payload: LocacaoUpdateInput): Promise<Locacao> =>
  patch<Locacao>(`/api/locacoes/${id}`, payload)
export const renovarLocacao = (
  id: number,
  payload: LocacaoRenewInput,
): Promise<RenewLeaseResponse> => post<RenewLeaseResponse>(`/api/locacoes/${id}/renovar`, payload)
export const encerrarLocacao = (id: number, hoje?: string): Promise<{ canceladas: number }> =>
  post<{ canceladas: number }>(`/api/locacoes/${id}/encerrar${query({ h: hoje })}`)
export const excluirLocacao = (id: number): Promise<void> => del(`/api/locacoes/${id}`)

// ------------------------------------------------------------- installments
export interface InstallmentFilterParams {
  locacao_id?: number
  imovel_id?: number
  situacao?: string
  h?: string
}

export const listarParcelas = (params: InstallmentFilterParams = {}): Promise<Parcela[]> =>
  get<Parcela[]>(`/api/parcelas${query({ ...params })}`)
export const buscarParcela = (id: number): Promise<Parcela> => get<Parcela>(`/api/parcelas/${id}`)
export const criarParcelaExtra = (item: Partial<Parcela> & Pick<Parcela, 'locacao_id' | 'vencimento' | 'valor_base_c'>): Promise<Parcela> =>
  post<Parcela>('/api/parcelas', item)
export const atualizarParcela = (id: number, payload: InstallmentUpdate): Promise<Parcela> =>
  patch<Parcela>(`/api/parcelas/${id}`, payload)
export const estornarParcela = (id: number): Promise<Parcela> =>
  post<Parcela>(`/api/parcelas/${id}/estornar`)
export const cancelarParcela = (id: number): Promise<Parcela> =>
  post<Parcela>(`/api/parcelas/${id}/cancelar`)
export const excluirParcela = (id: number): Promise<void> => del(`/api/parcelas/${id}`)
export const simularParcela = (id: number, hoje?: string): Promise<Simulation> =>
  get<Simulation>(`/api/parcelas/${id}/simular${query({ h: hoje })}`)
export const quitarParcela = (id: number, pagamento: InstallmentPayment): Promise<Parcela> =>
  post<Parcela>(`/api/parcelas/${id}/quitar`, pagamento)
export const quitarLote = (
  ids: number[],
  pagamento: Pick<InstallmentPayment, 'data' | 'forma'>,
): Promise<Parcela[]> =>
  post<Parcela[]>('/api/parcelas/quitar-lote', {
    ids,
    data: pagamento.data,
    forma: pagamento.forma,
  })
export const quitarLoteIndividual = (pagamentos: InstallmentPayment[]): Promise<Parcela[]> =>
  post<Parcela[]>('/api/parcelas/quitar-lote', { pagamentos })
export const salvarRecibo = (id: number): Promise<string> =>
  SaveBackendFile(`/recibos/${id}.pdf`, `recibo_${id}.pdf`)

// --------------------------------------------------------------- projects
export const listarEmpreendimentos = (incluirInativos = false): Promise<Empreendimento[]> =>
  get<Empreendimento[]>(`/api/empreendimentos${incluirInativos ? '?incluir_inativos=true' : ''}`)
export const buscarEmpreendimento = (id: number): Promise<Empreendimento> =>
  get<Empreendimento>(`/api/empreendimentos/${id}`)
export const criarEmpreendimento = (item: Empreendimento): Promise<Empreendimento> =>
  post<Empreendimento>('/api/empreendimentos', item)
export const atualizarEmpreendimento = (item: Empreendimento): Promise<Empreendimento> =>
  patch<Empreendimento>(`/api/empreendimentos/${item.id}`, item)
export const arquivarEmpreendimento = (id: number): Promise<void> =>
  del(`/api/empreendimentos/${id}`)
export const reativarEmpreendimento = (id: number): Promise<Empreendimento> =>
  post<Empreendimento>(`/api/empreendimentos/${id}/reativar`)
export const excluirEmpreendimento = (id: number): Promise<void> =>
  del(`/api/empreendimentos/${id}/permanente`)

// --------------------------------------------------------------- movements
export interface MovementFilterParams {
  tipo?: string
  natureza?: string
  inicio?: string
  fim?: string
  imovel_id?: number
  empreendimento_id?: number
}

export const listarMovimentacoes = (params: MovementFilterParams = {}): Promise<Movimento[]> =>
  get<Movimento[]>(`/api/movimentacoes${query({ ...params })}`)
export const rateiosMovimentacao = (id: number): Promise<Rateio[]> =>
  get<Rateio[]>(`/api/movimentacoes/${id}/rateios`)
export const criarMovimentacao = (
  movimento: Movimento,
  rateios: Rateio[] = [],
): Promise<Movimento> => post<Movimento>('/api/movimentacoes', { movimento, rateios })
export const atualizarMovimentacao = (
  movimento: Movimento,
  rateios: Rateio[] = [],
): Promise<Movimento> =>
  patch<Movimento>(`/api/movimentacoes/${movimento.id}`, { movimento, rateios })
export const excluirMovimentacao = (id: number): Promise<void> => del(`/api/movimentacoes/${id}`)

// -------------------------------------------------------------------- debts
export const listarDividas = (): Promise<DebtSummary[]> => get<DebtSummary[]>('/api/dividas')
export const criarDivida = (item: Divida): Promise<Divida> => post<Divida>('/api/dividas', item)
export const atualizarDivida = (item: Divida): Promise<Divida> =>
  patch<Divida>(`/api/dividas/${item.id}`, item)
export const excluirDivida = (id: number): Promise<void> => del(`/api/dividas/${id}`)
export const adicionarMovimentoDivida = (item: MovimentoDivida): Promise<MovimentoDivida> =>
  post<MovimentoDivida>(`/api/dividas/${item.divida_id}/movimentos`, item)
export const excluirMovimentoDivida = (id: number): Promise<void> =>
  del(`/api/dividas/movimentos/${id}`)

// -------------------------------------------------------------- dashboards
export interface RangeQuery {
  h?: string
  inicio?: string
  fim?: string
}

export const dashboard = (hoje?: string, range?: { inicio?: string; fim?: string }): Promise<Dashboard> =>
  get<Dashboard>(`/api/dashboard${query({ h: hoje, inicio: range?.inicio, fim: range?.fim })}`)
export const roiImovel = (id: number, hoje?: string, range?: { inicio?: string; fim?: string }): Promise<Roi> =>
  get<Roi>(`/api/imoveis/${id}/roi${query({ h: hoje, inicio: range?.inicio, fim: range?.fim })}`)
export const analyticsSeries = (params: AnalyticsParams): Promise<AnalyticsResult> =>
  get<AnalyticsResult>(`/api/analytics/series${query({ ...params })}`)
export const estatisticas = (
  unidadesSomente: boolean,
  range?: { inicio?: string; fim?: string },
): Promise<Statistics> =>
  get<Statistics>(
    `/api/estatisticas${query({
      modo: unidadesSomente ? 'unidades' : 'todos',
      inicio: range?.inicio,
      fim: range?.fim,
    })}`,
  )
export const relatorioAnual = (ano: number): Promise<AnnualReport> =>
  get<AnnualReport>(`/api/relatorios/anual${query({ ano })}`)

// ---------------------------------------------------------------- importer
export async function escolherPlanilha(): Promise<{ path: string; preview: ImportPreview } | null> {
  const path = await openFileDialog('Selecionar planilha de gerenciamento', '*.xlsx')
  if (!path) return null
  const preview = await post<ImportPreview>('/api/import/preview', { path })
  return { path, preview }
}

export const importarPlanilha = (path: string): Promise<ImportExecuteResponse> =>
  post<ImportExecuteResponse>('/api/import/execute', { path })

// ------------------------------------------------------------------ exports
export async function salvarBackupCompleto(): Promise<string> {
  const today = new Date().toISOString().slice(0, 10)
  return SaveBackendFile('/api/backup/full', `rerum-backup-completo-${today}.zip`)
}

export async function salvarBackupSqlite(): Promise<string> {
  const today = new Date().toISOString().slice(0, 10)
  return SaveBackendFile('/api/backup/sqlite', `rerum-backup-${today}.sqlite`)
}
