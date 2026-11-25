<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { criarLocacao } from '../lib/api'
  import { endByDuration, money, todayISO } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'
  import type { Imovel, Inquilino } from '../lib/types'

  interface BatchLeaseRow {
    checked: boolean
    property: Imovel
    inquilino_id: number
    valor_mensal_c: number
  }

  export let properties: Imovel[] = []
  export let occupiedIds: number[] = []
  export let tenants: Inquilino[] = []
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  $: activeTenants = tenants.filter((t) => t.ativo)
  $: available = properties.filter(
    (item) =>
      item.ativo &&
      item.destinado_locacao &&
      item.situacao_patrimonial !== 'vendido' &&
      item.status === 'disponivel' &&
      !occupiedIds.includes(item.id),
  )

  let inicio = todayISO()
  let duracao = 12
  let dia_vencimento = 5
  let intermediario = 'Contrato direto'
  let valor_padrao_c = 150000

  let rows: BatchLeaseRow[] = []
  let initialized = false
  $: if (!initialized && available.length > 0) {
    initialized = true
    rows = available.map((property) => ({
      checked: true,
      property,
      inquilino_id: activeTenants[0]?.id ?? 0,
      valor_mensal_c: valor_padrao_c,
    }))
  }

  $: fim = endByDuration(inicio, duracao)
  $: selectedRows = rows.filter((r) => r.checked)

  let saving = false
  let message = ''

  function setAllChecked(checked: boolean): void {
    rows = rows.map((r) => ({ ...r, checked }))
  }

  function applyStandardValue(): void {
    rows = rows.map((r) => (r.checked ? { ...r, valor_mensal_c: valor_padrao_c } : r))
  }

  async function save(): Promise<void> {
    message = ''
    if (selectedRows.length === 0) {
      message = 'Marque ao menos um imóvel para lançar contratos.'
      return
    }
    const usedTenants = new Set<number>()
    for (const row of selectedRows) {
      if (!row.inquilino_id) {
        message = `Selecione o inquilino para «${row.property.apelido}».`
        return
      }
      if (row.valor_mensal_c <= 0) {
        message = `Informe um aluguel mensal maior que zero para «${row.property.apelido}».`
        return
      }
      usedTenants.add(row.inquilino_id)
    }
    saving = true
    try {
      for (const row of selectedRows) {
        await criarLocacao({
          imovel_id: row.property.id,
          inquilino_id: Number(row.inquilino_id),
          inicio,
          fim,
          dia_vencimento: Number(dia_vencimento),
          valor_mensal_c: row.valor_mensal_c,
          multa_pct: 2.0,
          juros_dia_pct: 0.033,
          reajuste_indice: 'nenhum',
          intermediario: intermediario.trim() || 'Contrato direto',
          observacao: '',
          primeiro_vencimento: null,
        })
      }
      success(`${selectedRows.length} contrato(s) de locação lançados com sucesso!`)
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
  title="Lançar locações em lote"
  subtitle="Defina as condições gerais padronizadas e vincule os inquilinos às unidades disponíveis."
  wide
  onClose={onClose}
>
  {#if message}<div class="form-error">{message}</div>{/if}

  {#if available.length === 0}
    <div class="status-banner error">Não há imóveis disponíveis e destinados à locação para novos contratos.</div>
  {:else if activeTenants.length === 0}
    <div class="status-banner error">Cadastre ao menos um inquilino ativo antes de lançar locações.</div>
  {:else}
    <div class="form-section" style="margin-top:0; padding-top:0; border-top:0">
      <div class="form-section-title">Condições gerais padronizadas do contrato</div>
      <div class="form-grid">
        <div class="field">
          <label for="lb-inicio">Início</label>
          <input id="lb-inicio" class="input" type="date" bind:value={inicio} />
        </div>
        <div class="field">
          <label for="lb-dur">Duração (meses)</label>
          <input id="lb-dur" class="input" type="number" min="1" max="360" bind:value={duracao} />
        </div>
        <div class="field">
          <label for="lb-dia">Dia de vencimento</label>
          <input id="lb-dia" class="input" type="number" min="1" max="31" bind:value={dia_vencimento} />
        </div>
        <div class="field">
          <label for="lb-inter">Intermediário / origem</label>
          <input id="lb-inter" class="input" bind:value={intermediario} />
        </div>
        <div class="field full">
          <span class="label">Aluguel mensal padrão</span>
          <div class="field-row">
            <div class="grow"><MoneyInput bind:value={valor_padrao_c} /></div>
            <button class="btn secondary" type="button" on:click={applyStandardValue}>
              Aplicar valor às linhas selecionadas
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="form-section">
      <div class="toolbar" style="margin-bottom:8px">
        <span class="form-section-title" style="margin:0">Seleção de unidades e atribuição de inquilinos</span>
        <div class="spacer"></div>
        <span class="helper">Término calculado: <strong>{fim.split('-').reverse().join('/')}</strong></span>
        <button class="btn secondary small" type="button" on:click={() => setAllChecked(true)}>Marcar todos</button>
        <button class="btn secondary small" type="button" on:click={() => setAllChecked(false)}>Desmarcar todos</button>
      </div>

      <div class="table-wrap" style="max-height:360px">
        <table class="data">
          <thead>
            <tr>
              <th class="align-center" style="width:42px">✓</th>
              <th style="width:220px">Imóvel</th>
              <th>Inquilino vinculado *</th>
              <th style="width:190px">Aluguel mensal (R$)</th>
            </tr>
          </thead>
          <tbody>
            {#each rows as row (row.property.id)}
              <tr class:selected={row.checked}>
                <td class="align-center">
                  <input type="checkbox" bind:checked={row.checked} aria-label="Selecionar {row.property.apelido}" />
                </td>
                <td>
                  <div class="cell-strong">{row.property.apelido}</div>
                  <div class="cell-muted" style="font-size:11px">{row.property.endereco || ''}</div>
                </td>
                <td>
                  <select class="select small" bind:value={row.inquilino_id} disabled={!row.checked} aria-label="Inquilino">
                    {#each activeTenants as tenant (tenant.id)}
                      <option value={tenant.id}>{tenant.nome}{tenant.cpf ? ` · ${tenant.cpf}` : ''}</option>
                    {/each}
                  </select>
                </td>
                <td>
                  <MoneyInput bind:value={row.valor_mensal_c} disabled={!row.checked} />
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  {/if}

  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button
      class="btn"
      on:click={save}
      disabled={saving || selectedRows.length === 0 || activeTenants.length === 0}
    >
      {saving
        ? 'Lançando…'
        : `Criar ${selectedRows.length} locação(ões) · ${money(selectedRows.reduce((s, r) => s + r.valor_mensal_c, 0))}/mês`}
    </button>
  </div>
</Modal>
