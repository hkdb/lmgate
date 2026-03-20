<script lang="ts">
	import { get, post, del } from '$lib/api';
	import { auth } from '$lib/stores.svelte';
	import DataTable from '$lib/components/data-table.svelte';
	import { Plus, Trash2, Ban, Copy, Check, X } from 'lucide-svelte';

	interface Token {
		id: string;
		name: string;
		prefix: string;
		user_email: string;
		created_at: string;
		expires_at: string | null;
		last_used_at: string | null;
		status: string;
	}

	interface CreateTokenResponse {
		token: string;
		id: string;
	}

	let tokens = $state<Token[]>([]);
	let loading = $state(true);
	let error = $state('');

	let showCreate = $state(false);
	let tokenName = $state('');
	let tokenExpiry = $state('');
	let createError = $state('');
	let createLoading = $state(false);

	let rawToken = $state<string | null>(null);
	let copied = $state(false);

	let showRevokeConfirm = $state<Token | null>(null);
	let showDeleteConfirm = $state<Token | null>(null);

	let apiBase = $derived(auth.isAdmin ? '/tokens' : '/my/tokens');

	const allColumns = [
		{ key: 'name' as const, label: 'Name' },
		{ key: 'prefix' as const, label: 'Token', render: (v: unknown) => String(v) },
		{ key: 'user_email' as const, label: 'Owner' },
		{
			key: 'status' as const,
			label: 'Status',
			render: (v: unknown) => {
				const val = v as string;
				switch (val) {
					case 'active': return 'Active';
					case 'expired': return 'Expired';
					default: return 'Revoked';
				}
			}
		},
		{
			key: 'created_at' as const,
			label: 'Created',
			render: (v: unknown) => new Date(v as string).toLocaleDateString()
		},
		{
			key: 'last_used_at' as const,
			label: 'Last Used',
			render: (v: unknown) => {
				if (!v) return 'Never';
				return new Date(v as string).toLocaleDateString();
			}
		}
	];

	let columns = $derived(
		auth.isAdmin ? allColumns : allColumns.filter((c) => c.key !== 'user_email')
	);

	$effect(() => {
		loadTokens();
	});

	async function loadTokens() {
		loading = true;
		error = '';
		try {
			tokens = await get<Token[]>(apiBase);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load tokens';
		} finally {
			loading = false;
		}
	}

	function openCreate() {
		tokenName = '';
		tokenExpiry = '';
		createError = '';
		rawToken = null;
		showCreate = true;
	}

	function closeCreate() {
		showCreate = false;
		rawToken = null;
	}

	async function handleCreate(e: Event) {
		e.preventDefault();
		if (!tokenName) {
			createError = 'Token name is required';
			return;
		}

		createLoading = true;
		createError = '';
		try {
			const body: Record<string, string> = { name: tokenName };
			if (tokenExpiry) body.expires_at = tokenExpiry;

			const res = await post<CreateTokenResponse>(apiBase, body);
			rawToken = res.token;
			await loadTokens();
		} catch (err) {
			createError = err instanceof Error ? err.message : 'Failed to create token';
		} finally {
			createLoading = false;
		}
	}

	async function copyToken() {
		if (!rawToken) return;
		await navigator.clipboard.writeText(rawToken);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}

	async function confirmRevoke() {
		if (!showRevokeConfirm) return;
		try {
			await post(`${apiBase}/${showRevokeConfirm.id}/revoke`, {});
			showRevokeConfirm = null;
			await loadTokens();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to revoke token';
			showRevokeConfirm = null;
		}
	}

	async function confirmDelete() {
		if (!showDeleteConfirm) return;
		try {
			await del(`${apiBase}/${showDeleteConfirm.id}`);
			showDeleteConfirm = null;
			await loadTokens();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete token';
			showDeleteConfirm = null;
		}
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-2xl font-bold">API Tokens</h1>
		<button
			onclick={openCreate}
			class="flex items-center gap-2 rounded-lg bg-accent px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
		>
			<Plus class="h-4 w-4" />
			Create Token
		</button>
	</div>

	{#if error}
		<div class="mb-4 rounded-lg border border-danger/30 bg-danger/10 p-4 text-danger">{error}</div>
	{/if}

	{#if loading}
		<div class="flex items-center justify-center py-20 text-text-muted">Loading...</div>
	{/if}

	{#if !loading}
		<DataTable {columns} rows={tokens}>
			{#snippet actions(token)}
				{#if token.status === 'active'}
					<button
						onclick={() => (showRevokeConfirm = token)}
						class="rounded p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-warning"
						aria-label="Revoke token"
					>
						<Ban class="h-4 w-4" />
					</button>
				{/if}
				<button
					onclick={() => (showDeleteConfirm = token)}
					class="rounded p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-danger"
					aria-label="Delete token"
				>
					<Trash2 class="h-4 w-4" />
				</button>
			{/snippet}
		</DataTable>
	{/if}

	<!-- Create Token Modal -->
	{#if showCreate}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
			<div class="w-full max-w-md rounded-xl border border-border-primary bg-bg-secondary p-6">
				<div class="mb-4 flex items-center justify-between">
					<h2 class="text-lg font-semibold">Create API Token</h2>
					<button onclick={closeCreate} class="text-text-muted hover:text-text-primary">
						<X class="h-5 w-5" />
					</button>
				</div>

				{#if rawToken}
					<div class="space-y-4">
						<div
							class="rounded-lg border border-warning/30 bg-warning/10 p-3 text-sm text-warning"
						>
							Copy this token now. You won't be able to see it again.
						</div>
						<div class="flex items-center gap-2">
							<code
								class="flex-1 overflow-x-auto rounded-lg bg-bg-primary p-3 text-sm break-all"
							>
								{rawToken}
							</code>
							<button
								onclick={copyToken}
								class="shrink-0 rounded-lg border border-border-primary p-2 transition-colors hover:bg-bg-tertiary"
								aria-label="Copy token"
							>
								{#if copied}
									<Check class="h-4 w-4 text-success" />
								{/if}
								{#if !copied}
									<Copy class="h-4 w-4" />
								{/if}
							</button>
						</div>
						<div class="flex justify-end">
							<button
								onclick={closeCreate}
								class="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
							>
								Done
							</button>
						</div>
					</div>
				{/if}

				{#if !rawToken}
					<form onsubmit={handleCreate} class="space-y-4">
						{#if createError}
							<div
								class="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger"
							>
								{createError}
							</div>
						{/if}

						<div>
							<label for="token-name" class="mb-1.5 block text-sm text-text-secondary"
								>Name</label
							>
							<input
								id="token-name"
								type="text"
								bind:value={tokenName}
								placeholder="My API token"
								class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
							/>
						</div>

						<div>
							<label for="token-expiry" class="mb-1.5 block text-sm text-text-secondary">
								Expiry (optional)
							</label>
							<input
								id="token-expiry"
								type="date"
								bind:value={tokenExpiry}
								class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
							/>
						</div>

						<div class="flex justify-end gap-2 pt-2">
							<button
								type="button"
								onclick={closeCreate}
								class="rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
							>
								Cancel
							</button>
							<button
								type="submit"
								disabled={createLoading}
								class="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
							>
								{createLoading ? 'Creating...' : 'Create'}
							</button>
						</div>
					</form>
				{/if}
			</div>
		</div>
	{/if}

	<!-- Revoke Confirmation -->
	{#if showRevokeConfirm}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
			<div class="w-full max-w-sm rounded-xl border border-border-primary bg-bg-secondary p-6">
				<h2 class="mb-2 text-lg font-semibold">Revoke Token</h2>
				<p class="mb-4 text-sm text-text-secondary">
					Are you sure you want to revoke <strong>{showRevokeConfirm.name}</strong>? Applications
					using this token will lose access immediately.
				</p>
				<div class="flex justify-end gap-2">
					<button
						onclick={() => (showRevokeConfirm = null)}
						class="rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
					>
						Cancel
					</button>
					<button
						onclick={confirmRevoke}
						class="rounded-lg bg-danger px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-danger-hover"
					>
						Revoke
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Delete Confirmation -->
	{#if showDeleteConfirm}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
			<div class="w-full max-w-sm rounded-xl border border-border-primary bg-bg-secondary p-6">
				<h2 class="mb-2 text-lg font-semibold">Delete Token</h2>
				<p class="mb-4 text-sm text-text-secondary">
					Are you sure you want to permanently delete <strong>{showDeleteConfirm.name}</strong>? This action cannot be undone.
				</p>
				<div class="flex justify-end gap-2">
					<button
						onclick={() => (showDeleteConfirm = null)}
						class="rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
					>
						Cancel
					</button>
					<button
						onclick={confirmDelete}
						class="rounded-lg bg-danger px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-danger-hover"
					>
						Delete
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
