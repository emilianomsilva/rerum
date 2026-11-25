<script lang="ts">
  import Icon from './Icon.svelte'

  export let title = ''
  export let subtitle = ''
  export let wide = false
  export let narrow = false
  export let onClose: () => void = () => {}
  export let closeOnBackdrop = true

  function keydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      event.preventDefault()
      onClose()
    }
  }
</script>

<svelte:window on:keydown={keydown} />

<div
  class="modal-backdrop"
  role="presentation"
  on:click|self={() => closeOnBackdrop && onClose()}
>
  <div class="modal" class:wide class:narrow role="dialog" aria-modal="true" aria-label={title}>
    <div class="modal-header">
      <div>
        <h2>{title}</h2>
        {#if subtitle}<p>{subtitle}</p>{/if}
      </div>
      <button class="icon-button" type="button" on:click={onClose} aria-label="Fechar">
        <Icon name="close" size={18} />
      </button>
    </div>
    <div class="modal-body">
      <slot />
    </div>
    <div class="modal-footer">
      <slot name="footer" />
    </div>
  </div>
</div>
