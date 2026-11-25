<script lang="ts">
  import { onMount } from 'svelte'
  import CommandPalette from './components/CommandPalette.svelte'
  import ConfirmHost from './components/ConfirmHost.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import Toasts from './components/Toasts.svelte'
  import { waitForBackend } from './lib/api'
  import { getSetting, setSetting } from './lib/settings'
  import { errorMessage } from './lib/toast'
  import type { PageRoute } from './lib/types'
  import DashboardPage from './pages/DashboardPage.svelte'
  import FinancialPage from './pages/FinancialPage.svelte'
  import LeasesPage from './pages/LeasesPage.svelte'
  import ProjectsPage from './pages/ProjectsPage.svelte'
  import PropertiesPage from './pages/PropertiesPage.svelte'
  import SettingsPage from './pages/SettingsPage.svelte'
  import StatisticsPage from './pages/StatisticsPage.svelte'
  import TenantsPage from './pages/TenantsPage.svelte'

  const ROUTES_BY_DIGIT: PageRoute[] = [
    'dashboard',
    'empreendimentos',
    'imoveis',
    'inquilinos',
    'locacoes',
    'financeiro',
    'estatisticas',
    'configuracoes',
  ]

  const savedPage = getSetting<PageRoute>('app/current_page', 'dashboard')
  let current: PageRoute = ROUTES_BY_DIGIT.includes(savedPage) ? savedPage : 'dashboard'
  let backendReady = false
  let backendError = ''
  let refreshToken = 0
  let paletteOpen = false

  function navigate(page: PageRoute): void {
    current = page
    setSetting('app/current_page', page)
    refreshToken += 1
  }

  function triggerRefresh(): void {
    refreshToken += 1
  }

  function onGlobalKeydown(event: KeyboardEvent): void {
    const target = event.target as HTMLElement | null
    const inEditable = Boolean(
      target &&
        (target.tagName === 'INPUT' ||
          target.tagName === 'TEXTAREA' ||
          target.tagName === 'SELECT' ||
          target.isContentEditable),
    )

    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault()
      paletteOpen = !paletteOpen
      return
    }

    if ((event.ctrlKey || event.metaKey) && event.key === ',') {
      event.preventDefault()
      navigate('configuracoes')
      return
    }

    if ((event.ctrlKey || event.metaKey) && /^[1-8]$/.test(event.key)) {
      event.preventDefault()
      const route = ROUTES_BY_DIGIT[Number(event.key) - 1]
      if (route) navigate(route)
      return
    }

    if (event.key === 'F5') {
      event.preventDefault()
      triggerRefresh()
      return
    }

    if (!inEditable && event.key === '/' && !paletteOpen) {
      const searchInput = document.querySelector<HTMLInputElement>(
        'main input[type="search"], main .search input',
      )
      if (searchInput) {
        event.preventDefault()
        searchInput.focus()
        searchInput.select()
      }
    }
  }

  onMount(async () => {
    try {
      await waitForBackend()
      backendReady = true
    } catch (err) {
      backendError = errorMessage(err) || 'Não foi possível iniciar o backend local.'
    }
  })
</script>

<svelte:window on:keydown={onGlobalKeydown} />

<div class="app-shell">
  <Sidebar {current} onNavigate={navigate} onOpenPalette={() => (paletteOpen = true)} />
  <main class="main">
    {#if backendError}
      <div class="page">
        <div class="surface-card">
          <div class="empty error-text">
            <strong>Falha ao iniciar o backend:</strong> {backendError}
          </div>
        </div>
      </div>
    {:else if !backendReady}
      <div class="page">
        <div class="surface-card">
          <div class="empty">Iniciando backend local…</div>
        </div>
      </div>
    {:else if current === 'dashboard'}
      <DashboardPage onNavigate={navigate} {refreshToken} />
    {:else if current === 'empreendimentos'}
      <ProjectsPage {refreshToken} />
    {:else if current === 'imoveis'}
      <PropertiesPage {refreshToken} />
    {:else if current === 'inquilinos'}
      <TenantsPage {refreshToken} />
    {:else if current === 'locacoes'}
      <LeasesPage {refreshToken} />
    {:else if current === 'financeiro'}
      <FinancialPage {refreshToken} />
    {:else if current === 'estatisticas'}
      <StatisticsPage {refreshToken} />
    {:else if current === 'configuracoes'}
      <SettingsPage onDataChanged={triggerRefresh} />
    {/if}
  </main>
</div>

{#if paletteOpen && backendReady}
  <CommandPalette
    onClose={() => (paletteOpen = false)}
    onNavigate={navigate}
    onRefresh={triggerRefresh}
  />
{/if}

<Toasts />
<ConfirmHost />
