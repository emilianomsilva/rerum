<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { criarMovimentacao, atualizarMovimentacao } from '../lib/api'
  import { money, todayISO } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'
  import type { Empreendimento, Imovel, Inquilino, Movimento, Rateio } from '../lib/types'

  export let properties: Imovel[] = []
  export let projects: Empreendimento[] = []
  export let tenants: Inquilino[] = []
  export let movement: Movimento | null = null
  export let rateios: Rateio[] = []
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  const m = movement
  let form = {
    tipo: m?.tipo ?? 'despesa',
    natureza: m?.natureza ?? 'operacional',
    categoria: m?.categoria ?? 'outros',
    reversao: m?.reversao ?? false,
    imovel_id: m?.imovel_id ?? 0,
    empreendimento_id: m?.empreendimento_id ?? 0,
    inquilino_id: m?.inquilino_id ?? 0,
    data: m?.data ? String(m.data).slice(0, 10) : todayISO(),
    valor_c: m?.valor_c ?? 0,
    referencia: m?.referencia ?? '',
    forma_pagamento: m?.forma_pagamento ?? 'pix',
    origem: m?.origem ?? '',
    fornecedor: m?.fornecedor ?? '',
    descricao: m?.descricao ?? '',
  }

  let useRateio = rateios.length > 0
  let allocMap: Record<number, { checked: boolean; valor_c: number }> = {}

  for (const p of properties) {
    const existing = rateios.find((r) => r.imovel_id === p.id)
    allocMap[p.id] = {
      checked: Boolean(existing),
      valor_c: existing ? existing.valor_c : 0,
    }
  }

  let saving = false
  let message = ''

  $: categories =
    form.tipo === 'receita'
      ? ['aluguel', 'entrada', 'seguro', 'multa rescisória', 'rendimento financeiro', 'outros']
      : form.natureza === 'investimento'
        ? ['terreno', 'materiais', 'serviços', 'projeto', 'infraestrutura', 'documentos', 'outros']
        : ['iptu', 'condomínio', 'manutenção', 'seguro', 'taxas', 'comissão', 'outros']

  $: selectedProject = projects.find((pr) => pr.id === Number(form.empreendimento_id)) || null
  $: scopeProperties = selectedProject
    ? properties.filter(
        (p) =>
          (selectedProject.unidade_ids || []).includes(p.id) ||
          p.empreendimento_id === selectedProject.id,
      )
    : properties.filter((p) => p.ativo)

  $: canUseRateio = !form.imovel_id && Number(form.empreendimento_id) > 0 && scopeProperties.length > 0
  $: allocatedSum = scopeProperties.reduce(
    (sum, p) => sum + (allocMap[p.id]?.checked ? allocMap[p.id].valor_c : 0),
    0,
  )

  function syncCategory(): void {
    if (!categories.includes(form.categoria)) form.categoria = categories[0]
  }

  function onProjectChange(): void {
    const proj = projects.find((pr) => pr.id === Number(form.empreendimento_id))
    if (proj && !form.imovel_id && (proj.unidade_ids || []).length > 0) {
      const next = { ...allocMap }
      for (const id of proj.unidade_ids) {
        if (!next[id]) next[id] = { checked: true, valor_c: 0 }
        else next[id] = { ...next[id], checked: true }
      }
      allocMap = next
    }
  }

  function splitEqually(): void {
    const chosen = scopeProperties.filter((p) => allocMap[p.id]?.checked)
    if (chosen.length === 0 || form.valor_c <= 0) return
    const base = Math.floor(form.valor_c / chosen.length)
    const remainder = form.valor_c - base * chosen.length
    const next = { ...allocMap }
    chosen.forEach((p, idx) => {
      next[p.id] = { checked: true, valor_c: base + (idx < remainder ? 1 : 0) }
    })
    allocMap = next
  }

  async function save(): Promise<void> {
    message = ''
    if (form.valor_c <= 0) {
      message = 'Informe um valor maior que zero.'
      return
    }
    if (!form.imovel_id && !form.empreendimento_id) {
      message = 'Associe a movimentação a uma unidade ou a um empreendimento.'
      return
    }
    let nextRateios: Rateio[] = []
    if (canUseRateio && useRateio) {
      nextRateios = scopeProperties
        .filter((p) => allocMap[p.id]?.checked && allocMap[p.id].valor_c > 0)
        .map((p) => ({ imovel_id: p.id, valor_c: allocMap[p.id].valor_c }))
      if (nextRateios.length > 0 && allocatedSum !== form.valor_c) {
        message = `A soma do rateio (${money(allocatedSum)}) deve corresponder ao valor total (${money(form.valor_c)}).`
        return
      }
    }
    saving = true
    try {
      const payload: Movimento = {
        id: m?.id ?? 0,
        tipo: form.tipo,
        natureza: form.natureza,
        categoria: form.categoria,
        reversao: form.reversao,
        imovel_id: form.imovel_id ? Number(form.imovel_id) : null,
        empreendimento_id: form.empreendimento_id ? Number(form.empreendimento_id) : null,
        inquilino_id: form.inquilino_id ? Number(form.inquilino_id) : null,
        data: form.data,
        valor_c: form.valor_c,
        referencia: form.referencia.trim(),
        forma_pagamento: form.forma_pagamento,
        origem: form.origem.trim(),
        fornecedor: form.fornecedor.trim(),
        descricao: form.descricao.trim(),
      }
      if (m) await atualizarMovimentacao(payload, nextRateios)
      else await criarMovimentacao(payload, nextRateios)
      success(m ? 'Movimentação atualizada.' : 'Movimentação registrada.')
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
  title={m ? 'Editar movimentação' : 'Nova movimentação'}
  subtitle="Receitas, despesas operacionais e investimentos compõem o livro financeiro."
  wide={canUseRateio && useRateio}
  onClose={onClose}
>
  {#if message}<div class="form-error">{message}</div>{/if}
  <div class="form-grid">
    <div class="field">
      <label for="mv-tipo">Tipo</label>
      <select id="mv-tipo" class="select" bind:value={form.tipo} on:change={syncCategory}>
        <option value="receita">Receita</option>
        <option value="despesa">Despesa</option>
      </select>
    </div>
    <div class="field">
      <label for="mv-nat">Natureza</label>
      <select id="mv-nat" class="select" bind:value={form.natureza} on:change={syncCategory}>
        <option value="operacional">Operacional (rotina)</option>
        <option value="investimento">Investimento / obra / capex</option>
      </select>
    </div>
    <div class="field">
      <label for="mv-cat">Categoria</label>
      <input id="mv-cat" class="input" list="mov-categories" bind:value={form.categoria} />
      <datalist id="mov-categories">{#each categories as category}<option value={category}></option>{/each}</datalist>
    </div>
    <div class="field">
      <label for="mv-data">Data</label>
      <input id="mv-data" class="input" type="date" bind:value={form.data} />
    </div>
    <div class="field">
      <label for="mv-imovel">Unidade</label>
      <select id="mv-imovel" class="select" bind:value={form.imovel_id}>
        <option value={0}>Sem unidade específica</option>
        {#each properties as item (item.id)}<option value={item.id}>{item.apelido}</option>{/each}
      </select>
    </div>
    <div class="field">
      <label for="mv-proj">Empreendimento</label>
      <select id="mv-proj" class="select" bind:value={form.empreendimento_id} on:change={onProjectChange}>
        <option value={0}>Nenhum empreendimento</option>
        {#each projects as project (project.id)}<option value={project.id}>{project.nome}</option>{/each}
      </select>
    </div>
    <div class="field">
      <label for="mv-inq">Inquilino</label>
      <select id="mv-inq" class="select" bind:value={form.inquilino_id}>
        <option value={0}>Sem inquilino</option>
        {#each tenants as tenant (tenant.id)}<option value={tenant.id}>{tenant.nome}{tenant.ativo ? '' : ' (arquivado)'}</option>{/each}
      </select>
    </div>
    <div class="field">
      <span class="label">Valor</span>
      <MoneyInput bind:value={form.valor_c} />
    </div>
    <div class="field">
      <label for="mv-ref">Referência</label>
      <input id="mv-ref" class="input" bind:value={form.referencia} />
    </div>
    <div class="field">
      <label for="mv-forma">Forma de pagamento</label>
      <input id="mv-forma" class="input" list="mov-forms" bind:value={form.forma_pagamento} />
      <datalist id="mov-forms"><option value="pix"></option><option value="dinheiro"></option><option value="ted"></option><option value="boleto"></option><option value="cartão"></option><option value="outros"></option></datalist>
    </div>
    <div class="field">
      <label for="mv-orig">Origem / intermediário</label>
      <input id="mv-orig" class="input" bind:value={form.origem} />
    </div>
    <div class="field">
      <label for="mv-forn">Fornecedor</label>
      <input id="mv-forn" class="input" bind:value={form.fornecedor} />
    </div>
    <div class="field full">
      <label for="mv-desc">Descrição</label>
      <input id="mv-desc" class="input" bind:value={form.descricao} placeholder="Nome do serviço, fornecedor ou especificação" />
    </div>
    {#if form.tipo === 'despesa'}
      <div class="field full">
        <label class="checkbox-line">
          <input type="checkbox" bind:checked={form.reversao} />
          Estorno / reembolso de despesa
        </label>
      </div>
    {/if}
  </div>

  {#if canUseRateio}
    <div class="form-section">
      <div class="toolbar" style="margin-bottom:8px">
        <label class="checkbox-line" style="font-weight:650">
          <input type="checkbox" bind:checked={useRateio} />
          Ratear valor entre as unidades do empreendimento
        </label>
        {#if useRateio}
          <div class="spacer"></div>
          <span class="helper">Rateado: <strong>{money(allocatedSum)}</strong> / {money(form.valor_c)}</span>
          <button class="btn secondary small" type="button" on:click={splitEqually}>
            Dividir igualmente
          </button>
        {/if}
      </div>

      {#if useRateio}
        <div class="table-wrap" style="max-height:220px">
          <table class="data">
            <thead>
              <tr>
                <th class="align-center" style="width:40px">✓</th>
                <th>Unidade</th>
                <th style="width:190px">Valor alocado</th>
              </tr>
            </thead>
            <tbody>
              {#each scopeProperties as prop (prop.id)}
                {#if allocMap[prop.id]}
                  <tr>
                    <td class="align-center">
                      <input type="checkbox" bind:checked={allocMap[prop.id].checked} />
                    </td>
                    <td>{prop.apelido}</td>
                    <td>
                      <MoneyInput bind:value={allocMap[prop.id].valor_c} disabled={!allocMap[prop.id].checked} />
                    </td>
                  </tr>
                {/if}
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {/if}

  <div class="payment-summary mt-14">
    <span class="label">Valor da movimentação</span><span class="value total">{money(form.valor_c)}</span>
  </div>
  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={save} disabled={saving}>{saving ? 'Salvando…' : 'Salvar movimentação'}</button>
  </div>
</Modal>
