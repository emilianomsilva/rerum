import { LoadPreferences, SavePreferences } from '../../wailsjs/go/main/App'

const PREFIX = 'rerum:'
const settings = new Map<string, unknown>()
let pendingSave = Promise.resolve()
let nativePersistence = false

export async function loadSettings(): Promise<void> {
  try {
    for (let index = 0; index < localStorage.length; index += 1) {
      const key = localStorage.key(index)
      if (!key?.startsWith(PREFIX)) continue
      try {
        settings.set(key.slice(PREFIX.length), JSON.parse(localStorage.getItem(key) ?? 'null'))
      } catch {
        continue
      }
    }
  } catch {
    // The desktop preferences can still be loaded when browser storage is unavailable.
  }

  try {
    const saved = JSON.parse(await LoadPreferences()) as Record<string, unknown> | null
    for (const [key, value] of Object.entries(saved ?? {})) settings.set(key, value)
    nativePersistence = true
    if (settings.size > 0) persistSettings()
  } catch (error) {
    console.warn('Não foi possível carregar as preferências locais do Rerum.', error)
  }
}

function persistSettings(): void {
  if (!nativePersistence) return
  const payload = JSON.stringify(Object.fromEntries(settings))
  pendingSave = pendingSave
    .then(() => SavePreferences(payload))
    .catch((error: unknown) => console.warn('Não foi possível salvar as preferências do Rerum.', error))
}

export function getSetting<T>(key: string, fallback: T): T {
  return settings.has(key) ? (settings.get(key) as T) : fallback
}

export function setSetting<T>(key: string, value: T): void {
  if (Object.is(settings.get(key), value)) return
  settings.set(key, value)
  try {
    localStorage.setItem(PREFIX + key, JSON.stringify(value))
  } catch {
    // The desktop settings file can still be saved.
  }
  persistSettings()
}

export function removeSetting(key: string): void {
  settings.delete(key)
  try {
    localStorage.removeItem(PREFIX + key)
  } catch {
    // The desktop settings file can still be saved.
  }
  persistSettings()
}
