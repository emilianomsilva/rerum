<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { criarInquilino, atualizarInquilino } from '../lib/api'
  import { formatCPF, formatPhone, validateContact } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'
  import type { Inquilino } from '../lib/types'

  export let tenant: Inquilino | null = null
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  let form = {
    nome: tenant?.nome ?? '',
    cpf: tenant?.cpf ?? '',
    telefone: tenant?.telefone ?? '',
    email: tenant?.email ?? '',
    rg: tenant?.rg ?? '',
    renda_mensal_c: tenant?.renda_mensal_c ?? 0,
    local_trabalho: tenant?.local_trabalho ?? '',
    endereco_atual: tenant?.endereco_atual ?? '',
    observacao: tenant?.observacao ?? '',
  }
  let saving = false
  let message = ''

  function formatCpf(): void {
    form.cpf = formatCPF(form.cpf)
  }
  function formatTelefone(): void {
    form.telefone = formatPhone(form.telefone)
  }

  async function save(): Promise<void> {
    message = ''
    if (!form.nome.trim()) {
      message = 'Informe o nome do inquilino.'
      return
    }
    const cpfError = validateContact(form.cpf, 'cpf')
    const phoneError = validateContact(form.telefone, 'telefone')
    if (cpfError) {
      message = cpfError
      return
    }
    if (phoneError) {
      message = phoneError
      return
    }
    saving = true
    try {
      const payload: Inquilino = { id: tenant?.id ?? 0, ...form, ativo: tenant?.ativo ?? true }
      if (tenant) await atualizarInquilino(payload)
      else await criarInquilino(payload)
      success(tenant ? 'Inquilino atualizado.' : 'Inquilino cadastrado.')
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
  title={tenant ? 'Editar inquilino' : 'Novo inquilino'}
  subtitle="Os dados de contato também são usados nos alertas de renovação e no informe de IRPF."
  onClose={onClose}
>
  {#if message}<div class="form-error">{message}</div>{/if}
  <div class="form-grid">
    <div class="field full">
      <label for="tq-nome">Nome *</label>
      <input id="tq-nome" class="input" bind:value={form.nome} />
    </div>
    <div class="field">
      <label for="tq-cpf">CPF</label>
      <input id="tq-cpf" class="input" bind:value={form.cpf} on:blur={formatCpf} placeholder="000.000.000-00" />
    </div>
    <div class="field">
      <label for="tq-rg">RG</label>
      <input id="tq-rg" class="input" bind:value={form.rg} />
    </div>
    <div class="field">
      <label for="tq-tel">Telefone</label>
      <input id="tq-tel" class="input" bind:value={form.telefone} on:blur={formatTelefone} placeholder="(11) 99999-9999" />
    </div>
    <div class="field">
      <label for="tq-email">E-mail</label>
      <input id="tq-email" class="input" type="email" bind:value={form.email} />
    </div>
    <div class="field">
      <span class="label">Renda mensal</span>
      <MoneyInput bind:value={form.renda_mensal_c} />
    </div>
    <div class="field">
      <label for="tq-trabalho">Local de trabalho</label>
      <input id="tq-trabalho" class="input" bind:value={form.local_trabalho} />
    </div>
    <div class="field full">
      <label for="tq-endereco">Endereço atual</label>
      <input id="tq-endereco" class="input" bind:value={form.endereco_atual} />
    </div>
    <div class="field full">
      <label for="tq-obs">Observações</label>
      <textarea id="tq-obs" class="textarea" rows="2" bind:value={form.observacao}></textarea>
    </div>
  </div>
  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={save} disabled={saving}>{saving ? 'Salvando…' : 'Salvar inquilino'}</button>
  </div>
</Modal>
