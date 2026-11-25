<script lang="ts">
  import Icon from './Icon.svelte'
  import type { IconName, PageRoute } from '../lib/types'

  const logo = import.meta.env.BASE_URL + 'rerum.svg'

  export let current: PageRoute = 'dashboard'
  export let onNavigate: (page: PageRoute) => void = () => {}
  export let onOpenPalette: () => void = () => {}

  const items: Array<[PageRoute, string, IconName]> = [
    ['dashboard', 'Visão geral', 'dashboard'],
    ['empreendimentos', 'Empreendimentos', 'projects'],
    ['imoveis', 'Imóveis', 'properties'],
    ['inquilinos', 'Inquilinos', 'tenants'],
    ['locacoes', 'Locações', 'leases'],
    ['financeiro', 'Financeiro', 'ledger'],
    ['estatisticas', 'Relatórios e IRPF', 'statistics'],
  ]
</script>

<aside class="sidebar">
  <div class="brand">
    <img src={logo} alt="" />
    <span class="brand-name">Rerum</span>
  </div>
  <div class="brand-sub">Gestão patrimonial e de aluguéis</div>

  <button class="nav-shortcut-hint" type="button" on:click={onOpenPalette} title="Busca rápida e comandos (Ctrl+K)">
    <span class="flex" style="gap:7px"><Icon name="search" size={14} /> Buscar / Ir para…</span>
    <span class="kbd">Ctrl+K</span>
  </button>

  <div style="height:4px"></div>

  {#each items as [key, label, icon], idx}
    <button
      class="nav-item"
      class:active={current === key}
      on:click={() => onNavigate(key)}
      type="button"
      title="{label} (Ctrl+{idx + 1})"
    >
      <Icon name={icon} />
      <span>{label}</span>
    </button>
  {/each}

  <div class="nav-spacer"></div>
  <div class="nav-footer">
    <button
      class="nav-item"
      class:active={current === 'configuracoes'}
      on:click={() => onNavigate('configuracoes')}
      type="button"
      title="Configurações (Ctrl+,)"
    >
      <Icon name="settings" />
      <span>Configurações</span>
    </button>
  </div>
</aside>
