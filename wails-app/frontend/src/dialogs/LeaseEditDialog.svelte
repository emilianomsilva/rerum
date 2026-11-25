<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { atualizarLocacao, renovarLocacao } from '../lib/api'
  import { dateBR, endByDuration, money } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'
  import type { EditableLease } from '../lib/types'

  export let lease: EditableLease
  export let mode: 'edit' | 'renew' = 'edit'
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  let form = {
    intermediario: lease?.intermediario || 'Contrato direto',
    dia_vencimento: lease?.dia_vencimento || 5,
    valor_mensal_c: lease?.valor_mensal_c || 0,
    observacao: lease?.observacao || '',
    novo_fim: endByDuration(lease?.fim || new Date(), 12),
    novo_valor_mensal_c: lease?.valor_mensal_c || 0,
  }
  let saving = false
  let message = ''

  async function save(): Promise<void> {
    message = ''
    if (mode === 'renew') {
      if (!form.novo_fim) {
        message = 'Informe o novo término.'
        return
      }
      saving = true
      try {
        const result = await renovarLocacao(lease.id, {
          novo_fim: form.novo_fim,
          novo_valor_mensal_c: form.novo_valor_mensal_c,
        })
        success(`Contrato renovado. ${result?.parcelas?.length ?? 0} novas parcelas geradas.`)
        onSaved()
        onClose()
      } catch (err) {
        message = errorMessage(err)
      } finally {
        saving = false
      }
      return
    }
    if (!form.valor_mensal_c || form.valor_mensal_c <= 0) {
      message = 'Informe o aluguel mensal.'
      return
    }
    saving = true
    try {
      await atualizarLocacao(lease.id, {
        inicio: lease.inicio,
        fim: lease.fim,
        multa_pct: lease.multa_pct,
        juros_dia_pct: lease.juros_dia_pct,
        reajuste_indice: lease.reajuste_indice,
        intermediario: form.intermediario.trim() || 'Contrato direto',
        observacao: form.observacao.trim(),
        dia_vencimento: Number(form.dia_vencimento),
        valor_mensal_c: form.valor_mensal_c,
      })
      success('Contrato atualizado.')
      onSaved()
      onClose()
    } catch (err) {
      message = errorMessage(err)
    } finally {
      saving = false
    }
  }

  $: subtitleText = lease?.imovel && lease?.inquilino ? `${lease.imovel} · ${lease.inquilino}` : ''
</script>

<Modal
  title={mode === 'renew' ? 'Renovar locação' : 'Editar locação'}
  subtitle={subtitleText}
  onClose={onClose}
>
  {#if message}<div class="form-error">{message}</div>{/if}
  {#if mode === 'renew'}
    <div class="payment-summary">
      <span class="label">Término atual</span><span class="value">{dateBR(lease.fim)}</span>
      <span class="label">Aluguel atual</span><span class="value">{money(lease.valor_mensal_c)}</span>
    </div>
    <div class="form-grid">
      <div class="field">
        <label for="le-novofim">Novo término do contrato</label>
        <input id="le-novofim" class="input" type="date" bind:value={form.novo_fim} />
      </div>
      <div class="field">
        <span class="label">Novo aluguel mensal</span>
        <MoneyInput bind:value={form.novo_valor_mensal_c} />
      </div>
    </div>
    <p class="helper mt-8">As parcelas futuras serão geradas pelo backend a partir do novo período.</p>
  {:else}
    <div class="form-grid">
      <div class="field full">
        <label for="le-inter">Intermediário / origem</label>
        <input id="le-inter" class="input" bind:value={form.intermediario} />
      </div>
      <div class="field">
        <label for="le-dia">Dia de vencimento</label>
        <input id="le-dia" class="input" type="number" min="1" max="31" bind:value={form.dia_vencimento} />
      </div>
      <div class="field">
        <span class="label">Aluguel mensal</span>
        <MoneyInput bind:value={form.valor_mensal_c} />
      </div>
      <div class="field full">
        <label for="le-obs">Observações</label>
        <textarea id="le-obs" class="textarea" rows="2" bind:value={form.observacao}></textarea>
      </div>
    </div>
  {/if}
  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={save} disabled={saving}>{saving ? 'Salvando…' : mode === 'renew' ? 'Renovar e gerar parcelas' : 'Salvar alterações'}</button>
  </div>
</Modal>
