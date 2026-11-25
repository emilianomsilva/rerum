<script lang="ts">
  import { confirmState, resolveConfirm } from '../lib/confirm'
  import Modal from './Modal.svelte'
</script>

{#if $confirmState.open}
  <Modal
    narrow
    title={$confirmState.title}
    onClose={() => resolveConfirm(false)}
  >
    <p style="white-space: pre-line; line-height: 1.55">{$confirmState.message}</p>
    <div slot="footer" class="flex" style="justify-content:flex-end; width:100%">
      <button class="btn secondary" on:click={() => resolveConfirm(false)}>
        {$confirmState.cancelLabel}
      </button>
      <button
        class="btn"
        class:destructive={$confirmState.destructive}
        on:click={() => resolveConfirm(true)}
      >
        {$confirmState.confirmLabel}
      </button>
    </div>
  </Modal>
{/if}
