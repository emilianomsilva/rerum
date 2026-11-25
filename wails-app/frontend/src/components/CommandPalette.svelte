<script lang="ts">
  import { onMount } from 'svelte'
  import {
    listarEmpreendimentos,
    listarImoveis,
    listarInquilinos,
    listarLocacoes,
    salvarBackupCompleto,
    salvarBackupSqlite,
  } from '../lib/api'
  import { setSetting } from '../lib/settings'
  import { error as notifyError, errorMessage, success } from '../lib/toast'
  import type { IconName, PageRoute } from '../lib/types'
  import Icon from './Icon.svelte'

  export let onClose: () => void = () => {}
  export let onNavigate: (page: PageRoute) => void = () => {}
  export let onRefresh: () => void = () => {}

  interface PaletteItem {
    id: string
    group: string
    label: string
    sub?: string
    icon: IconName
    shortcut?: string
    run: () => void | Promise<void>
  }

  let query = ''
  let activeIndex = 0
  let inputEl: HTMLInputElement | null = null
  let dynamicItems: PaletteItem[] = []

  const staticItems: PaletteItem[] = [
    { id: 'nav-dash', group: 'Navegação', label: 'Ir para Visão geral', icon: 'dashboard', shortcut: 'Ctrl+1', run: () => onNavigate('dashboard') },
    { id: 'nav-emp', group: 'Navegação', label: 'Ir para Empreendimentos', icon: 'projects', shortcut: 'Ctrl+2', run: () => onNavigate('empreendimentos') },
    { id: 'nav-imo', group: 'Navegação', label: 'Ir para Imóveis', icon: 'properties', shortcut: 'Ctrl+3', run: () => onNavigate('imoveis') },
    { id: 'nav-inq', group: 'Navegação', label: 'Ir para Inquilinos', icon: 'tenants', shortcut: 'Ctrl+4', run: () => onNavigate('inquilinos') },
    { id: 'nav-loc', group: 'Navegação', label: 'Ir para Locações', icon: 'leases', shortcut: 'Ctrl+5', run: () => onNavigate('locacoes') },
    {
      id: 'nav-cob',
      group: 'Navegação',
      label: 'Ir para Cobranças e Aluguéis',
      icon: 'ledger',
      shortcut: 'Ctrl+6',
      run: () => {
        setSetting('financeiro/tab', 'cobrancas')
        onNavigate('financeiro')
      },
    },
    {
      id: 'nav-mov',
      group: 'Navegação',
      label: 'Ir para Livro Financeiro (Movimentações)',
      icon: 'cash',
      run: () => {
        setSetting('financeiro/tab', 'movimentacoes')
        onNavigate('financeiro')
      },
    },
    {
      id: 'nav-div',
      group: 'Navegação',
      label: 'Ir para Débitos e Financiamentos',
      icon: 'debts',
      run: () => {
        setSetting('financeiro/tab', 'dividas')
        onNavigate('financeiro')
      },
    },
    { id: 'nav-est', group: 'Navegação', label: 'Ir para Relatórios e IRPF', icon: 'statistics', shortcut: 'Ctrl+7', run: () => onNavigate('estatisticas') },
    { id: 'nav-cfg', group: 'Navegação', label: 'Abrir Configurações', icon: 'settings', shortcut: 'Ctrl+,', run: () => onNavigate('configuracoes') },
    { id: 'act-ref', group: 'Ações rápidas', label: 'Atualizar dados da tela atual', icon: 'refresh', shortcut: 'F5', run: () => onRefresh() },
    {
      id: 'act-bkfull',
      group: 'Ações rápidas',
      label: 'Exportar backup completo (.zip)…',
      icon: 'download',
      run: async () => {
        try {
          const saved = await salvarBackupCompleto()
          if (saved) success(`Backup completo salvo em: ${saved}`)
        } catch (err) {
          notifyError(errorMessage(err))
        }
      },
    },
    {
      id: 'act-bksql',
      group: 'Ações rápidas',
      label: 'Exportar backup SQLite (.sqlite)…',
      icon: 'download',
      run: async () => {
        try {
          const saved = await salvarBackupSqlite()
          if (saved) success(`Backup SQLite salvo em: ${saved}`)
        } catch (err) {
          notifyError(errorMessage(err))
        }
      },
    },
  ]

  $: allItems = [...staticItems, ...dynamicItems]
  $: filtered = filterPalette(allItems, query)
  $: if (activeIndex >= filtered.length) activeIndex = 0

  function filterPalette(list: PaletteItem[], termRaw: string): PaletteItem[] {
    const term = termRaw.trim().toLocaleLowerCase('pt-BR')
    if (!term) return list.slice(0, 14)
    return list
      .filter((item) =>
        `${item.group} ${item.label} ${item.sub || ''}`.toLocaleLowerCase('pt-BR').includes(term),
      )
      .slice(0, 20)
  }

  async function execute(item: PaletteItem): Promise<void> {
    onClose()
    await item.run()
  }

  function onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      event.preventDefault()
      onClose()
    } else if (event.key === 'ArrowDown') {
      event.preventDefault()
      if (filtered.length) activeIndex = (activeIndex + 1) % filtered.length
    } else if (event.key === 'ArrowUp') {
      event.preventDefault()
      if (filtered.length) activeIndex = (activeIndex - 1 + filtered.length) % filtered.length
    } else if (event.key === 'Enter' && filtered[activeIndex]) {
      event.preventDefault()
      void execute(filtered[activeIndex])
    }
  }

  onMount(() => {
    inputEl?.focus()
    void Promise.all([
      listarImoveis(true),
      listarInquilinos(true),
      listarLocacoes(),
      listarEmpreendimentos(true),
    ])
      .then(([properties, tenants, leases, projects]) => {
        const entries: PaletteItem[] = []
        for (const p of properties || []) {
          entries.push({
            id: `im-${p.id}`,
            group: 'Imóveis',
            label: p.apelido,
            sub: p.endereco || (p.ativo ? 'Ativo' : 'Arquivado'),
            icon: 'properties',
            run: () => {
              setSetting('imoveis/search', p.apelido)
              setSetting('imoveis/category_index', p.ativo ? 0 : 4)
              onNavigate('imoveis')
            },
          })
        }
        for (const t of tenants || []) {
          entries.push({
            id: `in-${t.id}`,
            group: 'Inquilinos',
            label: t.nome,
            sub: t.cpf || t.telefone || '',
            icon: 'tenants',
            run: () => {
              setSetting('inquilinos/search', t.nome)
              setSetting('inquilinos/status_index', t.ativo ? 0 : 2)
              onNavigate('inquilinos')
            },
          })
        }
        for (const l of leases || []) {
          entries.push({
            id: `lc-${l.id}`,
            group: 'Locações',
            label: `${l.imovel} · ${l.inquilino}`,
            sub: `Contrato #${l.id} (${l.status})`,
            icon: 'leases',
            run: () => {
              setSetting('locacoes/search', l.imovel)
              setSetting('locacoes/status_filter', 'todas')
              onNavigate('locacoes')
            },
          })
        }
        for (const pr of projects || []) {
          entries.push({
            id: `pr-${pr.id}`,
            group: 'Empreendimentos',
            label: pr.nome,
            sub: pr.endereco || '',
            icon: 'projects',
            run: () => {
              setSetting('empreendimentos/search', pr.nome)
              setSetting('empreendimentos/status_index', 2)
              onNavigate('empreendimentos')
            },
          })
        }
        dynamicItems = entries
      })
      .catch(() => {})
  })
