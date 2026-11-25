<script lang="ts">
  import { centsToReais, reaisToCents } from '../lib/format'

  export let value = 0
  export let allowNegative = false
  export let disabled = false
  export let placeholder = '0,00'
  export let ariaLabel = 'Valor em reais'

  let focused = false
  let display = format(value)

  function format(cents: number): string {
    return centsToReais(cents).toFixed(2).replace('.', ',')
  }

  $: if (!focused) {
    const next = format(value)
    if (next !== display) display = next
  }

  function onInput(): void {
    value = reaisToCents(display)
  }

  function onFocus(): void {
    focused = true
    display = format(value)
  }

  function onBlur(): void {
    focused = false
    if (!allowNegative && value < 0) value = 0
    display = format(value)
  }
</script>

<div class="money-field">
  <span>R$</span>
  <input
    class="input"
    inputmode="decimal"
    {disabled}
    {placeholder}
    aria-label={ariaLabel}
    bind:value={display}
    on:input={onInput}
    on:focus={onFocus}
    on:blur={onBlur}
  />
</div>

<style>
  .money-field {
    display: flex;
    align-items: center;
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    background: #fff;
    padding-left: 9px;
    min-height: 34px;
    transition: border-color 0.1s ease, box-shadow 0.1s ease;
  }
  .money-field:focus-within {
    border-color: var(--primary);
    box-shadow: 0 0 0 3px rgba(69, 105, 144, 0.14);
  }
  .money-field > span {
    color: var(--muted);
    font-size: 12px;
    font-weight: 600;
  }
  .money-field :global(input.input) {
    border: 0;
    box-shadow: none;
    background: transparent;
    padding-left: 5px;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
</style>
