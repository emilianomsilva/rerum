<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { criarDivida, atualizarDivida } from '../lib/api'
  import { todayISO } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'
  import type { Divida } from '../lib/types'

  export let debt: Divida | null = null
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  let form = {
    credor: debt?.credor ?? '',
    descricao: debt?.descricao ?? '',
    valor_original_c: debt?.valor_original_c ?? 0,
    data_contratacao: debt?.data_contratacao ? String(debt.data_contratacao).slice(0, 10) : todayISO(),
    tem_vencimento: Boolean(debt?.vencimento),
    vencimento: debt?.vencimento ? String(debt.vencimento).slice(0, 10) : todayISO(),
    observacao: debt?.observacao ?? '',
  }
  let saving = false
  let message = ''

  async function save(): Promise<void> {
    message = ''
    if (!form.credor.trim()) {
      message = 'Informe o credor.'
      return
    }
    if (form.valor_original_c <= 0) {
      message = 'O valor contratado deve ser maior que zero.'
      return
    }
    saving = true
    try {
      const payload: Divida = {
        id: debt?.id ?? 0,
        credor: form.credor.trim(),
        descricao: form.descricao.trim(),
        valor_original_c: form.valor_original_c,
        data_contratacao: form.data_contratacao,
        vencimento: form.tem_vencimento ? form.vencimento : null,
        observacao: form.observacao.trim(),
        ativa: debt?.ativa ?? true,
      }
      if (debt) await atualizarDivida(payload)
      else await criarDivida(payload)
      success(debt ? 'Débito atualizado.' : 'Débito cadastrado.')
      onSaved()
      onClose()
    } catch (err) {
      message = errorMessage(err)
    } finally {
      saving = false
    }
  }
</script>

<Modal title={debt ? 'Editar débito' : 'Novo débito / empréstimo'} onClose={onClose}>
  {#if message}<div class="form-error">{message}</div>{/if}
  <div class="form-grid">
    <div class="field full">
      <label for="db-credor">Credor / instituição *</label>
      <input id="db-credor" class="input" bind:value={form.credor} />
    </div>
    <div class="field full">
      <label for="db-desc">Descrição</label>
      <input id="db-desc" class="input" bind:value={form.descricao} />
    </div>
    <div class="field">
      <span class="label">Valor contratado</span>
      <MoneyInput bind:value={form.valor_original_c} />
    </div>
    <div class="field">
      <label for="db-data">Data de contratação</label>
      <input id="db-data" class="input" type="date" bind:value={form.data_contratacao} />
    </div>
    <div class="field full">
      <label class="checkbox-line"><input type="checkbox" bind:checked={form.tem_vencimento} /> Possui data de vencimento</label>
    </div>
    {#if form.tem_vencimento}
      <div class="field">
        <label for="db-venc">Vencimento</label>
        <input id="db-venc" class="input" type="date" bind:value={form.vencimento} />
      </div>
    {/if}
    <div class="field full">
      <label for="db-obs">Observações</label>
      <textarea id="db-obs" class="textarea" rows="2" bind:value={form.observacao}></textarea>
    </div>
  </div>
  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={save} disabled={saving}>{saving ? 'Salvando…' : 'Salvar débito'}</button>
  </div>
</Modal>
