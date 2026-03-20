<script lang="ts">
	import { AlertTriangle } from 'lucide-svelte';

	interface Props {
		open: boolean;
		title?: string;
		message: string;
		confirmLabel?: string;
		cancelLabel?: string;
		variant?: 'danger' | 'warning' | 'default';
		onconfirm: () => void;
		oncancel: () => void;
	}

	let {
		open = $bindable(false),
		title = 'Confirm',
		message,
		confirmLabel = 'Confirm',
		cancelLabel = 'Cancel',
		variant = 'default',
		onconfirm,
		oncancel
	}: Props = $props();

	function handleConfirm() {
		open = false;
		onconfirm();
	}

	function handleCancel() {
		open = false;
		oncancel();
	}

	function handleBackdrop(e: MouseEvent) {
		if (e.target === e.currentTarget) {
			handleCancel();
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			handleCancel();
		}
	}

	let confirmButtonClass = $derived(
		variant === 'danger'
			? 'bg-danger text-white hover:bg-danger-hover'
			: variant === 'warning'
				? 'bg-warning text-black hover:brightness-110'
				: 'bg-accent text-white hover:bg-accent-hover'
	);

	let iconColor = $derived(
		variant === 'danger'
			? 'text-danger'
			: variant === 'warning'
				? 'text-warning'
				: 'text-accent'
	);
</script>

{#if open}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions a11y_interactive_supports_focus -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
		role="dialog"
		aria-modal="true"
		aria-labelledby="confirm-dialog-title"
		tabindex="-1"
		onclick={handleBackdrop}
		onkeydown={handleKeydown}
	>
		<div class="mx-4 w-full max-w-md rounded-xl border border-border-primary bg-bg-secondary p-6 shadow-2xl">
			<div class="mb-4 flex items-start gap-3">
				<div class="mt-0.5 rounded-lg bg-bg-tertiary p-2 {iconColor}">
					<AlertTriangle class="h-5 w-5" />
				</div>
				<div>
					<h2 id="confirm-dialog-title" class="text-lg font-semibold">{title}</h2>
					<p class="mt-1 text-sm text-text-secondary">{message}</p>
				</div>
			</div>

			<div class="flex justify-end gap-3">
				<button
					onclick={handleCancel}
					class="rounded-lg border border-border-primary px-4 py-2 text-sm font-medium text-text-secondary transition-colors hover:bg-bg-tertiary"
				>
					{cancelLabel}
				</button>
				<button
					onclick={handleConfirm}
					class="rounded-lg px-4 py-2 text-sm font-medium transition-colors {confirmButtonClass}"
				>
					{confirmLabel}
				</button>
			</div>
		</div>
	</div>
{/if}
