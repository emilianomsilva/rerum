<script lang="ts" generics="T">
  import { onDestroy } from 'svelte'
  import { saveTextFile } from '../lib/api'
  import { csvContent } from '../lib/format'
  import { getSetting, removeSetting, setSetting } from '../lib/settings'
  import { error as notifyError, errorMessage, success } from '../lib/toast'
  import type { TableAction, TableColumn } from '../lib/types'
  import Icon from './Icon.svelte'

  export let columns: TableColumn<T>[] = []
  export let rows: T[] = []
  export let keyFor: (row: T) => string | number = (row: T) =>
    (row as unknown as { id: string | number }).id
  export let empty = 'Nenhum registro encontrado.'
  export let loading = false
  export let selectable = false
  export let selectedIds: Array<string | number> = []
  export let canSelect: (row: T) => boolean = () => true
  export let onRowClick: ((row: T) => void) | null = null
  export let onRowDoubleClick: ((row: T) => void) | null = null
  export let rowClass: (row: T) => string = () => ''
  export let actions: boolean | TableAction<T>[] = false
  export let footer: Record<string, string | number> | null = null
  export let compact = false
  export let emptyText: string | null = null
  export let settingsKey = ''
  export let maxHeight = ''
  export let exportFilename = ''
  export let showToolbar = true

  let sortKey = settingsKey ? getSetting<string>(`table/${settingsKey}/sort_key`, '') : ''
  let sortDir: 1 | -1 = settingsKey
    ? (getSetting<number>(`table/${settingsKey}/sort_dir`, 1) === -1 ? -1 : 1)
    : 1
  let manualWidths: Record<string, number> = settingsKey
    ? getSetting<Record<string, number>>(`table/${settingsKey}/widths`, {})
    : {}
  let dense = settingsKey
    ? getSetting<boolean>(`table/${settingsKey}/dense`, compact)
    : compact

  let contextMenu: { x: number; y: number } | null = null
  let resizing: { key: string; startX: number; startW: number } | null = null

  function raw(row: T, column: TableColumn<T>): string | number | boolean | null | undefined {
    if (column.value) return column.value(row)
    const record = row as unknown as Record<string, unknown>
    const val = record[column.key]
    if (
      typeof val === 'string' ||
      typeof val === 'number' ||
      typeof val === 'boolean' ||
      val === null ||
      val === undefined
    ) {
      return val
    }
    return undefined
  }

  function cellText(row: T, column: TableColumn<T>): string | number | boolean | null | undefined {
    if (column.text) return column.text(row)
    return raw(row, column)
  }

  function parseSmartSortValue(input: string | number | boolean | null | undefined): string | number | null {
    if (input === null || input === undefined) return null
    if (typeof input === 'number') return Number.isNaN(input) ? null : input
    if (typeof input === 'boolean') return input ? 1 : 0

    const trimmed = String(input).trim()
    if (!trimmed || trimmed === '—') return null

    // ISO date YYYY-MM-DD or competence YYYY-MM
    if (/^\d{4}-\d{2}(?:-\d{2})?$/.test(trimmed)) return trimmed

    // Brazilian date DD/MM/YYYY (including period start DD/MM/YYYY – DD/MM/YYYY)
    const brDate = trimmed.match(/^(\d{2})\/(\d{2})\/(\d{4})(?:\s+[–-]\s+\d{2}\/\d{2}\/\d{4})?$/)
    if (brDate) {
      return Number(`${brDate[3]}${brDate[2]}${brDate[1]}`)
    }

    // Month/Year MM/YYYY
    const monthYear = trimmed.match(/^(\d{2})\/(\d{4})$/)
    if (monthYear) {
      return Number(`${monthYear[2]}${monthYear[1]}`)
    }

    // Currency R$ 1.234,56 or percentage 12,34% or Dia 10
    const diaMatch = trimmed.match(/^Dia\s+(\d+)$/i)
    if (diaMatch) return Number(diaMatch[1])

    const fracMatch = trimmed.match(/^(\d+)\/(\d+)$/)
    if (fracMatch) return Number(fracMatch[1])

    const numericCandidate = trimmed.replace(/^R\$\s*/, '').replace(/%$/, '').trim()
    if (/^[+-]?\d+(?:\.\d{3})*(?:,\d+)?$/.test(numericCandidate)) {
      const parsed = Number(numericCandidate.replace(/\./g, '').replace(',', '.'))
      if (Number.isFinite(parsed)) return parsed
    }

    return trimmed
  }

  function sortValue(row: T, column: TableColumn<T>): string | number | null {
    if (column.sortValue) return column.sortValue(row)
    const rawVal = raw(row, column)
    if (rawVal !== undefined && rawVal !== null) {
      const parsed = parseSmartSortValue(rawVal)
      if (parsed !== null) return parsed
    }
    if (column.badge) {
      const b = column.badge(row)
      if (b) return parseSmartSortValue(b.text ?? b.label ?? '')
    }
    return parseSmartSortValue(cellText(row, column))
  }

  function compare(a: T, b: T, column: TableColumn<T>, dir: 1 | -1): number {
    const left = sortValue(a, column)
    const right = sortValue(b, column)
    // Empty values always stay at the bottom regardless of sort direction (PySide parity)
    if (left === null && right === null) return 0
    if (left === null) return 1
    if (right === null) return -1
    if (typeof left === 'number' && typeof right === 'number') return (left - right) * dir
    return (
      String(left).localeCompare(String(right), 'pt-BR', {
        numeric: true,
        sensitivity: 'base',
      }) * dir
    )
  }

  $: emptyLabel = emptyText ?? empty
  $: actionList = Array.isArray(actions) ? actions : []
  $: hasActions = Array.isArray(actions) ? actions.length > 0 : Boolean(actions)
  $: sortColumn = columns.find((column) => column.key === sortKey)
  $: displayRows = sortColumn ? [...rows].sort((a, b) => compare(a, b, sortColumn, sortDir)) : rows
  $: selectableRows = selectable ? displayRows.filter((row) => canSelect(row)) : []
  $: allSelected =
    selectableRows.length > 0 &&
    selectableRows.every((row) => selectedIds.includes(keyFor(row)))
  $: colspan = columns.length + (selectable ? 1 : 0) + (hasActions ? 1 : 0)
  $: effectiveCompact = compact || dense
  $: resolvedExportName = exportFilename || (settingsKey ? `${settingsKey}.csv` : '')

  function toggleSort(column: TableColumn<T>): void {
    if (column.sortable === false || resizing) return
    if (sortKey === column.key) {
      if (sortDir === 1) {
        sortDir = -1
      } else {
        // Third click restores natural order
        sortKey = ''
        sortDir = 1
      }
    } else {
      sortKey = column.key
      sortDir = 1
    }
    if (settingsKey) {
      setSetting(`table/${settingsKey}/sort_key`, sortKey)
      setSetting(`table/${settingsKey}/sort_dir`, sortDir)
    }
  }

  function toggleAll(): void {
    if (allSelected) {
      const ids = new Set(selectableRows.map(keyFor))
      selectedIds = selectedIds.filter((id) => !ids.has(id))
    } else {
      const merged = new Set(selectedIds)
      for (const row of selectableRows) merged.add(keyFor(row))
      selectedIds = [...merged]
    }
  }

  function toggleRow(row: T): void {
    const id = keyFor(row)
    if (selectedIds.includes(id)) selectedIds = selectedIds.filter((item) => item !== id)
    else selectedIds = [...selectedIds, id]
  }

  function widthStyle(column: TableColumn<T>, widths: Record<string, number>): string {
    const manual = widths[column.key]
    if (manual) return `width:${manual}px; min-width:${manual}px; max-width:${manual}px`
    if (!column.width) return ''
    const w = typeof column.width === 'number' ? `${column.width}px` : column.width
    return `width:${w}`
  }

  function startResize(event: MouseEvent, column: TableColumn<T>): void {
    event.preventDefault()
    event.stopPropagation()
    const th = (event.currentTarget as HTMLElement).parentElement
    const startW = th ? th.getBoundingClientRect().width : 120
    resizing = { key: column.key, startX: event.clientX, startW }
    window.addEventListener('mousemove', onResizeMove)
    window.addEventListener('mouseup', stopResize)
  }

  function onResizeMove(event: MouseEvent): void {
    if (!resizing) return
    const nextW = Math.max(56, Math.min(720, Math.round(resizing.startW + (event.clientX - resizing.startX))))
    manualWidths = { ...manualWidths, [resizing.key]: nextW }
  }

  function stopResize(): void {
    if (resizing && settingsKey) {
      setSetting(`table/${settingsKey}/widths`, manualWidths)
    }
    window.removeEventListener('mousemove', onResizeMove)
    window.removeEventListener('mouseup', stopResize)
    setTimeout(() => {
      resizing = null
    }, 0)
  }

  function resetSingleWidth(event: MouseEvent, column: TableColumn<T>): void {
    event.preventDefault()
    event.stopPropagation()
    const copy = { ...manualWidths }
    delete copy[column.key]
    manualWidths = copy
    if (settingsKey) setSetting(`table/${settingsKey}/widths`, manualWidths)
  }

  function resetAllWidths(): void {
    manualWidths = {}
    if (settingsKey) removeSetting(`table/${settingsKey}/widths`)
    contextMenu = null
  }

  function resetSort(): void {
    sortKey = ''
    sortDir = 1
    if (settingsKey) {
      removeSetting(`table/${settingsKey}/sort_key`)
      removeSetting(`table/${settingsKey}/sort_dir`)
    }
    contextMenu = null
  }

  function toggleDensity(): void {
    dense = !dense
    if (settingsKey) setSetting(`table/${settingsKey}/dense`, dense)
    contextMenu = null
  }

  function openHeaderMenu(event: MouseEvent): void {
    event.preventDefault()
    contextMenu = {
      x: Math.min(event.clientX, window.innerWidth - 230),
      y: Math.min(event.clientY, window.innerHeight - 150),
    }
  }

  function closeContextMenu(): void {
    if (contextMenu) contextMenu = null
  }

  async function exportTableCsv(): Promise<void> {
    contextMenu = null
    const headers = columns.map((c) => c.label)
    const bodyRows = displayRows.map((row) =>
      columns.map((col) => {
        if (col.badge) {
          const b = col.badge(row)
          if (b) return b.text ?? b.label ?? ''
        }
        const val = cellText(row, col)
        return val === null || val === undefined ? '' : String(val)
      }),
    )
    if (footer) {
      bodyRows.push(columns.map((col) => String(footer[col.key] ?? '')))
    }
    try {
      const content = csvContent(headers, bodyRows)
      const saved = await saveTextFile(resolvedExportName || 'tabela-rerum.csv', content)
      if (saved) success(`CSV exportado em: ${saved}`)
    } catch (err) {
      notifyError(errorMessage(err))
    }
  }

  function cellClass(row: T, column: TableColumn<T>): string {
    const extra = column.class ? column.class(row) : ''
    return `${column.align ? `align-${column.align}` : ''} ${extra || ''}`.trim()
  }

  function cellTooltip(row: T, column: TableColumn<T>): string {
    if (column.badge || column.html) return ''
    const v = cellText(row, column)
    if (v === null || v === undefined || v === '—') return ''
    const s = String(v)
    return s.length > 24 ? s : ''
  }

  onDestroy(() => {
    window.removeEventListener('mousemove', onResizeMove)
    window.removeEventListener('mouseup', stopResize)
  })
