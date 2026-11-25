<script lang="ts">
  import { dateBR, money, monthLabel } from '../lib/format'
  import type { MonthlyCashflow } from '../lib/types'

  export let items: MonthlyCashflow[] = []
  export let stacked = false
  export let title = 'Receitas × Despesas'
  export let subtitle = 'Regime de caixa por período'

  const W = 680
  const H = 292
  const PAD = { top: 16, right: 20, bottom: 66, left: 68 }
  const plotW = W - PAD.left - PAD.right
  const plotH = H - PAD.top - PAD.bottom
  const axisY = PAD.top + plotH

  let cardEl: HTMLDivElement | null = null
  let hovered: { item: MonthlyCashflow; x: number; y: number } | null = null

  $: data = items || []
  $: values = data.map((item) =>
    stacked
      ? Math.max(0, (item.receitas_c || 0) + (item.despesas_operacionais_c || 0))
      : Math.max(item.receitas_c || 0, item.despesas_operacionais_c || 0),
  )
  $: rawMax = Math.max(1, ...values)
  $: step = niceStep(rawMax / 4)
  $: axisMax = Math.max(step * 4, rawMax)
  $: ticks = [0, 1, 2, 3, 4].map((index) => (axisMax / 4) * index)
  $: slot = plotW / Math.max(1, data.length)
  // With 45-degree rotated labels, many more ticks fit legibly without colliding
  $: labelStride = data.length <= 24 ? 1 : Math.ceil(data.length / 22)

  function niceStep(value: number): number {
    const power = Math.pow(10, Math.floor(Math.log10(Math.max(1, value))))
    const normalized = value / power
    const factor = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10
    return factor * power
  }

  function barHeight(cents: number): number {
    return ((cents || 0) / axisMax) * plotH
  }

  function compact(cents: number): string {
    const value = Math.abs(cents) / 100
    if (value >= 1000000) return `R$ ${(value / 1000000).toFixed(1).replace('.', ',')}M`
    if (value >= 1000) return `R$ ${(value / 1000).toFixed(0)}k`
    return `R$ ${value.toFixed(0)}`
  }

  function label(item: MonthlyCashflow): string {
    if (!item?.mes) return ''
    if (item.mes.length === 10) return dateBR(item.mes).slice(0, 5)
    return item.mes.length === 7 ? monthLabel(item.mes) : item.mes
  }

  function fullLabel(item: MonthlyCashflow): string {
    if (!item?.mes) return ''
    if (item.mes.length === 10) return dateBR(item.mes)
    return item.mes.length === 7 ? monthLabel(item.mes) : item.mes
  }

  function onHover(event: MouseEvent, item: MonthlyCashflow): void {
    if (!cardEl) return
    const rect = cardEl.getBoundingClientRect()
    hovered = {
      item,
      x: Math.max(95, Math.min(rect.width - 95, event.clientX - rect.left)),
      y: Math.max(60, event.clientY - rect.top),
    }
  }
</script>

<div class="chart-card" bind:this={cardEl} role="group" on:mouseleave={() => (hovered = null)}>
  <h3>{title}</h3>
  <div class="chart-sub">{subtitle}</div>
  <div class="chart-legend">
    <span class="legend-item"><span class="legend-swatch" style="background:#2563EB"></span>Receitas</span>
    <span class="legend-item"><span class="legend-swatch" style="background:#EA580C"></span>Despesas operacionais</span>
  </div>
  {#if hovered}
    {@const liq = (hovered.item.receitas_c || 0) - (hovered.item.despesas_operacionais_c || 0)}
    <div class="chart-tooltip" style="left:{hovered.x}px; top:{hovered.y}px">
      <div style="font-weight:700; margin-bottom:2px">{fullLabel(hovered.item)}</div>
      <div>Receitas: <strong>{money(hovered.item.receitas_c)}</strong></div>
      <div>Despesas: <strong>{money(hovered.item.despesas_operacionais_c)}</strong></div>
      <div style="border-top:1px solid rgba(255,255,255,0.18); margin-top:3px; padding-top:2px">
        Resultado: <strong style="color:{liq >= 0 ? '#6EE7B7' : '#FCA5A5'}">{money(liq)}</strong>
      </div>
    </div>
  {/if}
  {#if data.length === 0}
    <div class="empty-state" style="margin:8px 0 14px">Sem dados no período selecionado.</div>
  {:else}
    <svg viewBox="0 0 {W} {H}" style="width:100%; height:auto; display:block" role="img" aria-label={title}>
      {#each ticks as tick}
        <line
          x1={PAD.left}
          x2={W - PAD.right}
          y1={axisY - (tick / axisMax) * plotH}
          y2={axisY - (tick / axisMax) * plotH}
          stroke={tick === 0 ? '#CBD5E1' : '#EEF2F7'}
          stroke-width="1"
        />
        <text x={PAD.left - 8} y={axisY - (tick / axisMax) * plotH + 3.5} text-anchor="end" font-size="9.5" fill="#667085">
          {compact(tick)}
        </text>
      {/each}

      {#each data as item, index (item.mes + index)}
        {@const x = PAD.left + slot * index}
        {@const cx = x + slot / 2}
        {@const labelY = axisY + 15}
        {@const recH = barHeight(item.receitas_c)}
        {@const despH = barHeight(item.despesas_operacionais_c)}
        <g role="presentation" on:mousemove={(e) => onHover(e, item)}>
          <rect
            x={x}
            y={PAD.top}
            width={slot}
            height={plotH}
            fill={hovered?.item === item ? 'rgba(69,105,144,0.06)' : 'transparent'}
          />
          {#if stacked}
            <rect
              x={x + slot * 0.2}
              y={axisY - despH}
              width={slot * 0.6}
              height={despH}
              fill="#EA580C"
              rx="2"
            />
            <rect
              x={x + slot * 0.2}
              y={axisY - recH - despH}
              width={slot * 0.6}
              height={recH}
              fill="#2563EB"
              rx="2"
            />
          {:else}
            <rect
              x={x + slot * 0.18}
              y={axisY - recH}
              width={slot * 0.3}
              height={recH}
              fill="#2563EB"
              rx="2"
            />
            <rect
              x={x + slot * 0.52}
              y={axisY - despH}
              width={slot * 0.3}
              height={despH}
              fill="#EA580C"
              rx="2"
            />
          {/if}
        </g>
        {#if index % labelStride === 0 || index === data.length - 1}
          <line x1={cx} y1={axisY} x2={cx} y2={axisY + 5} stroke="#CBD5E1" stroke-width="1" />
          <text
            x={cx}
            y={labelY}
            transform="rotate(-45 {cx} {labelY})"
            text-anchor="end"
            font-size="9.5"
            fill="#475467"
          >
            {label(item)}
          </text>
        {/if}
      {/each}
    </svg>
  {/if}
</div>
