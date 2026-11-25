<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { criarImovel, atualizarImovel } from '../lib/api'
  import { todayISO } from '../lib/format'
  import { errorMessage, success } from '../lib/toast'
  import type { Empreendimento, Imovel, ImovelInput } from '../lib/types'

  export let property: Imovel | null = null
  export let projects: Empreendimento[] = []
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  const p = property
  let form = {
    apelido: p?.apelido ?? '',
    endereco: p?.endereco ?? '',
    numero: p?.numero ?? '',
    unidade: p?.unidade ?? '',
    bairro: p?.bairro ?? '',
    empreendimento_id: p?.empreendimento_id ?? 0,
    situacao_patrimonial: p?.situacao_patrimonial ?? 'proprio',
    destinado_locacao: p?.destinado_locacao ?? true,
    matricula_compartilhada: p?.matricula_compartilhada ?? false,
    tipo: p?.tipo ?? 'casa',
    status: p?.status === 'manutencao' ? 'manutencao' : 'disponivel',
    data_venda: p?.data_venda ? String(p.data_venda).slice(0, 10) : todayISO(),
    valor_venda_c: p?.valor_venda_c ?? 0,
    valor_compra_c: p?.valor_compra_c ?? 0,
    reforma_c: p?.reforma_c ?? 0,
    itbi_c: p?.itbi_c ?? 0,
    corretagem_c: p?.corretagem_c ?? 0,
    outros_custos_c: p?.outros_custos_c ?? 0,
    matricula_cartorio: p?.matricula_cartorio ?? '',
    inscricao_iptu: p?.inscricao_iptu ?? '',
    unidade_agua: p?.unidade_agua ?? '',
    unidade_energia: p?.unidade_energia ?? '',
    data_aquisicao: p?.data_aquisicao ? String(p.data_aquisicao).slice(0, 10) : todayISO(),
    observacao: p?.observacao ?? '',
  }
  let saving = false
  let message = ''

  $: vendido = form.situacao_patrimonial === 'vendido'

  function selectProject(): void {
    if (!form.empreendimento_id) return
    const project = projects.find((item) => item.id === Number(form.empreendimento_id))
    if (project && project.endereco && !form.endereco) form.endereco = project.endereco
  }

  async function save(): Promise<void> {
    message = ''
    if (!form.apelido.trim()) {
      message = 'Informe o apelido do imóvel.'
      return
    }
    saving = true
    try {
      const payload: ImovelInput = {
        id: property?.id ?? 0,
        ...form,
        empreendimento_id: form.empreendimento_id ? Number(form.empreendimento_id) : null,
        data_aquisicao: form.data_aquisicao || null,
        data_venda: vendido ? form.data_venda : null,
        valor_venda_c: vendido ? form.valor_venda_c : 0,
        ativo: property?.ativo ?? true,
        investimento_importado_c: property?.investimento_importado_c ?? 0,
      }
      if (property && property.id) await atualizarImovel(payload)
      else await criarImovel(payload)
      success(property && property.id ? 'Imóvel atualizado.' : 'Imóvel cadastrado.')
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
  title={property && property.id ? 'Editar imóvel' : 'Novo imóvel'}
  subtitle="Custos de aquisição, identificadores e destinação compõem o cálculo de ROI."
  onClose={onClose}
>
  {#if message}<div class="form-error">{message}</div>{/if}

  <div class="form-grid">
    <div class="field full">
      <label for="im-apelido">Apelido *</label>
      <input id="im-apelido" class="input" bind:value={form.apelido} placeholder="Ex.: Casa 01 — Jardim Europa" />
    </div>
    <div class="field">
      <label for="im-tipo">Tipo</label>
      <select id="im-tipo" class="select" bind:value={form.tipo}>
        <option value="casa">Casa</option>
        <option value="apartamento">Apartamento</option>
        <option value="sala">Sala</option>
        <option value="galpao">Galpão</option>
        <option value="lote">Lote</option>
        <option value="outros">Outros</option>
      </select>
    </div>
    <div class="field">
      <label for="im-status">Status</label>
      <select id="im-status" class="select" bind:value={form.status}>
        <option value="disponivel">Disponível</option>
        <option value="manutencao">Em manutenção</option>
      </select>
    </div>
    <div class="field">
      <label for="im-situacao">Situação patrimonial</label>
      <select id="im-situacao" class="select" bind:value={form.situacao_patrimonial}>
        <option value="proprio">Próprio</option>
        <option value="vendido">Vendido</option>
      </select>
    </div>
    <div class="field">
      <label for="im-project">Empreendimento</label>
      <select id="im-project" class="select" bind:value={form.empreendimento_id} on:change={selectProject}>
        <option value={0}>Sem empreendimento</option>
        {#each projects as project (project.id)}
          <option value={project.id}>{project.nome}</option>
        {/each}
      </select>
    </div>
    <div class="field full">
      <label for="im-endereco">Endereço</label>
      <input id="im-endereco" class="input" bind:value={form.endereco} />
    </div>
    <div class="field">
      <label for="im-numero">Número do imóvel</label>
      <input id="im-numero" class="input" bind:value={form.numero} placeholder="Ex.: 620, 630 ou 620-A" />
    </div>
    <div class="field">
      <label for="im-unidade">Unidade / complemento</label>
      <input id="im-unidade" class="input" bind:value={form.unidade} placeholder="Ex.: 1, 2 ou A" />
    </div>
    <div class="field">
      <label for="im-bairro">Bairro</label>
      <input id="im-bairro" class="input" bind:value={form.bairro} />
    </div>
    <div class="field">
      <label for="im-aquisicao">Data de aquisição</label>
      <input id="im-aquisicao" class="input" type="date" bind:value={form.data_aquisicao} />
    </div>
    <div class="field full">
      <label class="checkbox-line">
        <input type="checkbox" bind:checked={form.destinado_locacao} disabled={vendido} />
        Destinado à locação (unidade para alugar)
      </label>
    </div>
    <div class="field full">
      <label class="checkbox-line">
        <input type="checkbox" bind:checked={form.matricula_compartilhada} />
        Matrícula compartilhada (desmembramento)
      </label>
    </div>

    {#if vendido}
      <div class="field">
        <label for="im-data-venda">Data da venda</label>
        <input id="im-data-venda" class="input" type="date" bind:value={form.data_venda} />
      </div>
      <div class="field">
        <span class="label">Valor da venda</span>
        <MoneyInput bind:value={form.valor_venda_c} />
      </div>
    {/if}
  </div>

  <div class="form-section">
    <div class="form-section-title">Custos do investimento</div>
    <div class="form-grid">
      <div class="field"><span class="label">Valor de compra</span><MoneyInput bind:value={form.valor_compra_c} /></div>
      <div class="field"><span class="label">Reforma</span><MoneyInput bind:value={form.reforma_c} /></div>
      <div class="field"><span class="label">ITBI</span><MoneyInput bind:value={form.itbi_c} /></div>
      <div class="field"><span class="label">Corretagem</span><MoneyInput bind:value={form.corretagem_c} /></div>
      <div class="field"><span class="label">Outros custos</span><MoneyInput bind:value={form.outros_custos_c} /></div>
      {#if property?.investimento_importado_c}
        <div class="field">
          <span class="label">Investimento histórico importado</span>
          <div class="input" style="background:#f8fafc">R$ {(property.investimento_importado_c / 100).toFixed(2).replace('.', ',')}</div>
        </div>
      {/if}
    </div>
  </div>

  <div class="form-section">
    <div class="form-section-title">Identificadores</div>
    <div class="form-grid">
      <div class="field"><label for="im-mat">Matrícula do imóvel</label><input id="im-mat" class="input" bind:value={form.matricula_cartorio} /></div>
      <div class="field"><label for="im-iptu">Inscrição municipal / IPTU</label><input id="im-iptu" class="input" bind:value={form.inscricao_iptu} /></div>
      <div class="field"><label for="im-agua">Unidade consumidora de água</label><input id="im-agua" class="input" bind:value={form.unidade_agua} /></div>
      <div class="field"><label for="im-energia">Unidade consumidora de energia</label><input id="im-energia" class="input" bind:value={form.unidade_energia} /></div>
      <div class="field full"><label for="im-obs">Observações</label><textarea id="im-obs" class="textarea" rows="2" bind:value={form.observacao}></textarea></div>
    </div>
  </div>

  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={save} disabled={saving}>{saving ? 'Salvando…' : 'Salvar imóvel'}</button>
  </div>
</Modal>
