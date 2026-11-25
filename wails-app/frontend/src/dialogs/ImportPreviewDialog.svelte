<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import { importarPlanilha } from '../lib/api'
  import { money } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'
  import type { ImportExecuteResponse, ImportPreview } from '../lib/types'

  export let path: string
  export let preview: ImportPreview
  export let onClose: () => void = () => {}
  export let onImported: (result: ImportExecuteResponse) => void = () => {}

  let saving = false
  let message = ''

  async function confirmImport(): Promise<void> {
    saving = true
    message = ''
    try {
      const result = await importarPlanilha(path)
      success(`Importação concluída. Backup: ${result.backup}`)
      onImported(result)
      onClose()
    } catch (err) {
      message = errorMessage(err)
    } finally {
      saving = false
    }
  }
</script>

<Modal
  title="Prévia da importação"
  subtitle={preview.source}
  wide
  onClose={onClose}
>
  {#if message}<div class="form-error">{message}</div>{/if}
  {#if preview.already_imported}
    <div class="status-banner info">Esta planilha já consta como importada neste banco.</div>
  {/if}
  <div class="payment-summary" style="grid-template-columns:repeat(3, minmax(0,1fr))">
    <span class="label">Imóveis previstos</span><span class="value">{preview.properties}</span>
    <span class="label">Recebimentos</span><span class="value">{preview.receipts} · {money(preview.receipts_total_c)}</span>
    <span class="label">Investimentos / despesas</span><span class="value">{preview.expenses} · {money(preview.expenses_total_c)}</span>
    <span class="label">Linhas de débitos</span><span class="value">{preview.debts}</span>
    <span class="label">Liberado / pago</span><span class="value">{money(preview.debt_received_c)} / {money(preview.debt_paid_c)}</span>
    <span class="label">Fórmulas em Estatísticas</span><span class="value">{preview.formula_rows}</span>
    <span class="label">Abas</span><span class="value" style="font-size:11px">{(preview.sheets || []).join(', ')}</span>
    <span class="label">SHA-256</span><span class="value mono" style="font-size:9px">{String(preview.sha256).slice(0, 24)}…</span>
  </div>

  <div class="form-section">
    <div class="form-section-title">Pontos para revisão</div>
    {#if preview.warnings?.length}
      <ul class="helper" style="margin:0; padding-left:18px; line-height:1.6">
        {#each preview.warnings as warning}<li>{warning}</li>{/each}
      </ul>
    {:else}
      <p class="helper">Nenhum aviso encontrado.</p>
    {/if}
  </div>
  <p class="helper mt-14">Será criado um backup automático do banco antes de importar.</p>
  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={confirmImport} disabled={saving || preview.already_imported}>
      {saving ? 'Importando…' : 'Importar agora'}
    </button>
  </div>
</Modal>
