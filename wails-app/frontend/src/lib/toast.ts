import { writable } from 'svelte/store'

export type ToastKind = 'info' | 'success' | 'error'

export interface ToastItem {
  id: number
  kind: ToastKind
  message: string
  title: string
}

export const toasts = writable<ToastItem[]>([])
let counter = 0

export function dismiss(id: number): void {
  toasts.update((items) => items.filter((item) => item.id !== id))
}

export function notify(message: unknown, kind: ToastKind = 'info', title = ''): number {
  const id = ++counter
  toasts.update((items) => [...items, { id, kind, message: String(message), title }])
  const timeout = kind === 'error' ? 8000 : 4500
  setTimeout(() => dismiss(id), timeout)
  return id
}

export const info = (message: unknown, title = ''): number => notify(message, 'info', title)
export const success = (message: unknown, title = ''): number => notify(message, 'success', title)
export const error = (message: unknown, title = 'Atenção'): number => notify(message, 'error', title)

export function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message
  if (typeof err === 'object' && err !== null && 'message' in err) {
    return String((err as { message: unknown }).message)
  }
  return String(err)
}
