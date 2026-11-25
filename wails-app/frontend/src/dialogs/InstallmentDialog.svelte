<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { criarParcelaExtra, atualizarParcela } from '../lib/api'
  import { money, todayISO } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'
  import type { LeaseSummary, Parcela } from '../lib/types'

  export let parcela: Parcela | null = null
  export let leases: LeaseSummary[] | null = null
  export let leaseId = 0
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  const tipoOptions = ['mensal', 'inicial', 'taxa', 'seguro', 'entrada', 'multa', 'reforma', 'outros']
  const p = parcela
  let form = {
    locacao_id: p?.locacao_id ?? (leaseId || leases?.[0]?.id || 0),
    vencimento: p?.vencimento ? String(p.vencimento).slice(0, 10) : todayISO(),
    valor_base_c: p?.valor_base_c ?? 0,
    multa_c: p?.multa_c ?? 0,
    juros_c: p?.juros_c ?? 0,
    desconto_c: p?.desconto_c ?? 0,
    tipo: p?.tipo ?? (leases ? 'taxa' : 'mensal'),
    observacao: p?.observacao ?? '',
  }
  let saving = false
  let message = ''

  $: total = form.valor_base_c + form.multa_c + form.juros_c - form.desconto_c
  $: selectedLease = (leases || []).find((item) => item.id === Number(form.locacao_id))

  async function save(): Promise<void> {
    message = ''
    if (!form.locacao_id) {
      message = 'Selecione uma locação.'
      return
    }
    if (form.valor_base_c <= 0 && !p) {
      message = 'Informe um valor base maior que zero.'
      return
    }
    if (form.valor_base_c < 0) {
      message = 'O valor base não pode ser negativo.'
      return
    }
    saving = true
    try {
      if (p) {
        await atualizarParcela(p.id, {
          vencimento: form.vencimento,
          valor_base_c: form.valor_base_c,
          multa_c: form.multa_c,
          juros_c: form.juros_c,
          desconto_c: form.desconto_c,
          tipo: form.tipo,
          observacao: form.observacao.trim(),
        })
        success('Parcela atualizada.')
      } else {
        await criarParcelaExtra({
          id: 0,
          locacao_id: Number(form.locacao_id),
          competencia: form.vencimento.slice(0, 7),
          vencimento: form.vencimento,
          valor_base_c: form.valor_base_c,
          multa_c: form.multa_c,
          juros_c: form.juros_c,
          desconto_c: form.desconto_c,
          valor_pago_c: 0,
          data_pagamento: null,
          forma: null,
          status: 'aberta',
          recibo_path: null,
          numero: 0,
          total_parcelas: 0,
          tipo: form.tipo,
          observacao: form.observacao.trim(),
        })
        success('Cobrança extra lançada.')
      }
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
  title={p ? 'Editar parcela' : leases ? 'Nova parcela' : 'Lançar cobrança extra'}
  subtitle={p ? `${p.imovel || ''} · ${p.inquilino || ''}` : 'A cobrança é somada ao extrato financeiro da locação.'}
  onClose={onClose}
>
  {#if message}<div class="form-error">{message}</div>{/if}
  <div class="form-grid">
    {#if leases && !p}
      <div class="field full">
        <label for="in-locacao">Locação</label>
        <select id="in-locacao" class="select" bind:value={form.locacao_id}>
          {#each leases as lease (lease.id)}
            <option value={lease.id}>{lease.imovel} · {lease.inquilino} · #{lease.id} ({lease.status})</option>
          {/each}
        </select>
      </div>
    {:else if selectedLease}
      <div class="field full">
        <span class="helper">Locação: {selectedLease.imovel} · {selectedLease.inquilino}</span>
      </div>
    {/if}
    <div class="field">
      <label for="in-venc">Vencimento</label>
      <input id="in-venc" class="input" type="date" bind:value={form.vencimento} />
    </div>
    <div class="field">
      <label for="in-tipo">Tipo</label>
      <select id="in-tipo" class="select" bind:value={form.tipo}>
        {#each tipoOptions as tipo}<option value={tipo}>{tipo}</option>{/each}
      </select>
    </div>
    <div class="field"><span class="label">Valor base</span><MoneyInput bind:value={form.valor_base_c} /></div>
    <div class="field"><span class="label">Multa</span><MoneyInput bind:value={form.multa_c} /></div>
    <div class="field"><span class="label">Juros</span><MoneyInput bind:value={form.juros_c} /></div>
    <div class="field"><span class="label">Desconto / comissão</span><MoneyInput bind:value={form.desconto_c} allowNegative /></div>
    <div class="field full"><label for="in-obs">Observação</label><input id="in-obs" class="input" bind:value={form.observacao} /></div>
  </div>
  <div class="payment-summary mt-14">
    <span class="label">Total devido</span><span class="value total">{money(total)}</span>
  </div>
  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={save} disabled={saving}>{saving ? 'Salvando…' : p ? 'Salvar parcela' : 'Lançar cobrança'}</button>
  </div>
</Modal>
