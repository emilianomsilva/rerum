<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { adicionarMovimentoDivida } from '../lib/api'
  import { todayISO } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'
  import type { DebtMovementKind } from '../lib/types'

  export let debtId = 0
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  let form: {
    tipo: DebtMovementKind
    data: string
    valor_c: number
    juros_c: number
    forma_pagamento: string
    descricao: string
  } = {
    tipo: 'pagamento',
    data: todayISO(),
    valor_c: 0,
    juros_c: 0,
    forma_pagamento: 'pix',
    descricao: '',
  }
  let saving = false
  let message = ''

  $: isPagamento = form.tipo === 'pagamento'

  async function save(): Promise<void> {
    message = ''
    if (form.valor_c <= 0) {
      message = 'Informe um valor positivo.'
      return
    }
    if (isPagamento && form.juros_c > form.valor_c) {
      message = 'O valor dos juros não pode exceder o valor total do pagamento.'
      return
    }
    saving = true
    try {
      await adicionarMovimentoDivida({
        id: 0,
        divida_id: debtId,
        tipo: form.tipo,
        data: form.data,
        valor_c: form.valor_c,
        juros_c: isPagamento ? form.juros_c : 0,
        forma_pagamento: form.forma_pagamento,
        descricao: form.descricao.trim(),
      })
      success('Movimento registrado.')
      onSaved()
      onClose()
    } catch (err) {
      message = errorMessage(err)
    } finally {
      saving = false
    }
  }
</script>

<Modal title="Lançar pagamento / liberação" onClose={onClose}>
  {#if message}<div class="form-error">{message}</div>{/if}
  <div class="form-grid">
    <div class="field full">
      <label for="dm-tipo">Tipo de operação</label>
      <select id="dm-tipo" class="select" bind:value={form.tipo}>
        <option value="pagamento">Pagamento (amortização)</option>
        <option value="liberacao">Liberação de recurso (aporte)</option>
      </select>
    </div>
    <div class="field">
      <label for="dm-data">Data</label>
      <input id="dm-data" class="input" type="date" bind:value={form.data} />
    </div>
    <div class="field">
      <span class="label">Valor total</span>
      <MoneyInput bind:value={form.valor_c} />
    </div>
    {#if isPagamento}
      <div class="field">
        <span class="label">Juros / encargos</span>
        <MoneyInput bind:value={form.juros_c} />
      </div>
    {/if}
    <div class="field">
      <label for="dm-forma">Forma</label>
      <input id="dm-forma" class="input" list="debt-forms" bind:value={form.forma_pagamento} />
      <datalist id="debt-forms"><option value="pix"></option><option value="dinheiro"></option><option value="ted"></option><option value="boleto"></option><option value="outros"></option></datalist>
    </div>
    <div class="field full">
      <label for="dm-desc">Descrição</label>
      <input id="dm-desc" class="input" bind:value={form.descricao} />
    </div>
  </div>
  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={save} disabled={saving}>{saving ? 'Registrando…' : 'Registrar movimento'}</button>
  </div>
</Modal>