</script>

<svelte:window on:click={closeContextMenu} />

<div class="table-shell">
  {#if showToolbar && !compact && (rows.length > 0 || Object.keys(manualWidths).length > 0 || sortKey)}
    <div class="table-meta-bar">
      <span>
        {displayRows.length} {displayRows.length === 1 ? 'registro' : 'registros'}
        {#if selectable && selectedIds.length > 0}
          · <strong>{selectedIds.length} selecionado(s)</strong>
        {/if}
        {#if sortColumn}
          · Ordenado por <strong>{sortColumn.label}</strong> ({sortDir > 0 ? 'crescente' : 'decrescente'})
        {/if}
      </span>
      <div class="flex" style="gap:4px">
        {#if Object.keys(manualWidths).length > 0}
          <button class="btn ghost small" type="button" on:click={resetAllWidths} title="Restaurar largura automática das colunas">
            Ajustar colunas
          </button>
        {/if}
        <button
          class="btn ghost small"
          type="button"
          on:click={toggleDensity}
          title="Alternar densidade das linhas"
        >
          {effectiveCompact ? 'Confortável' : 'Compacta'}
        </button>
        {#if displayRows.length > 0}
          <button
            class="btn ghost small"
            type="button"
            on:click={exportTableCsv}
            title="Exportar visão atual para CSV (Excel)"
          >
            <Icon name="download" size={13} /> CSV
          </button>
        {/if}
      </div>
    </div>
  {/if}

  <div
    class="table-wrap"
    class:table-compact={effectiveCompact}
    style={maxHeight ? `max-height:${maxHeight}` : ''}
  >
    <table class="data">
      <thead on:contextmenu={openHeaderMenu}>
        <tr>
          {#if selectable}
            <th class="align-center" style="width:38px; min-width:38px">
              <input
                type="checkbox"
                checked={allSelected}
                disabled={selectableRows.length === 0}
                on:change={toggleAll}
                aria-label="Selecionar todas as linhas"
              />
            </th>
          {/if}
          {#each columns as column}
            <th
              class:sortable={column.sortable !== false}
              class:align-right={column.align === 'right'}
              class:align-center={column.align === 'center'}
              style={widthStyle(column, manualWidths)}
              on:click={() => toggleSort(column)}
              title={column.title || 'Clique para ordenar · Arraste a borda direita para redimensionar · Duplo clique na borda restaura'}
            >
              <span class="th-inner">
                <span>{column.label}</span>
                {#if sortKey === column.key}
                  <span class="sort-arrow">{sortDir > 0 ? '▲' : '▼'}</span>
                {:else if column.sortable !== false}
                  <span class="sort-hint">⇅</span>
                {/if}
              </span>
              <button
                type="button"
                tabindex="-1"
                class="col-resizer"
                aria-label="Redimensionar coluna {column.label}"
                on:mousedown={(e) => startResize(e, column)}
                on:dblclick={(e) => resetSingleWidth(e, column)}
                on:click|stopPropagation
              ></button>
            </th>
          {/each}
          {#if hasActions}
            <th class="sticky-actions" style="width:1%; white-space:nowrap">Ações</th>
          {/if}
        </tr>
      </thead>
      <tbody>
        {#if loading && rows.length === 0}
          <tr>
            <td colspan={colspan} class="empty-row">Carregando…</td>
          </tr>
        {:else if displayRows.length === 0}
          <tr>
            <td colspan={colspan} class="empty-row">{emptyLabel}</td>
          </tr>
        {:else}
          {#each displayRows as row (keyFor(row))}
            {@const selected = selectedIds.includes(keyFor(row))}
            <tr
              class="{rowClass(row)} {selected ? 'selected' : ''}"
              class:clickable={Boolean(onRowClick || onRowDoubleClick)}
              on:click={() => onRowClick && onRowClick(row)}
              on:dblclick={() => onRowDoubleClick && onRowDoubleClick(row)}
            >
              {#if selectable}
                <td class="align-center">
                  <input
                    type="checkbox"
                    checked={selected}
                    disabled={!canSelect(row)}
                    on:click|stopPropagation
                    on:change={() => toggleRow(row)}
                    aria-label="Selecionar linha"
                  />
                </td>
              {/if}
              {#each columns as column}
                {@const badge = column.badge ? column.badge(row) : null}
                <td
                  class={cellClass(row, column)}
                  style={manualWidths[column.key] ? `max-width:${manualWidths[column.key]}px` : ''}
                  title={cellTooltip(row, column)}
                >
                  {#if badge}
                    <span class="badge {badge.tone || 'gray'}">{badge.text ?? badge.label}</span>
                  {:else if column.html}
                    {@html column.html(row)}
                  {:else}
                    {cellText(row, column) ?? '—'}
                  {/if}
                </td>
              {/each}
              {#if hasActions}
                <td class="actions-td sticky-actions">
                  <div class="actions-cell">
                    {#if actionList.length}
                      {#each actionList.filter((action) => !action.when || action.when(row)) as action}
                        <button
                          class="btn small {action.danger ? 'destructive' : 'secondary'}"
                          type="button"
                          on:click|stopPropagation={() => action.onClick(row)}
                        >{action.label}</button>
                      {/each}
                    {:else}
                      <slot name="actions" {row} />
                    {/if}
                  </div>
                </td>
              {/if}
            </tr>
          {/each}
        {/if}
      </tbody>
      {#if footer}
        <tfoot>
          <tr>
            {#if selectable}<td></td>{/if}
            {#each columns as column}
              <td class={column.align === 'right' ? 'align-right' : column.align === 'center' ? 'align-center' : ''}>
                {footer[column.key] ?? ''}
              </td>
            {/each}
            {#if hasActions}<td class="sticky-actions"></td>{/if}
          </tr>
        </tfoot>
      {/if}
    </table>
  </div>
</div>

{#if contextMenu}
  <div
    class="context-menu"
    style="left:{contextMenu.x}px; top:{contextMenu.y}px"
    role="menu"
    tabindex="-1"
    on:click|stopPropagation
    on:keydown|stopPropagation
  >
    <button type="button" on:click={resetAllWidths}>Ajustar todas as colunas automaticamente</button>
    {#if sortKey}
      <button type="button" on:click={resetSort}>Restaurar ordem original</button>
    {/if}
    <button type="button" on:click={toggleDensity}>
      Modo {effectiveCompact ? 'confortável' : 'compacto'}
    </button>
    {#if displayRows.length > 0}
      <button type="button" on:click={exportTableCsv}>Exportar tabela em CSV…</button>
    {/if}
  </div>
{/if}

<style>
  tr.clickable {
    cursor: pointer;
  }
  .table-compact :global(table.data tbody td) {
    padding: 5px 9px;
  }
  .table-compact :global(table.data thead th) {
    padding: 7px 9px;
  }
  .col-resizer {
    position: absolute;
    top: 0;
    right: 0;
    width: 7px;
    height: 100%;
    padding: 0;
    border: 0;
    background: transparent;
    cursor: col-resize;
    user-select: none;
  }
  .col-resizer:hover {
    background: rgba(69, 105, 144, 0.35);
  }
</style>