</script>

<div class="modal-backdrop" role="presentation" on:click|self={onClose} on:keydown={onKeydown}>
  <div class="palette" role="dialog" aria-modal="true" aria-label="Busca rápida e comandos">
    <div class="palette-input-wrap">
      <Icon name="search" size={17} />
      <input
        bind:this={inputEl}
        bind:value={query}
        on:input={() => (activeIndex = 0)}
        placeholder="Buscar imóvel, inquilino, contrato ou ação…"
        aria-label="Buscar comando"
      />
      <span class="kbd light">ESC</span>
    </div>

    <div class="palette-list">
      {#if filtered.length === 0}
        <div class="empty">Nenhum resultado para «{query}».</div>
      {:else}
        {#each filtered as item, index (item.id)}
          <button
            type="button"
            class="palette-item"
            class:selected={index === activeIndex}
            on:mouseenter={() => (activeIndex = index)}
            on:click={() => execute(item)}
          >
            <Icon name={item.icon} size={16} />
            <div class="palette-text">
              <span class="palette-label">{item.label}</span>
              {#if item.sub}<span class="palette-sub">{item.sub}</span>{/if}
            </div>
            <span class="palette-group">{item.group}</span>
            {#if item.shortcut}<span class="kbd light">{item.shortcut}</span>{/if}
          </button>
        {/each}
      {/if}
    </div>
  </div>
</div>

<style>
  .palette {
    background: #fff;
    border-radius: 12px;
    box-shadow: var(--shadow);
    width: min(620px, 95vw);
    max-height: 75vh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    align-self: flex-start;
    margin-top: 7vh;
    border: 1px solid var(--border);
  }
  .palette-input-wrap {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 13px 16px;
    border-bottom: 1px solid var(--border);
    color: var(--muted);
  }
  .palette-input-wrap input {
    flex: 1;
    border: 0;
    outline: none;
    font-size: 14px;
    background: transparent;
    color: var(--text);
  }
  .palette-list {
    overflow-y: auto;
    padding: 6px;
    max-height: 56vh;
  }
  .palette-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    border: 0;
    background: transparent;
    padding: 9px 11px;
    border-radius: 8px;
    text-align: left;
    color: var(--text);
  }
  .palette-item.selected {
    background: #e8f0f8;
    color: var(--primary-dark);
  }
  .palette-text {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
  }
  .palette-label {
    font-weight: 600;
    font-size: 13px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .palette-sub {
    color: var(--muted);
    font-size: 11.5px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .palette-group {
    font-size: 11px;
    color: var(--muted);
    background: var(--surface-alt);
    border: 1px solid var(--border);
    padding: 1px 7px;
    border-radius: 999px;
    white-space: nowrap;
  }
</style>
