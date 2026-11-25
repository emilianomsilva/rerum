<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { quitarParcela, simularParcela } from '../lib/api'
  import { dateBR, money, todayISO } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'
  import type { Parcela, Simulation } from '../lib/types'

  export let parcela: Parcela
  export let simulation: Simulation
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  let form = {
    base_c: simulation.base_c,
    multa_c: simulation.multa_c,
    juros_c: simulation.juros_c,
    desconto_c: parcela.desconto_c || 0,
    data: parcela.vencimento ? String(parcela.vencimento).slice(0, 10) : todayISO(),
    forma: 'pix',
    valor_pago_c: 0,
  }
  let manualReceived = false
  let saving = false
  let recalculating = false
  let message = ''

  $: total = form.base_c + form.multa_c + form.juros_c - form.desconto_c
  $: remaining = total - (parcela.valor_pago_c || 0)
  $: if (!manualReceived) {
    form.valor_pago_c = Math.max(0, remaining)
  }

  function receivedChanged(): void {
    manualReceived = true
  }

  async function recalculate(): Promise<void> {
    message = ''
    recalculating = true
    try {
      const next = await simularParcela(parcela.id, form.data)
      form.base_c = next.base_c
      form.multa_c = next.multa_c
      form.juros_c = next.juros_c
      if (!manualReceived) form.valor_pago_c = Math.max(0, next.total_c - form.desconto_c - (parcela.valor_pago_c || 0))
    } catch (err) {
      message = errorMessage(err)
    } finally {
      recalculating = false
    }
  }

  async function save(): Promise<void> {
    message = ''
    if (remaining <= 0) {
      message = 'Os ajustes devem deixar um saldo positivo para receber.'
      return
    }
    if (!form.valor_pago_c || form.valor_pago_c <= 0) {
      message = 'Informe um valor recebido maior que zero.'
      return
    }
    saving = true
    try {
      await quitarParcela(parcela.id, {
        data: form.data,
        forma: form.forma,
        desconto_c: form.desconto_c,
        valor_pago_c: form.valor_pago_c,
        multa_c: form.multa_c,
        juros_c: form.juros_c,
      })
      success('Pagamento registrado com sucesso.')
      onSaved()
      onClose()
    } catch (err) {
      message = errorMessage(err)
    } finally {
      saving = false
    }
  }
</script>

<Modal
  title="Quitar parcela de aluguel"
  subtitle={`${parcela.imovel || ''} · ${parcela.inquilino || ''}`}
  onClose={onClose}
>
  {#if message}<div class="form-error">{message}</div>{/if}
  <div class="payment-summary">
    <span class="label">Valor base</span><span class="value">{money(form.base_c)}</span>
    <span class="label">Já recebido</span><span class="value">{money(parcela.valor_pago_c)}</span>
    <span class="label">Vencimento</span><span class="value">{dateBR(parcela.vencimento)}</span>
    <span class="label">Dias em atraso</span><span class="value">{simulation.dias_atraso}</span>
  </div>

  <div class="form-grid">
    <div class="field"><span class="label">Multa (R$)</span><MoneyInput bind:value={form.multa_c} /></div>
    <div class="field"><span class="label">Juros (R$)</span><MoneyInput bind:value={form.juros_c} /></div>
    <div class="field"><span class="label">Desconto / acréscimo</span><MoneyInput bind:value={form.desconto_c} allowNegative /></div>
    <div class="field"><label for="py-data">Data de pagamento</label><input id="py-data" class="input" type="date" bind:value={form.data} /></div>
    <div class="field"><label for="py-forma">Forma de recebimento</label>
      <select id="py-forma" class="select" bind:value={form.forma}>
        <option value="pix">Pix</option>
        <option value="dinheiro">Dinheiro</option>
        <option value="ted">TED</option>
        <option value="boleto">Boleto</option>
        <option value="outros">Outros</option>
      </select>
    </div>
    <div class="field">
      <span class="label">Valor recebido agora</span>
      <div on:input={receivedChanged} on:change={receivedChanged}>
        <MoneyInput bind:value={form.valor_pago_c} />
      </div>
    </div>
  </div>

  <p class="helper mt-8">Multa e juros são editáveis. Ao mudar a data, recalcule os encargos ou ajuste-os manualmente.</p>
  <div class="flex mt-8">
    <button class="btn secondary" on:click={recalculate} disabled={recalculating || saving}>
      {recalculating ? 'Calculando…' : 'Recalcular multas e juros pelas datas'}
    </button>
  </div>

  <div class="payment-summary mt-14">
    <span class="label">Total devido</span><span class="value">{money(total)}</span>
    <span class="label">Saldo a receber</span><span class="value total">{money(remaining)}</span>
  </div>

  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={save} disabled={saving}>{saving ? 'Registrando…' : 'Confirmar pagamento'}</button>
  </div>
</Modal>
