<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import { criarEmpreendimento, atualizarEmpreendimento } from '../lib/api'
  import { errorMessage, success } from '../lib/toast'
  import type { Empreendimento, Imovel } from '../lib/types'

  export let project: Empreendimento | null = null
  export let properties: Imovel[] = []
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  let form = {
    nome: project?.nome ?? '',
    endereco: project?.endereco ?? '',
    observacao: project?.observacao ?? '',
  }
  let unitIds: number[] = [...(project?.unidade_ids || [])]
  let saving = false
  let message = ''

  function toggleUnit(id: number): void {
    if (unitIds.includes(id)) unitIds = unitIds.filter((item) => item !== id)
    else unitIds = [...unitIds, id]
  }

  async function save(): Promise<void> {
    message = ''
    if (!form.nome.trim()) {
      message = 'Informe o nome do empreendimento.'
      return
    }
    saving = true
    try {
      const payload: Empreendimento = {
        id: project?.id ?? 0,
        ...form,
        ativo: project?.ativo ?? true,
        unidade_ids: unitIds,
      }
      if (project) await atualizarEmpreendimento(payload)
      else await criarEmpreendimento(payload)
      success(project ? 'Empreendimento atualizado.' : 'Empreendimento cadastrado.')
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
  title={project ? 'Editar empreendimento' : 'Novo empreendimento / obra'}
  subtitle="As unidades selecionadas formam o escopo de rateio de despesas e investimentos."
  onClose={onClose}
>
  {#if message}<div class="form-error">{message}</div>{/if}
  <div class="form-grid">
    <div class="field full">
      <label for="pr-nome">Nome *</label>
      <input id="pr-nome" class="input" bind:value={form.nome} placeholder="Ex.: Residencial Jardim Europa" />
    </div>
    <div class="field full">
      <label for="pr-endereco">Endereço</label>
      <input id="pr-endereco" class="input" bind:value={form.endereco} />
    </div>
    <div class="field full">
      <label for="pr-obs">Observações</label>
      <textarea id="pr-obs" class="textarea" rows="2" bind:value={form.observacao}></textarea>
    </div>
  </div>

  <div class="form-section">
    <div class="form-section-title">Unidades para rateio</div>
    <p class="helper mb-8">Selecione os imóveis que pertencem a este empreendimento. O rateio de despesas usa esta lista.</p>
    {#if properties.length === 0}
      <div class="empty-state">Cadastre imóveis antes de vincular unidades.</div>
    {:else}
      <div class="unit-picker">
        {#each properties as item (item.id)}
          <label>
            <input type="checkbox" checked={unitIds.includes(item.id)} on:change={() => toggleUnit(item.id)} />
            <span>{item.apelido}</span>
            <span class="cell-muted" style="margin-left:auto; font-size:11px">{item.endereco || ''}</span>
          </label>
        {/each}
      </div>
    {/if}
  </div>

  <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
    <button class="btn secondary" on:click={onClose} disabled={saving}>Cancelar</button>
    <button class="btn" on:click={save} disabled={saving}>{saving ? 'Salvando…' : 'Salvar empreendimento'}</button>
  </div>
</Modal>
