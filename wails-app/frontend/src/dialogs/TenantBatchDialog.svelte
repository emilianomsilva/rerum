<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import { criarInquilino } from '../lib/api'
  import { formatCPF, formatPhone } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'

  interface TenantBatchRow {
    nome: string
    cpf: string
    telefone: string
    email: string
    observacao: string
  }

  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  let rows: TenantBatchRow[] = [blank()]
  let saving = false
  let message = ''

  function blank(): TenantBatchRow {
    return { nome: '', cpf: '', telefone: '', email: '', observacao: '' }
  }

  function addRow(): void {
    rows = [...rows, blank()]
  }

  function removeRow(index: number): void {
    rows = rows.filter((_, position) => position !== index)
    if (rows.length === 0) rows = [blank()]
  }

  function formatRow(row: TenantBatchRow, kind: 'cpf' | 'tel'): void {
    if (kind === 'cpf') row.cpf = formatCPF(row.cpf)
    else row.telefone = formatPhone(row.telefone)
    rows = rows
  }

  async function paste(): Promise<void> {
    try {
      const text = await navigator.clipboard.readText()
      const parsed: TenantBatchRow[] = text
        .split(/\r?\n/)
        .map((line) => line.split('\t'))
        .filter((cells) => cells.some((cell) => cell.trim()))
        .map((cells) => ({
          nome: (cells[0] || '').trim(),
          cpf: formatCPF(cells[1] || ''),
          telefone: formatPhone(cells[2] || ''),
          email: (cells[3] || '').trim(),
          observacao: (cells[4] || '').trim(),
        }))
      if (parsed.length) rows = parsed
    } catch {
      message = 'Não foi possível ler a área de transferência. Use Ctrl+V nos campos.'
    }
  }

  async function save(): Promise<void> {
    message = ''
    const valid = rows.filter((row) => row.nome.trim())
    if (valid.length === 0) {
      message = 'Informe ao menos um nome.'
      return
    }
    saving = true
    try {
      for (const row of valid) {
        await criarInquilino({
          id: 0,
          nome: row.nome.trim(),
          cpf: row.cpf,
          telefone: row.telefone,
          email: row.email,
          observacao: row.observacao,
          ativo: true,
          rg: '',
          renda_mensal_c: 0,
          local_trabalho: '',
          endereco_atual: '',
        })
      }
      success(`${valid.length} inquilinos cadastrados.`)
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
  title="Cadastrar inquilinos em lote"
  subtitle="Uma linha por inquilino. Nome é obrigatório; CPF e telefone são formatados automaticamente."
  wide
  onClose={onClose}
>
  {#if message}<div class="form-error">{message}</div>{/if}
  <div class="toolbar">
    <button class="btn secondary small" on:click={addRow}>+ Adicionar linha</button>
    <button class="btn secondary small" on:click={paste}>Colar da área de transferência</button>
    <span class="helper">Sequência esperada: Nome, CPF, Telefone, E-mail, Observações.</span>
  </div>
  <div class="table-wrap" style="max-height:440px">
    <table class="data">
      <thead>
        <tr><th style="min-width:190px">Nome *</th><th>CPF</th><th>Telefone</th><th>E-mail</th><th>Observações</th><th></th></tr>
      </thead>
      <tbody>
        {#each rows as row, index}
          <tr>
            <td><input class="input small" bind:value={row.nome} aria-label="Nome" /></td>
            <td><input class="input small" bind:value={row.cpf} on:blur={() => formatRow(row, 'cpf')} placeholder="000.000.000-00" aria-label="CPF" /></td>
            <td><input class="input small" bind:value={row.telefone} on:blur={() => formatRow(row, 'tel')} placeholder="(11) 99999-9999" aria-label="Telefone" /></td>
            <td><input class="input small" bind:value={row.email} aria-label="E-mail" /></td>
            <td><input class="input small" bind:value={row.observacao} aria-label="Observações" /></td>
            <td><button class="btn icon ghost" title="Remover linha" on:click={() => removeRow(index)}>×</button></td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={save} disabled={saving}>{saving ? 'Cadastrando…' : 'Cadastrar inquilinos'}</button>
  </div>
</Modal>
