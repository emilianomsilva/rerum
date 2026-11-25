<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { loteImoveis } from '../lib/api'
  import { money, todayISO } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'
  import type { Empreendimento, PropertyBatch, PropertyKind } from '../lib/types'

  export let projects: Empreendimento[] = []
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  let form: {
    empreendimento_id: number
    tipo: PropertyKind
    endereco_base: string
    bairro: string
    data_aquisicao: string
    destinado_locacao: boolean
    matricula_compartilhada: boolean
    prefixo: string
    sequencia: 'pad2' | 'num' | 'alpha'
    inicio: number
    quantidade: number
    sufixo: string
    numero: string
    matricula: string
    modo_custo: 'dividir' | 'fixo'
    valor_compra_c: number
    reforma_c: number
    outros_custos_c: number
  } = {
    empreendimento_id: 0,
    tipo: 'casa',
    endereco_base: '',
    bairro: '',
    data_aquisicao: todayISO(),
    destinado_locacao: true,
    matricula_compartilhada: false,
    prefixo: 'Casa ',
    sequencia: 'pad2',
    inicio: 1,
    quantidade: 5,
    sufixo: '',
    numero: '',
    matricula: '',
    modo_custo: 'dividir',
    valor_compra_c: 0,
    reforma_c: 0,
    outros_custos_c: 0,
  }
  let saving = false
  let message = ''
  let removed: number[] = []

  function sequenceValue(index: number): string {
    const value = Number(form.inicio) + index
    if (form.sequencia === 'alpha') return String.fromCharCode(64 + value)
    if (form.sequencia === 'num') return String(value)
    return String(value).padStart(2, '0')
  }

  $: count = Math.max(1, Math.min(100, Number(form.quantidade) || 1))
  $: rows = Array.from({ length: count }, (_, index) => {
    const seq = sequenceValue(index)
    const endereco = form.endereco_base.includes('{n}')
      ? form.endereco_base.replaceAll('{n}', String(Number(form.inicio) + index))
      : form.endereco_base
    return {
      apelido: `${form.prefixo}${seq}${form.sufixo}`.trim(),
      endereco,
      numero: form.numero,
      unidade: seq,
      bairro: form.bairro,
      matricula_cartorio: form.matricula,
    }
  }).filter((_, index) => !removed.includes(index))
  $: totalCost = form.valor_compra_c + form.reforma_c + form.outros_custos_c
  $: costPerUnit = form.modo_custo === 'dividir' ? Math.round(totalCost / Math.max(1, rows.length)) : totalCost

  function removeRow(index: number): void {
    const current = rows[index]
    const original = Array.from({ length: count }, (_, i) => i).filter((i) => !removed.includes(i))
    const position = original[rows.indexOf(current)]
    if (position !== undefined) removed = [...removed, position]
  }

  async function save(): Promise<void> {
    message = ''
    if (rows.length === 0) {
      message = 'Gere ao menos uma unidade para cadastrar.'
      return
    }
    saving = true
    try {
      const payload: PropertyBatch = {
        imoveis: rows.map((row) => ({
          id: 0,
          apelido: row.apelido,
          endereco: row.endereco,
          tipo: form.tipo,
          status: 'disponivel',
          valor_compra_c: 0,
          reforma_c: 0,
          itbi_c: 0,
          corretagem_c: 0,
          outros_custos_c: 0,
          data_aquisicao: form.data_aquisicao,
          observacao: '',
          ativo: true,
          bairro: row.bairro,
          empreendimento_id: form.empreendimento_id ? Number(form.empreendimento_id) : null,
          destinado_locacao: form.destinado_locacao,
          matricula_compartilhada: form.matricula_compartilhada,
          matricula_cartorio: row.matricula_cartorio,
          numero: row.numero,
          unidade: row.unidade,
        })),
        modo_custo: form.modo_custo,
        valor_compra_c: form.valor_compra_c,
        reforma_c: form.reforma_c,
        outros_custos_c: form.outros_custos_c,
      }
      await loteImoveis(payload)
      success(`${rows.length} imóveis cadastrados em lote.`)
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
  title="Cadastrar imóveis em lote"
  subtitle="Gere várias unidades com o mesmo padrão de nomenclatura e rateio de custos."
  wide
  onClose={onClose}
>
  {#if message}<div class="form-error">{message}</div>{/if}
  <div class="split-two" style="grid-template-columns:minmax(0,1fr) minmax(330px,0.9fr)">
    <div>
      <div class="form-section" style="margin-top:0; padding-top:0; border-top:0">
        <div class="form-section-title">Dados comuns</div>
        <div class="form-grid">
          <div class="field full">
            <label for="pb-proj">Empreendimento</label>
            <select id="pb-proj" class="select" bind:value={form.empreendimento_id}>
              <option value={0}>Sem empreendimento</option>
              {#each projects as project (project.id)}<option value={project.id}>{project.nome}</option>{/each}
            </select>
          </div>
          <div class="field">
            <label for="pb-tipo">Tipo de unidade</label>
            <select id="pb-tipo" class="select" bind:value={form.tipo}>
              <option value="casa">Casa</option>
              <option value="apartamento">Apartamento</option>
              <option value="sala">Sala</option>
              <option value="galpao">Galpão</option>
              <option value="lote">Lote</option>
              <option value="outros">Outros</option>
            </select>
          </div>
          <div class="field">
            <label for="pb-aq">Data de aquisição</label>
            <input id="pb-aq" class="input" type="date" bind:value={form.data_aquisicao} />
          </div>
          <div class="field full">
            <label for="pb-end">Endereço base</label>
            <input id="pb-end" class="input" bind:value={form.endereco_base} placeholder="Rua Europa, {'{n}'} — use {'{n}'} para o número sequencial" />
          </div>
          <div class="field">
            <label for="pb-bairro">Bairro</label>
            <input id="pb-bairro" class="input" bind:value={form.bairro} />
          </div>
          <div class="field">
            <label for="pb-num">Número do imóvel</label>
            <input id="pb-num" class="input" bind:value={form.numero} placeholder="Opcional" />
          </div>
          <label class="checkbox-line"><input type="checkbox" bind:checked={form.destinado_locacao} /> Destinado à locação</label>
          <label class="checkbox-line"><input type="checkbox" bind:checked={form.matricula_compartilhada} /> Matrícula compartilhada</label>
        </div>
      </div>

      <div class="form-section">
        <div class="form-section-title">Padrão de nomenclatura</div>
        <div class="form-grid">
          <div class="field"><label for="pb-pref">Prefixo do apelido</label><input id="pb-pref" class="input" bind:value={form.prefixo} /></div>
          <div class="field">
            <label for="pb-seq">Sequência</label>
            <select id="pb-seq" class="select" bind:value={form.sequencia}>
              <option value="pad2">01, 02, 03…</option>
              <option value="num">1, 2, 3…</option>
              <option value="alpha">A, B, C…</option>
            </select>
          </div>
          <div class="field"><label for="pb-ini">Início da numeração</label><input id="pb-ini" class="input" type="number" min="1" bind:value={form.inicio} /></div>
          <div class="field"><label for="pb-qtd">Quantidade de unidades</label><input id="pb-qtd" class="input" type="number" min="1" max="100" bind:value={form.quantidade} /></div>
          <div class="field"><label for="pb-suf">Sufixo do apelido</label><input id="pb-suf" class="input" bind:value={form.sufixo} placeholder="Opcional" /></div>
          <div class="field"><label for="pb-mat">Matrícula do grupo</label><input id="pb-mat" class="input" bind:value={form.matricula} /></div>
        </div>
      </div>

      <div class="form-section">
        <div class="form-section-title">Custos e rateio</div>
        <div class="form-grid">
          <div class="field full">
            <label for="pb-modo">Modo de cálculo</label>
            <select id="pb-modo" class="select" bind:value={form.modo_custo}>
              <option value="dividir">Dividir valores totais igualmente entre as unidades</option>
              <option value="fixo">Aplicar valores fixos em cada unidade individualmente</option>
            </select>
          </div>
          <div class="field"><span class="label">Terreno / compra</span><MoneyInput bind:value={form.valor_compra_c} /></div>
          <div class="field"><span class="label">Construção / reforma</span><MoneyInput bind:value={form.reforma_c} /></div>
          <div class="field"><span class="label">Outros / ITBI / cartório</span><MoneyInput bind:value={form.outros_custos_c} /></div>
        </div>
      </div>
    </div>

    <div>
      <div class="panel" style="position:sticky; top:0">
        <div class="panel-header">
          <h3>Prévia das unidades</h3>
          <span class="badge gray no-dot">{rows.length}</span>
        </div>
        <p class="helper mb-8">
          Investimento por unidade: <strong>{money(costPerUnit)}</strong>
          {#if form.modo_custo === 'dividir'} (rateio do total){/if}
        </p>
        {#if rows.length === 0}
          <div class="empty-state">Nenhuma unidade na prévia.</div>
        {:else}
          <div class="table-wrap" style="max-height:420px">
            <table class="data">
              <thead>
                <tr><th>Apelido</th><th>Número</th><th>Unidade</th><th>Matrícula</th><th></th></tr>
              </thead>
              <tbody>
                {#each rows as row, index (row.apelido + index)}
                  <tr>
                    <td class="cell-strong">{row.apelido}</td>
                    <td>{row.numero || '—'}</td>
                    <td>{row.unidade}</td>
                    <td>{row.matricula_cartorio || '—'}</td>
                    <td><button class="btn icon ghost" title="Remover" on:click={() => removeRow(index)}>×</button></td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
    </div>
  </div>

  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={save} disabled={saving || rows.length === 0}>
      {saving ? 'Cadastrando…' : `Cadastrar ${rows.length} unidade(s)`}
    </button>
  </div>
</Modal>
