<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { criarLocacao } from '../lib/api'
  import { endByDuration, money, todayISO } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'
  import type { Imovel, Inquilino } from '../lib/types'

  export let properties: Imovel[] = []
  export let occupiedIds: number[] = []
  export let tenants: Inquilino[] = []
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  $: available = properties.filter(
    (item) =>
      item.ativo &&
      item.destinado_locacao &&
      item.situacao_patrimonial !== 'vendido' &&
      item.status === 'disponivel' &&
      !occupiedIds.includes(item.id),
  )
  $: activeTenants = tenants.filter((item) => item.ativo)

  let form = {
    imovel_id: 0,
    inquilino_id: 0,
    inicio: todayISO(),
    duracao: 12,
    dia_vencimento: 5,
    valor_mensal_c: 0,
    intermediario: 'Contrato direto',
    observacao: '',
    tem_primeiro_vencimento: false,
    primeiro_vencimento: todayISO(),
  }

  $: if (!form.imovel_id && available.length > 0) {
    form.imovel_id = available[0].id
  }
  $: if (!form.inquilino_id && activeTenants.length > 0) {
    form.inquilino_id = activeTenants[0].id
  }

  let saving = false
  let message = ''

  $: fim = endByDuration(form.inicio, form.duracao)

  async function save(): Promise<void> {
    message = ''
    if (!form.imovel_id) {
      message = 'Selecione um imóvel disponível.'
      return
    }
    if (!form.inquilino_id) {
      message = 'Selecione um inquilino ativo.'
      return
    }
    if (!form.valor_mensal_c || form.valor_mensal_c <= 0) {
      message = 'Informe o aluguel mensal.'
      return
    }
    saving = true
    try {
      const result = await criarLocacao({
        imovel_id: Number(form.imovel_id),
        inquilino_id: Number(form.inquilino_id),
        inicio: form.inicio,
        fim,
        dia_vencimento: Number(form.dia_vencimento),
        valor_mensal_c: form.valor_mensal_c,
        multa_pct: 2.0,
        juros_dia_pct: 0.033,
        reajuste_indice: 'nenhum',
        intermediario: form.intermediario.trim() || 'Contrato direto',
        observacao: form.observacao.trim(),
        primeiro_vencimento: form.tem_primeiro_vencimento ? form.primeiro_vencimento : null,
      })
      success(`Locação criada com ${result?.parcelas?.length ?? 0} parcelas.`)
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
  title="Nova locação"
  subtitle="O backend gera automaticamente as parcelas mensais do contrato."
  onClose={onClose}
>
  {#if message}<div class="form-error">{message}</div>{/if}
  {#if available.length === 0}
    <div class="status-banner error">Não há imóveis disponíveis e destinados à locação.</div>
  {:else if activeTenants.length === 0}
    <div class="status-banner error">Cadastre ao menos um inquilino ativo antes de criar locações.</div>
  {:else}
    <div class="form-grid">
      <div class="field full">
        <label for="lc-imovel">Imóvel</label>
        <select id="lc-imovel" class="select" bind:value={form.imovel_id}>
          {#each available as item (item.id)}
            <option value={item.id}>{item.apelido} — {item.endereco || 'sem endereço'}</option>
          {/each}
        </select>
      </div>
      <div class="field full">
        <label for="lc-inquilino">Inquilino</label>
        <select id="lc-inquilino" class="select" bind:value={form.inquilino_id}>
          {#each activeTenants as tenant (tenant.id)}
            <option value={tenant.id}>{tenant.nome}{tenant.cpf ? ` · ${tenant.cpf}` : ''}</option>
          {/each}
        </select>
      </div>
      <div class="field">
        <label for="lc-inicio">Início</label>
        <input id="lc-inicio" class="input" type="date" bind:value={form.inicio} />
      </div>
      <div class="field">
        <label for="lc-duracao">Duração (meses)</label>
        <input id="lc-duracao" class="input" type="number" min="1" max="120" bind:value={form.duracao} />
      </div>
      <div class="field">
        <label for="lc-dia">Dia de vencimento</label>
        <input id="lc-dia" class="input" type="number" min="1" max="31" bind:value={form.dia_vencimento} />
      </div>
      <div class="field">
        <span class="label">Aluguel mensal</span>
        <MoneyInput bind:value={form.valor_mensal_c} />
      </div>
      <div class="field full">
        <label for="lc-inter">Intermediário / origem</label>
        <input id="lc-inter" class="input" bind:value={form.intermediario} />
      </div>
      <div class="field full">
        <label class="checkbox-line"><input type="checkbox" bind:checked={form.tem_primeiro_vencimento} /> Definir primeiro vencimento diferente do dia do contrato</label>
      </div>
      {#if form.tem_primeiro_vencimento}
        <div class="field">
          <label for="lc-pvenc">Primeiro vencimento</label>
          <input id="lc-pvenc" class="input" type="date" bind:value={form.primeiro_vencimento} />
        </div>
      {/if}
      <div class="field full">
        <label for="lc-obs">Observações</label>
        <textarea id="lc-obs" class="textarea" rows="2" bind:value={form.observacao}></textarea>
      </div>
    </div>
    <div class="payment-summary" style="margin-top:14px">
      <span class="label">Término calculado</span><span class="value">{fim.split('-').reverse().join('/')}</span>
      <span class="label">Parcelas previstas</span><span class="value">{form.duracao}</span>
      <span class="label">Aluguel</span><span class="value total">{money(form.valor_mensal_c)}</span>
    </div>
  {/if}
  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={save} disabled={saving || available.length === 0 || activeTenants.length === 0}>
      {saving ? 'Criando…' : 'Criar locação'}
    </button>
  </div>
</Modal>
