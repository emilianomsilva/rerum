<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '../components/Icon.svelte'
  import ImportPreviewDialog from '../dialogs/ImportPreviewDialog.svelte'
  import {
    backendInfo,
    escolherPlanilha,
    openFileDialog,
    restoreBackup,
    salvarBackupCompleto,
    salvarBackupSqlite,
  } from '../lib/api'
  import { confirmAction } from '../lib/confirm'
  import { getSetting, setSetting } from '../lib/settings'
  import { error as notifyError, errorMessage, success } from '../lib/toast'
  import type { BackendDiagnostics, ImportPreview } from '../lib/types'

  export let onDataChanged: () => void = () => {}

  type SettingsTab = 'dados' | 'visao_geral' | 'sobre'

  const CARD_CATALOG: Array<[string, string]> = [
    ['recebido_mes', 'Receita bruta recebida no período'],
    ['a_receber_mes', 'A receber no período (competência/vencimento)'],
    ['vencidas', 'Cobranças vencidas com saldo pendente'],
    ['ocupacao', 'Taxa de ocupação de imóveis (%)'],
    ['resultado_mes', 'Resultado operacional líquido'],
    ['despesas_mes', 'Despesas operacionais no período'],
    ['capital_investido', 'Patrimônio investido total'],
    ['roi_12m', 'Retorno anualizado da carteira (12m)'],
  ]
  const DEFAULT_CARDS = ['recebido_mes', 'a_receber_mes', 'vencidas', 'ocupacao']

  let activeTab = getSetting<SettingsTab>('configuracoes/tab', 'dados')
  $: setSetting('configuracoes/tab', activeTab)
  let infoData: BackendDiagnostics | null = null
  let busy = false
  let importPreview: { path: string; preview: ImportPreview } | null = null

  let visibleCards: string[] = loadSavedCards()
  let renewalNoticeDays = Number(getSetting('dashboard/renewal_notice_days', 60)) || 60

  function loadSavedCards(): string[] {
    const raw = getSetting<unknown>('dashboard/visible_cards', DEFAULT_CARDS)
    const list = Array.isArray(raw) ? raw.map(String) : String(raw || '').split(',').filter(Boolean)
    return list.length ? list : [...DEFAULT_CARDS]
  }

  function toggleCard(key: string): void {
    if (visibleCards.includes(key)) {
      if (visibleCards.length === 1) return
      visibleCards = visibleCards.filter((k) => k !== key)
    } else {
      visibleCards = [...visibleCards, key]
    }
    setSetting('dashboard/visible_cards', visibleCards)
    onDataChanged()
  }

  function resetDashboardPrefs(): void {
    visibleCards = [...DEFAULT_CARDS]
    renewalNoticeDays = 60
    setSetting('dashboard/visible_cards', visibleCards)
    setSetting('dashboard/renewal_notice_days', renewalNoticeDays)
    onDataChanged()
    success('Preferências da Visão Geral restauradas ao padrão.')
  }

  function updateRenewalDays(val: number): void {
    renewalNoticeDays = val
    setSetting('dashboard/renewal_notice_days', val)
    onDataChanged()
  }

  async function loadInfo(): Promise<void> {
    try {
      infoData = await backendInfo()
    } catch (err) {
      notifyError(errorMessage(err))
    }
  }

  onMount(loadInfo)

  async function handleChooseSpreadsheet(): Promise<void> {
    busy = true
    try {
      const result = await escolherPlanilha()
      if (result) {
        importPreview = result
      }
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function handleBackupFull(): Promise<void> {
    busy = true
    try {
      const saved = await salvarBackupCompleto()
      if (saved) success(`Backup completo salvo com sucesso em:\n${saved}`)
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function handleBackupSqlite(): Promise<void> {
    busy = true
    try {
      const saved = await salvarBackupSqlite()
      if (saved) success(`Backup SQLite salvo com sucesso em:\n${saved}`)
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }

  async function handleRestoreBackup(): Promise<void> {
    const path = await openFileDialog('Selecionar backup completo (.zip)', '*.zip')
    if (!path) return
    const ok = await confirmAction({
      title: 'Restaurar backup completo',
      message:
        'A restauração substitui integralmente o banco de dados e a pasta de recibos atuais pelos dados do arquivo escolhido.\n\n' +
        'Um backup de segurança do estado atual será criado automaticamente antes da troca. Deseja continuar?',
      confirmLabel: 'Restaurar backup',
      danger: true,
    })
    if (!ok) return
    busy = true
    try {
      const safety = await restoreBackup(path)
      success(
        safety
          ? `Backup restaurado com sucesso. Cópia de segurança salva em: ${safety}`
          : 'Backup restaurado com sucesso.',
      )
      onDataChanged()
      await loadInfo()
    } catch (err) {
      notifyError(errorMessage(err))
    } finally {
      busy = false
    }
  }
</script>

<div class="page">
  <div class="page-header">
    <div>
      <h1 class="page-title">Configurações</h1>
      <p class="page-subtitle">
        Operações protegidas de backup/importação, personalização dos indicadores da Visão Geral e diagnóstico do sistema.
      </p>
    </div>
    <div class="segmented" role="tablist">
      <button class:active={activeTab === 'dados'} on:click={() => (activeTab = 'dados')} type="button">
        Dados e Backup
      </button>
      <button class:active={activeTab === 'visao_geral'} on:click={() => (activeTab = 'visao_geral')} type="button">
        Personalização da Visão Geral
      </button>
      <button class:active={activeTab === 'sobre'} on:click={() => (activeTab = 'sobre')} type="button">
        Diagnóstico e Atalhos
      </button>
    </div>
  </div>

  {#if busy}
    <div class="status-banner info"><span class="spinner"></span> Processando operação…</div>
  {/if}

  {#if activeTab === 'dados'}
    <div class="grid-2">
      <div class="surface-card stack">
        <div class="card-head" style="margin-bottom:4px">
          <h2>Backup e Restauração</h2>
        </div>
        <p class="muted">
          Exporte uma cópia completa verificada por manifesto SHA-256 (banco SQLite + recibos em PDF) ou apenas um snapshot consistente do banco.
        </p>
        <div class="settings-list">
          <div class="settings-row">
            <div class="settings-text">
              <strong>Backup completo (.zip)</strong>
              <span>Banco SQLite + todos os recibos PDF + manifesto de integridade.</span>
            </div>
            <button class="btn" on:click={handleBackupFull} disabled={busy}>
              <Icon name="download" size={14} /> Exportar ZIP…
            </button>
          </div>
          <div class="settings-row">
            <div class="settings-text">
              <strong>Somente banco de dados (.sqlite)</strong>
              <span>Snapshot atômico via VACUUM INTO para inspeção ou cópia rápida.</span>
            </div>
            <button class="btn secondary" on:click={handleBackupSqlite} disabled={busy}>
              <Icon name="download" size={14} /> Exportar SQLite…
            </button>
          </div>
          <div class="settings-row">
            <div class="settings-text">
              <strong>Restaurar backup completo (.zip)</strong>
              <span>Cria backup automático do estado atual antes de restaurar o arquivo escolhido.</span>
            </div>
            <button class="btn destructive" on:click={handleRestoreBackup} disabled={busy}>
              <Icon name="upload" size={14} /> Restaurar…
            </button>
          </div>
        </div>
      </div>

      <div class="surface-card stack">
        <div class="card-head" style="margin-bottom:4px">
          <h2>Importação de Planilha Excel</h2>
        </div>
        <p class="muted">
          Importe imóveis, inquilinos, locações, investimentos e débitos a partir da planilha de gerenciamento (.xlsx), com conferência prévia antes de gravar.
        </p>
        <div class="settings-row">
          <div class="settings-text">
            <strong>Planilha de gerenciamento (.xlsx)</strong>
            <span>Exibe resumo de abas, totais e avisos antes de executar a importação.</span>
          </div>
          <button class="btn" on:click={handleChooseSpreadsheet} disabled={busy}>
            <Icon name="upload" size={14} /> Selecionar planilha…
          </button>
        </div>
      </div>
    </div>
  {:else if activeTab === 'visao_geral'}
    <div class="surface-card stack">
      <div class="card-head">
        <div>
          <h2>Indicadores exibidos na Visão Geral</h2>
          <p class="muted" style="margin-top:3px">
            Escolha quais cartões de KPI aparecem no topo da tela inicial (ao menos 1 cartão permanece ativo).
          </p>
        </div>
        <button class="btn secondary small" type="button" on:click={resetDashboardPrefs}>
          Restaurar padrão
        </button>
      </div>

      <div class="settings-cards">
        {#each CARD_CATALOG as [key, label]}
          <label class="settings-row" style="cursor:pointer">
            <span class="flex" style="gap:10px">
              <input
                type="checkbox"
                checked={visibleCards.includes(key)}
                on:change={() => toggleCard(key)}
              />
              <span style="font-weight:550">{label}</span>
            </span>
          </label>
        {/each}
      </div>

      <div class="form-section">
        <div class="form-section-title">Alertas de renovação de contrato</div>
        <div class="toolbar" style="margin-bottom:0">
          <span class="toolbar-label">Notificar contratos ativos que vencem em até:</span>
          <select
            class="select"
            style="width:190px"
            value={renewalNoticeDays}
            on:change={(e) => updateRenewalDays(Number(e.currentTarget.value))}
            aria-label="Prazo de alerta de renovação"
          >
            <option value={30}>30 dias</option>
            <option value={60}>60 dias (Padrão)</option>
            <option value={90}>90 dias</option>
            <option value={120}>120 dias</option>
          </select>
        </div>
      </div>
    </div>
  {:else}
    <div class="grid-2">
      <div class="surface-card stack">
        <div class="card-head" style="margin-bottom:4px">
          <h2>Diagnóstico do Backend In-Process</h2>
        </div>
        {#if infoData}
          <div class="kv-grid">
            <div><div class="muted">Transporte</div><code>{infoData.base_url}</code></div>
            <div><div class="muted">Modo de execução</div><strong>{infoData.managed ? 'Go In-Process (Wails v2)' : 'HTTP Externo (RERUM_BACKEND_URL)'}</strong></div>
            <div style="grid-column: 1 / -1"><div class="muted">Banco SQLite ativo</div><code>{infoData.db_path}</code></div>
          </div>
        {:else}
          <p class="muted">Carregando informações…</p>
        {/if}
      </div>

      <div class="surface-card stack">
        <div class="card-head" style="margin-bottom:4px">
          <h2>Atalhos de Teclado</h2>
        </div>
        <div class="settings-list">
          <div class="settings-row"><span>Busca rápida e paleta de comandos</span><span class="kbd light">Ctrl + K</span></div>
          <div class="settings-row"><span>Focar campo de busca da tela atual</span><span class="kbd light">/</span></div>
          <div class="settings-row"><span>Atualizar dados da tela atual</span><span class="kbd light">F5</span></div>
          <div class="settings-row"><span>Navegar entre as seções principais</span><span class="kbd light">Ctrl + 1 … 8</span></div>
          <div class="settings-row"><span>Abrir Configurações</span><span class="kbd light">Ctrl + ,</span></div>
        </div>
      </div>
    </div>
  {/if}
</div>

{#if importPreview}
  <ImportPreviewDialog
    path={importPreview.path}
    preview={importPreview.preview}
    onClose={() => (importPreview = null)}
    onImported={() => {
      importPreview = null
      onDataChanged()
    }}
  />
{/if}
