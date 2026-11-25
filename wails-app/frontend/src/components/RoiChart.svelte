<script lang="ts">
  import { money, percent } from '../lib/format'
  import type { Roi } from '../lib/types'

  export let items: Roi[] = []
  export let periodLabel = 'Últimos 12 meses'
  export let allTime = false
  export let selectedPropertyId = 0
  export let onSelectProperty: ((id: number) => void) | null = null

  let cardEl: HTMLDivElement | null = null
  let hovered: { item: Roi; x: number; y: number } | null = null

  $: metricOf = (item: Roi): number =>
    allTime ? item.roi_acumulado_pct || 0 : item.roi_anual_pct || 0
  $: data = [...(items || [])].sort((a, b) => metricOf(b) - metricOf(a)).slice(0, 8)
  $: maxAbs = Math.max(1, ...data.map((item) => Math.abs(metricOf(item))))
  $: rowH = data.length ? Math.min(30, 185 / data.length) : 24
  $: height = Math.max(120, data.length * rowH + 40)

  function onHover(event: MouseEvent, item: Roi): void {
    if (!cardEl) return
    const rect = cardEl.getBoundingClientRect()
    hovered = {
      item,
      x: Math.max(115, Math.min(rect.width - 115, event.clientX - rect.left)),
      y: Math.max(65, event.clientY - rect.top),
    }
  }

  function handleClick(item: Roi): void {
    if (!onSelectProperty) return
    onSelectProperty(selectedPropertyId === item.imovel_id ? 0 : item.imovel_id)
  }
</script>

<div class="chart-card" bind:this={cardEl} role="group" on:mouseleave={() => (hovered = null)}>
  <div class="flex" style="justify-content:space-between">
    <div>
      <h3>Rentabilidade por imóvel (ROI · {periodLabel})</h3>
      <div class="chart-sub">
        Retorno líquido no período sobre o capital investido{onSelectProperty ? ' · Clique para filtrar' : ''}
      </div>
    </div>
    {#if selectedPropertyId && onSelectProperty}
      <button class="btn ghost small" type="button" on:click={() => onSelectProperty && onSelectProperty(0)}>
        Limpar filtro
      </button>
    {/if}
  </div>

  {#if hovered}
    {@const rec = allTime ? hovered.item.recebido_total_c : hovered.item.recebido_12m_c}
    {@const desp = allTime ? hovered.item.despesas_total_c : hovered.item.despesas_12m_c}
    <div class="chart-tooltip" style="left:{hovered.x}px; top:{hovered.y}px">
      <div style="font-weight:700; margin-bottom:2px">{hovered.item.apelido}</div>
      <div>ROI ({periodLabel}): <strong>{percent(metricOf(hovered.item))}</strong></div>
      {#if !allTime}
        <div>ROI acumulado total: <strong>{percent(hovered.item.roi_acumulado_pct)}</strong></div>
      {/if}
      <div>Capital investido: <strong>{money(hovered.item.investimento_c)}</strong></div>
      <div>Líquido no período: <strong>{money(rec - desp)}</strong></div>
    </div>
  {/if}

  {#if data.length === 0}
    <div class="empty-state" style="margin:8px 0 14px">Sem imóveis para calcular rentabilidade.</div>
  {:else}
    <svg viewBox="0 0 680 {height}" style="width:100%; height:auto; display:block" role="img" aria-label="ROI por imóvel">
      {#each data as item, index}
        {@const y = 18 + index * rowH}
        {@const value = metricOf(item)}
        {@const width = (Math.abs(value) / maxAbs) * 355}
        {@const isSelected = selectedPropertyId === item.imovel_id}
        <g
          style={onSelectProperty ? 'cursor:pointer' : ''}
          role="button"
          tabindex="0"
          on:mousemove={(e) => onHover(e, item)}
          on:click={() => handleClick(item)}
          on:keydown={(e) => (e.key === 'Enter' || e.key === ' ') && handleClick(item)}
        >
          <rect
            x="0"
            y={y}
            width="676"
            height={rowH}
            rx="4"
            fill={isSelected ? 'rgba(69,105,144,0.12)' : hovered?.item === item ? 'rgba(69,105,144,0.05)' : 'transparent'}
          />
          <text x="6" y={y + rowH / 2 + 3.5} font-size="10.5" font-weight={isSelected ? '700' : '500'} fill="#344054">
            {(item.apelido || '').slice(0, 26)}
          </text>
          <rect x="215" y={y + 4} width="375" height={Math.max(9, rowH - 11)} rx="4" fill="#F1F5F9" />
          <rect
            x="215"
            y={y + 4}
            width={width}
            height={Math.max(9, rowH - 11)}
            rx="4"
            fill={value < 0 ? '#DC2626' : value >= 6 ? '#059669' : '#456990'}
          />
          <text x="600" y={y + rowH / 2 + 3.5} font-size="10.5" font-weight="650" fill={value < 0 ? '#B42318' : '#1F2937'}>
            {percent(value)}
          </text>
        </g>
      {/each}
    </svg>
  {/if}
</div>
