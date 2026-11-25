import { MONTH_ABBR, dateBR, parseISO, todayISO, toISO } from './format'

export type PeriodPreset =
  | 'todos'
  | 'mes_atual'
  | 'mes_anterior'
  | '15'
  | '30'
  | '90'
  | '6'
  | '12'
  | 'ano'
  | 'ano_anterior'
  | 'custom'

export const DASHBOARD_PERIOD_OPTIONS: Array<[PeriodPreset, string]> = [
  ['mes_atual', 'Mês atual'],
  ['mes_anterior', 'Mês anterior (fechado)'],
  ['15', 'Últimos 15 dias'],
  ['30', 'Últimos 30 dias'],
  ['90', 'Últimos 90 dias'],
  ['6', 'Últimos 6 meses'],
  ['12', 'Últimos 12 meses'],
  ['ano', 'Ano fiscal atual'],
  ['ano_anterior', 'Ano fiscal anterior'],
  ['todos', 'Acumulado total (histórico)'],
  ['custom', 'Personalizado…'],
]

export const ROI_PERIOD_OPTIONS: Array<[PeriodPreset, string]> = [
  ['12', 'Últimos 12 meses'],
  ['ano', 'Ano fiscal atual'],
  ['ano_anterior', 'Ano fiscal anterior'],
  ['6', 'Últimos 6 meses'],
  ['90', 'Últimos 90 dias'],
  ['30', 'Últimos 30 dias'],
  ['mes_atual', 'Mês atual'],
  ['mes_anterior', 'Mês anterior (fechado)'],
  ['todos', 'Acumulado total (histórico)'],
  ['custom', 'Personalizado…'],
]

export interface ResolvedPeriod {
  preset: PeriodPreset
  start: string
  end: string
  allTime: boolean
  granularidade: 'dia' | 'mes'
  shortLabel: string
  receiptTitle: string
  receivableTitle: string
  expenseTitle: string
  roiTitle: string
  hint: string
  monthly: boolean
}

export function addDays(date: Date, amount: number): Date {
  const copy = new Date(date)
  copy.setDate(copy.getDate() + amount)
  return copy
}

export function lastDayOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth() + 1, 0)
}

