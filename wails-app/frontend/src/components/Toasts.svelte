<script lang="ts">
  import { toasts, dismiss } from '../lib/toast'
  import Icon from './Icon.svelte'
</script>

<div class="toast-stack" aria-live="polite">
  {#each $toasts as toast (toast.id)}
    <div class="toast {toast.kind}">
      <span class="toast-icon">
        <Icon name={toast.kind === 'error' ? 'warning' : toast.kind === 'success' ? 'check' : 'info'} size={16} />
      </span>
      <div>
        {#if toast.title}<strong>{toast.title}</strong><br />{/if}
        {toast.message}
      </div>
      <button class="toast-close" on:click={() => dismiss(toast.id)} aria-label="Fechar aviso">×</button>
    </div>
  {/each}
</div>
