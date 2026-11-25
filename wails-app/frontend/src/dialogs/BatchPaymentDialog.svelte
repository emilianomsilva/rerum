<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { quitarLoteIndividual, simularParcela } from '../lib/api'
  import { dateBR, money } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'
  import type { BatchPaymentItem, Parcela } from '../lib/types'

  interface BatchRow {
    parcela: Parcela
    base_c: number
    multa_c: number
    juros_c: number
    desconto_c: number
    data: string
    valor_pago_c: number
    manual: boolean
  }

  export let items: BatchPaymentItem[] = []
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  let forma = 'pix'
  let rows: BatchRow[] = items.map(({ parcela, simulation }) => ({
    parcela,
    base_c: simulation.base_c,
    multa_c: simulation.multa_c,
    juros_c: simulation.juros_c,
    desconto_c: parcela.desconto_c || 0,
    data: parcela.vencimento ? String(parcela.vencimento).slice(0, 10) : new Date().toISOString().slice(0, 10),
    valor_pago_c: Math.max(0, simulation.total_c - (parcela.valor_pago_c || 0)),
    manual: false,
  }))
  let saving = false
  let recalculating = false
  let message = ''

  function total(row: BatchRow): number {
    return row.base_c + row.multa_c + row.juros_c - row.desconto_c
  }
  function remaining(row: BatchRow): number {
    return total(row) - (row.parcela.valor_pago_c || 0)
  }
  function label(row: BatchRow): string {
    return row.parcela.total_parcelas
      ? `${String(row.parcela.numero).padStart(2, '0')}/${String(row.parcela.total_parcelas).padStart(2, '0')}`
      : 'Extra'
  }
  function markManual(row: BatchRow): void {
    row.manual = true
  }

  async function recalculate(): Promise<void> {
    message = ''
    recalculating = true
    try {
      const results = await Promise.all(rows.map((row) => simularParcela(row.parcela.id, row.data)))
      rows = rows.map((row, index) => {
        const simulation = results[index]
        const updated: BatchRow = {
          ...row,
          base_c: simulation.base_c,
          multa_c: simulation.multa_c,
          juros_c: simulation.juros_c,
        }
        if (!row.manual) {
          updated.valor_pago_c = Math.max(0, simulation.total_c - row.desconto_c - (row.parcela.valor_pago_c || 0))
        }
        return updated
      })
    } catch (err) {
      message = errorMessage(err)
    } finally {
      recalculating = false
    }
  }

  function zeroCharges(): void {
    rows = rows.map((row) => ({
      ...row,
      multa_c: 0,
      juros_c: 0,
      valor_pago_c: row.manual ? row.valor_pago_c : Math.max(0, row.base_c - row.desconto_c - (row.parcela.valor_pago_c || 0)),
    }))
  }

  async function save(): Promise<void> {
    message = ''
    for (const row of rows) {
      if (remaining(row) <= 0) {
        message = `Parcela ${label(row)}: os ajustes devem deixar um saldo positivo.`
        return
      }
      if (!row.valor_pago_c || row.valor_pago_c <= 0) {
        message = `Parcela ${label(row)}: informe um valor recebido maior que zero.`
        return
      }
    }
    saving = true
    try {
      await quitarLoteIndividual(
        rows.map((row) => ({
          parcela_id: row.parcela.id,
          data: row.data,
          forma,
          desconto_c: row.desconto_c,
          valor_pago_c: row.valor_pago_c,
          multa_c: row.multa_c,
          juros_c: row.juros_c,
        })),
      )
      success(`${rows.length} pagamento(s) registrado(s).`)
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
  title="Conferir pagamentos em lote"
  subtitle="Valores recebidos abaixo do saldo geram pagamento parcial. Nada é gravado antes de confirmar."
  wide
  onClose={onClose}
>
  {#if message}<div class="form-error">{message}</div>{/if}
  <div class="toolbar">
    <span class="toolbar-label">Forma de recebimento:</span>
    <select class="select" style="width:150px" bind:value={forma}>
      <option value="pix">Pix</option>
      <option value="dinheiro">Dinheiro</option>
      <option value="ted">TED</option>
      <option value="boleto">Boleto</option>
      <option value="outros">Outros</option>
    </select>
    <button class="btn secondary small" on:click={recalculate} disabled={recalculating || saving}>
      {recalculating ? 'Calculando…' : 'Recalcular pelas datas'}
    </button>
    <button class="btn secondary small" on:click={zeroCharges} disabled={saving}>Zerar multas e juros</button>
  </div>
  <div class="table-wrap" style="max-height:430px">
    <table class="data">
      <thead>
        <tr>
          <th>Parcela</th>
          <th>Imóvel / Inquilino</th>
          <th>Vencimento</th>
          <th class="align-right">Saldo</th>
          <th>Multa</th>
          <th>Juros</th>
          <th>Recebido agora</th>
          <th>Pagamento</th>
        </tr>
      </thead>
      <tbody>
        {#each rows as row}
          <tr>
            <td class="cell-strong">{label(row)}</td>
            <td>
              <div>{row.parcela.imovel || '—'}</div>
              <div class="cell-muted" style="font-size:11px">{row.parcela.inquilino || ''}</div>
            </td>
            <td class="nowrap">{dateBR(row.parcela.vencimento)}</td>
            <td class="align-right cell-mono cell-strong">{money(remaining(row))}</td>
            <td style="min-width:115px"><MoneyInput bind:value={row.multa_c} /></td>
            <td style="min-width:115px"><MoneyInput bind:value={row.juros_c} /></td>
            <td style="min-width:120px" on:input={() => markManual(row)} on:change={() => markManual(row)}>
              <MoneyInput bind:value={row.valor_pago_c} />
            </td>
            <td><input class="input small" type="date" bind:value={row.data} /></td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  <p class="helper mt-8">Ajustes de desconto já gravados na parcela são preservados.</p>
  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={save} disabled={saving}>{saving ? 'Registrando…' : `Confirmar ${rows.length} pagamento(s)`}</button>
  </div>
</Modal>