export function resolvePeriod(
  preset: string,
  customStart = '',
  customEnd = '',
  now: Date = new Date(),
): ResolvedPeriod {
  let start: Date
  let end: Date
  let allTime = false
  let granularidade: 'dia' | 'mes' = 'dia'
  let shortLabel: string
  let receiptTitle: string
  let receivableTitle: string
  let expenseTitle: string
  let roiTitle: string
  let hint: string

  if (preset === 'todos') {
    allTime = true
    start = new Date(2000, 0, 1)
    end = now
    shortLabel = 'Acumulado'
    receiptTitle = 'RECEBIDO (HISTÓRICO)'
    receivableTitle = 'A RECEBER (ABERTO)'
    expenseTitle = 'DESPESAS (HISTÓRICO)'
    roiTitle = 'ROI ACUMULADO'
    hint = `Todo o histórico até ${dateBR(end)}`
    granularidade = 'mes'
  } else if (preset === 'mes_anterior') {
    const prev = new Date(now.getFullYear(), now.getMonth() - 1, 1)
    start = prev
    end = lastDayOfMonth(prev)
    const abbr = (MONTH_ABBR[String(prev.getMonth() + 1).padStart(2, '0')] || '').toUpperCase()
    shortLabel = `${abbr}/${prev.getFullYear()}`
    receiptTitle = `RECEBIDO (${shortLabel})`
    receivableTitle = `A RECEBER (${shortLabel})`
    expenseTitle = `DESPESAS (${shortLabel})`
    roiTitle = `ROI (${shortLabel})`
    hint = `Mês fechado: ${dateBR(start)} a ${dateBR(end)}`
  } else if (preset === '15' || preset === '30' || preset === '90') {
    const d = Number(preset)
    start = addDays(now, -(d - 1))
    end = now
    shortLabel = `${d} dias`
    receiptTitle = `RECEBIDO (${d} DIAS)`
    receivableTitle = `A RECEBER (${d} DIAS)`
    expenseTitle = `DESPESAS (${d} DIAS)`
    roiTitle = `ROI (${d} DIAS)`
    hint = `Últimos ${d} dias: ${dateBR(start)} a ${dateBR(end)}`
    granularidade = d <= 45 ? 'dia' : 'mes'
  } else if (preset === '6') {
    start = new Date(now.getFullYear(), now.getMonth() - 5, 1)
    end = now
    shortLabel = '6 meses'
    receiptTitle = 'RECEBIDO (6 MESES)'
    receivableTitle = 'A RECEBER (6 MESES)'
    expenseTitle = 'DESPESAS (6 MESES)'
    roiTitle = 'ROI (6 MESES)'
    hint = `Últimos 6 meses: ${dateBR(start)} a ${dateBR(end)}`
    granularidade = 'mes'
  } else if (preset === '12') {
    // Rolling 365-day window (matches exact last-12-months ROI window)
    start = addDays(now, -365)
    end = now
    shortLabel = '12 meses'
    receiptTitle = 'RECEBIDO (12 MESES)'
    receivableTitle = 'A RECEBER (12 MESES)'
    expenseTitle = 'DESPESAS (12 MESES)'
    roiTitle = 'ROI (12 MESES)'
    hint = `Últimos 12 meses: ${dateBR(start)} a ${dateBR(end)}`
    granularidade = 'mes'
  } else if (preset === 'ano') {
    start = new Date(now.getFullYear(), 0, 1)
    end = now
    shortLabel = `${now.getFullYear()}`
    receiptTitle = `RECEBIDO (${now.getFullYear()})`
    receivableTitle = `A RECEBER (${now.getFullYear()})`
    expenseTitle = `DESPESAS (${now.getFullYear()})`
    roiTitle = `ROI (${now.getFullYear()})`
    hint = `Ano fiscal ${now.getFullYear()}: ${dateBR(start)} a ${dateBR(end)}`
    granularidade = 'mes'
  } else if (preset === 'ano_anterior') {
    const prevY = now.getFullYear() - 1
    start = new Date(prevY, 0, 1)
    end = new Date(prevY, 11, 31)
    shortLabel = `${prevY}`
    receiptTitle = `RECEBIDO (${prevY})`
    receivableTitle = `A RECEBER (${prevY})`
    expenseTitle = `DESPESAS (${prevY})`
    roiTitle = `ROI (${prevY})`
    hint = `Ano fiscal fechado ${prevY}: ${dateBR(start)} a ${dateBR(end)}`
    granularidade = 'mes'
  } else if (preset === 'custom') {
    start = parseISO(customStart) || new Date(now.getFullYear(), now.getMonth(), 1)
    end = parseISO(customEnd) || now
    if (start > end) [start, end] = [end, start]
    const span = Math.round((end.getTime() - start.getTime()) / 86400000) + 1
    shortLabel = `${span} dias`
    receiptTitle = `RECEBIDO (${span} DIAS)`
    receivableTitle = `A RECEBER (${span} DIAS)`
    expenseTitle = `DESPESAS (${span} DIAS)`
    roiTitle = `ROI (${span} DIAS)`
    hint = `Período de ${span} dias: ${dateBR(start)} a ${dateBR(end)}`
    granularidade = span <= 45 ? 'dia' : 'mes'
  } else {
    start = new Date(now.getFullYear(), now.getMonth(), 1)
    end = lastDayOfMonth(now)
    shortLabel = 'Mês atual'
    receiptTitle = 'RECEBIDO NO MÊS'
    receivableTitle = 'A RECEBER NO MÊS'
    expenseTitle = 'DESPESAS NO MÊS'
    roiTitle = 'ROI NO MÊS'
    hint = `Mês corrente: ${dateBR(start)} a ${dateBR(end)}`
  }

  return {
    preset: (preset as PeriodPreset) || 'mes_atual',
    start: allTime ? '' : toISO(start),
    end: allTime ? '' : toISO(end),
    allTime,
    granularidade,
    shortLabel,
    receiptTitle,
    receivableTitle,
    expenseTitle,
    roiTitle,
    hint,
    monthly: preset === 'mes_atual' || preset === 'mes_anterior',
  }
}

export function defaultCustomRange(): { start: string; end: string; days: number } {
  const now = new Date()
  const start = new Date(now.getFullYear(), now.getMonth(), 1)
  return {
    start: toISO(start),
    end: todayISO(),
    days: now.getDate(),
  }
}
