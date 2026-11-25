import { writable } from 'svelte/store'

export interface ConfirmOptions {
  title?: string
  message?: string
  confirmLabel?: string
  cancelLabel?: string
  destructive?: boolean
  danger?: boolean
}

export interface ConfirmState {
  open: boolean
  title: string
  message: string
  confirmLabel: string
  cancelLabel: string
  destructive: boolean
}

const initialState: ConfirmState = {
  open: false,
  title: 'Confirmar ação',
  message: '',
  confirmLabel: 'Confirmar',
  cancelLabel: 'Cancelar',
  destructive: false,
}

export const confirmState = writable<ConfirmState>(initialState)

let resolver: ((value: boolean) => void) | null = null

export function confirmAction(options: ConfirmOptions = {}): Promise<boolean> {
  const {
    title = 'Confirmar ação',
    message = '',
    confirmLabel = 'Confirmar',
    cancelLabel = 'Cancelar',
    destructive = false,
    danger = false,
  } = options
  return new Promise<boolean>((resolve) => {
    resolver = resolve
    confirmState.set({
      open: true,
      title,
      message,
      confirmLabel,
      cancelLabel,
      destructive: destructive || danger,
    })
  })
}

export function resolveConfirm(value: boolean): void {
  confirmState.set(initialState)
  if (resolver) {
    resolver(Boolean(value))
    resolver = null
  }
}
