import type { Parcela, ParcelaSituacao } from './types'

export const MONTH_ABBR: Record<string, string> = {
  '01': 'Jan',
  '02': 'Fev',
  '03': 'Mar',
  '04': 'Abr',
  '05': 'Mai',
  '06': 'Jun',
  '07': 'Jul',
  '08': 'Ago',
  '09': 'Set',
  '10': 'Out',
  '11': 'Nov',
  '12': 'Dez',
}

export function money(cents: number | null | undefined): string {
  const value = (Number(cents) || 0) / 100
  const negative = value < 0
  const fixed = Math.abs(value).toFixed(2)
  const [integer, decimals] = fixed.split('.')
  const grouped = integer.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  return `R$ ${negative ? '-' : ''}${grouped},${decimals}`
}

export function percent(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(Number(value))) return '—'
  return `${Number(value).toFixed(2).replace('.', ',')}%`
}

export function parseISO(value: string | Date | null | undefined): Date | null {
  if (!value) return null
  if (value instanceof Date) return value
  const text = String(value).slice(0, 10)
  const [year, month, day] = text.split('-').map(Number)
  if (!year || !month || !day) return null
  return new Date(year, month - 1, day)
}

export function toISO(value: string | Date | null | undefined): string {
  if (!value) return ''
  const date = value instanceof Date ? value : parseISO(value)
  if (!date) return ''
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function todayISO(): string {
  return toISO(new Date())
}

export function dateBR(value: string | Date | null | undefined): string {
  const date = parseISO(value)
  if (!date) return '—'
  return `${String(date.getDate()).padStart(2, '0')}/${String(date.getMonth() + 1).padStart(2, '0')}/${date.getFullYear()}`
}

export function daysBetween(
  from: string | Date | null | undefined,
  to: string | Date | null | undefined,
): number {
  const start = parseISO(from)
  const end = parseISO(to)
  if (!start || !end) return 0
  return Math.round((end.getTime() - start.getTime()) / 86400000)
}

export function addMonths(value: string | Date | null | undefined, months: number): Date | null {
  const date = parseISO(value)
  if (!date) return null
  const total = date.getFullYear() * 12 + date.getMonth() + months
  const year = Math.floor(total / 12)
  const month = total % 12
  const lastDay = new Date(year, month + 1, 0).getDate()
  return new Date(year, month, Math.min(date.getDate(), lastDay))
}

export function endByDuration(
  startISOValue: string | Date | null | undefined,
  durationMonths: number,
): string {
  const target = addMonths(startISOValue, Math.max(1, Number(durationMonths) || 1))
  if (!target) return ''
  target.setDate(target.getDate() - 1)
  return toISO(target)
}

export function reaisToCents(value: string | number | null | undefined): number {
  const number =
    typeof value === 'number'
      ? value
      : Number(String(value ?? '').replace(/\./g, '').replace(',', '.'))
  if (!Number.isFinite(number)) return 0
  return Math.round(number * 100)
}

export function centsToReais(value: number | null | undefined): number {
  return (Number(value) || 0) / 100
}

export function monthLabel(mes: string | null | undefined): string {
  if (!mes) return '—'
  const parts = String(mes).split('-')
  if (parts.length === 1) return mes
  const [year, month] = parts
  return `${MONTH_ABBR[month] || month}/${year}`
}

export function periodLabel(inicio: string | null | undefined, fim: string | null | undefined): string {
  if (!inicio || !fim) return '—'
  return `${dateBR(inicio)} – ${dateBR(fim)}`
}

export function digitsOnly(value: string | number | null | undefined): string {
  return String(value ?? '').replace(/\D/g, '')
}

export function formatCPF(value: string | null | undefined): string {
  const digits = digitsOnly(value).slice(0, 11)
  return digits
    .replace(/(\d{3})(\d)/, '$1.$2')
    .replace(/(\d{3})(\d)/, '$1.$2')
    .replace(/(\d{3})(\d{1,2})$/, '$1-$2')
}

export function formatPhone(value: string | null | undefined): string {
  let digits = digitsOnly(value)
  if (digits.startsWith('55') && digits.length > 11) digits = digits.slice(2)
  digits = digits.slice(0, 11)
  if (digits.length <= 2) return digits ? `(${digits}${digits.length === 2 ? ')' : ''}` : ''
  const split = digits.length === 10 ? 6 : 7
  return `(${digits.slice(0, 2)}) ${digits.slice(2, split)}${digits.length > split ? '-' + digits.slice(split) : ''}`
}

export function validateContact(value: string | null | undefined, kind: 'cpf' | 'telefone'): string {
  const digits = digitsOnly(value)
  if (!digits) return ''
  if (kind === 'cpf' && digits.length !== 11) return 'CPF deve conter 11 dígitos.'
  if (kind === 'telefone' && ![10, 11].includes(digits.length))
    return 'Telefone deve conter 10 ou 11 dígitos.'
  return ''
}

export function statusLabel(status: string): string {
  const labels: Record<string, string> = {
    paga: 'Paga',
    cancelada: 'Cancelada',
    vencida: 'Vencida',
    'parcial-vencida': 'Parcial (vencida)',
    parcial: 'Parcial',
    aberta: 'Aberta',
    ativa: 'Ativa',
    encerrada: 'Encerrada',
  }
  return labels[status] || status
}

export function situacaoParcela(
  parcela: Pick<Parcela, 'status' | 'vencimento'>,
  today: string = todayISO(),
): ParcelaSituacao {
  if (parcela.status === 'paga') return 'paga'
  if (parcela.status === 'cancelada') return 'cancelada'
  const due = String(parcela.vencimento ?? '').slice(0, 10)
  if (due && due < today) return parcela.status === 'parcial' ? 'parcial-vencida' : 'vencida'
  return parcela.status === 'parcial' ? 'parcial' : 'aberta'
}

export function csvCell(value: unknown): string {
  const text = String(value ?? '')
  if (/[";\n\r]/.test(text)) return `"${text.replace(/"/g, '""')}"`
  return text
}

export function csvContent(headers: string[], rows: unknown[][]): string {
  const lines = [headers.map(csvCell), ...rows.map((row) => row.map(csvCell))]
  return '\ufeff' + lines.map((line) => line.join(';')).join('\r\n')
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}
