<script lang="ts">
	import { get, post, del, streamPost } from '$lib/api';
	import { auth } from '$lib/stores.svelte';
	import { Plus, Trash2, Shield, Download, Search } from 'lucide-svelte';
	import ConfirmDialog from '$lib/components/confirm-dialog.svelte';

	interface ModelACL {
		id: string;
		model_pattern: string;
		user_email: string;
		permission: string;
		created_at: string;
	}

	interface Model {
		id: string;
		name: string;
		provider: string;
		status: string;
	}

	let rules = $state<ModelACL[]>([]);
	let models = $state<Model[]>([]);
	let upstreamType = $state('');
	let loading = $state(true);
	let error = $state('');

	let searchQuery = $state('');

	let filteredModels = $derived.by(() => {
		if (!searchQuery.trim()) return models;
		const q = searchQuery.toLowerCase();
		return models.filter(
			(m) =>
				m.name.toLowerCase().includes(q) ||
				m.provider.toLowerCase().includes(q)
		);
	});

	let showAdd = $state(false);
	let ruleModel = $state('');
	let ruleEmail = $state('');
	let rulePermission = $state('allow');
	let addError = $state('');
	let addLoading = $state(false);

	// Pull model state
	let showPull = $state(false);
	let pullName = $state('');
	let pullError = $state('');
	let pullLoading = $state(false);
	let pullStatus = $state('');
	let pullProgress = $state(0);
	let pullTotal = $state(0);

	// Delete model state
	let showDeleteConfirm = $state(false);
	let deleteTarget = $state<Model | null>(null);

	$effect(() => {
		loadData();
	});

	async function loadData() {
		loading = true;
		error = '';
		try {
			if (auth.isAdmin) {
				const [rulesRes, modelsRes, typeRes] = await Promise.all([
					get<ModelACL[]>('/models/acl'),
					get<Model[]>('/models'),
					get<{ type: string }>('/models/upstream-type')
				]);
				rules = rulesRes;
				models = modelsRes;
				upstreamType = typeRes.type;
			} else {
				models = await get<Model[]>('/models');
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	function openAdd() {
		ruleModel = '';
		ruleEmail = '';
		rulePermission = 'allow';
		addError = '';
		showAdd = true;
	}

	async function handleAdd(e: Event) {
		e.preventDefault();
		if (!ruleModel || !ruleEmail) {
			addError = 'Model pattern and user email are required';
			return;
		}

		addLoading = true;
		addError = '';
		try {
			await post('/models/acl', {
				model_pattern: ruleModel,
				user_email: ruleEmail,
				permission: rulePermission
			});
			showAdd = false;
			await loadData();
		} catch (err) {
			addError = err instanceof Error ? err.message : 'Failed to add rule';
		} finally {
			addLoading = false;
		}
	}

	async function removeRule(rule: ModelACL) {
		try {
			await del(`/models/acl/${rule.id}`);
			await loadData();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to remove rule';
		}
	}

	function permissionBadgeClass(perm: string): string {
		if (perm === 'allow') return 'bg-success/10 text-success border-success/30';
		return 'bg-danger/10 text-danger border-danger/30';
	}

	function openPull() {
		pullName = '';
		pullError = '';
		pullLoading = false;
		pullStatus = '';
		pullProgress = 0;
		pullTotal = 0;
		showPull = true;
	}

	async function handlePull(e: Event) {
		e.preventDefault();
		if (!pullName.trim()) {
			pullError = 'Model name is required';
			return;
		}

		pullLoading = true;
		pullError = '';
		pullStatus = 'Starting pull...';
		pullProgress = 0;
		pullTotal = 0;

		try {
			const response = await streamPost('/models/pull', { name: pullName.trim() });
			const reader = response.body?.getReader();
			if (!reader) {
				pullError = 'Failed to read stream';
				pullLoading = false;
				return;
			}

			const decoder = new TextDecoder();
			let buffer = '';

			while (true) {
				const { done, value } = await reader.read();
				if (done) break;

				buffer += decoder.decode(value, { stream: true });
				const lines = buffer.split('\n');
				buffer = lines.pop() || '';

				for (const line of lines) {
					if (!line.startsWith('data: ')) continue;
					try {
						const data = JSON.parse(line.slice(6));
						if (data.status) pullStatus = data.status;
						if (data.total) pullTotal = data.total;
						if (data.completed) pullProgress = data.completed;

						if (data.status === 'success') {
							showPull = false;
							await loadData();
							return;
						}

						if (data.error) {
							pullError = data.error;
							pullLoading = false;
							return;
						}
					} catch {
						// skip non-JSON lines
					}
				}
			}

			pullLoading = false;
		} catch (err) {
			pullError = err instanceof Error ? err.message : 'Pull failed';
			pullLoading = false;
		}
	}

	function confirmDelete(model: Model) {
		deleteTarget = model;
		showDeleteConfirm = true;
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		try {
			await post('/models/delete', { name: deleteTarget.name });
			await loadData();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete model';
		}
		deleteTarget = null;
	}

	function formatBytes(bytes: number): string {
		if (bytes === 0) return '0 B';
		const k = 1024;
		const sizes = ['B', 'KB', 'MB', 'GB'];
		const i = Math.floor(Math.log(bytes) / Math.log(k));
		return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-2xl font-bold">{auth.isAdmin ? 'Model Access Control' : 'Available Models'}</h1>
		{#if auth.isAdmin}
			<div class="flex items-center gap-2">
				{#if upstreamType === 'ollama'}
					<button
						onclick={openPull}
						class="flex items-center gap-2 rounded-lg border border-border-primary bg-bg-secondary px-3 py-2 text-sm font-medium transition-colors hover:bg-bg-tertiary"
					>
						<Download class="h-4 w-4" />
						Pull Model
					</button>
				{/if}
				<button
					onclick={openAdd}
					class="flex items-center gap-2 rounded-lg bg-accent px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
				>
					<Plus class="h-4 w-4" />
					Add Rule
				</button>
			</div>
		{/if}
	</div>

	{#if error}
		<div class="mb-4 rounded-lg border border-danger/30 bg-danger/10 p-4 text-danger">{error}</div>
	{/if}

	<!-- Available Models -->
	<div class="mb-6">
		<h2 class="mb-3 text-sm font-medium text-text-secondary">Available Models</h2>
		<div class="relative mb-3">
			<Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted" />
			<input
				type="text"
				placeholder="Search models..."
				bind:value={searchQuery}
				class="w-full rounded-lg border border-border-primary bg-bg-primary py-2 pl-10 pr-3 text-sm outline-none focus:border-accent"
			/>
		</div>
		{#if loading}
			<div class="flex items-center justify-center py-10 text-text-muted">Loading...</div>
		{/if}
		{#if !loading && filteredModels.length === 0}
			<div class="rounded-lg border border-border-primary bg-bg-secondary p-4 text-sm text-text-muted">
				No models available
			</div>
		{/if}
		{#if !loading && filteredModels.length > 0}
			<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
				{#each filteredModels as model}
					<div class="rounded-lg border border-border-primary bg-bg-secondary p-4">
						<div class="flex items-center justify-between">
							<div class="flex items-center gap-2">
								<div
									class="h-2 w-2 rounded-full {model.status === 'available' ? 'bg-success' : 'bg-text-muted'}"
								></div>
								<span class="text-sm font-medium">{model.name}</span>
							</div>
							{#if upstreamType === 'ollama'}
								<button
									onclick={() => confirmDelete(model)}
									class="rounded p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-danger"
									aria-label="Delete model {model.name}"
								>
									<Trash2 class="h-4 w-4" />
								</button>
							{/if}
						</div>
						<div class="mt-1 text-xs text-text-muted">{model.provider}</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>

	{#if auth.isAdmin}
	<!-- ACL Rules -->
	<div>
		<h2 class="mb-3 text-sm font-medium text-text-secondary">Access Rules</h2>
		{#if !loading && rules.length === 0}
			<div class="rounded-lg border border-border-primary bg-bg-secondary p-8 text-center text-sm text-text-muted">
				<Shield class="mx-auto mb-2 h-8 w-8" />
				No access rules configured. All authenticated users can access all models by default.
			</div>
		{/if}
		{#if !loading && rules.length > 0}
			<div class="space-y-2">
				{#each rules as rule}
					<div
						class="flex items-center gap-4 rounded-lg border border-border-primary bg-bg-secondary px-4 py-3"
					>
						<span
							class="rounded-md border px-2 py-0.5 text-xs font-medium {permissionBadgeClass(rule.permission)}"
						>
							{rule.permission.toUpperCase()}
						</span>
						<div class="flex-1">
							<span class="text-sm font-medium">{rule.user_email}</span>
							<span class="mx-2 text-text-muted">→</span>
							<code class="text-sm text-accent">{rule.model_pattern}</code>
						</div>
						<span class="text-xs text-text-muted">
							{new Date(rule.created_at).toLocaleDateString()}
						</span>
						<button
							onclick={() => removeRule(rule)}
							class="rounded p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-danger"
							aria-label="Remove rule"
						>
							<Trash2 class="h-4 w-4" />
						</button>
					</div>
				{/each}
			</div>
		{/if}
	</div>
	{/if}

	<!-- Add Rule Modal -->
	{#if showAdd}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
			<div class="w-full max-w-md rounded-xl border border-border-primary bg-bg-secondary p-6">
				<h2 class="mb-4 text-lg font-semibold">Add Access Rule</h2>

				<form onsubmit={handleAdd} class="space-y-4">
					{#if addError}
						<div class="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger">
							{addError}
						</div>
					{/if}

					<div>
						<label for="rule-model" class="mb-1.5 block text-sm text-text-secondary">
							Model Pattern
						</label>
						<input
							id="rule-model"
							type="text"
							bind:value={ruleModel}
							placeholder="gpt-4* or exact model name"
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
						<p class="mt-1 text-xs text-text-muted">Use * for wildcard matching</p>
					</div>

					<div>
						<label for="rule-email" class="mb-1.5 block text-sm text-text-secondary">
							User Email
						</label>
						<input
							id="rule-email"
							type="email"
							bind:value={ruleEmail}
							placeholder="user@example.com"
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<div>
						<label for="rule-permission" class="mb-1.5 block text-sm text-text-secondary">
							Permission
						</label>
						<select
							id="rule-permission"
							bind:value={rulePermission}
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						>
							<option value="allow">Allow</option>
							<option value="deny">Deny</option>
						</select>
					</div>

					<div class="flex justify-end gap-2 pt-2">
						<button
							type="button"
							onclick={() => (showAdd = false)}
							class="rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
						>
							Cancel
						</button>
						<button
							type="submit"
							disabled={addLoading}
							class="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
						>
							{addLoading ? 'Adding...' : 'Add Rule'}
						</button>
					</div>
				</form>
			</div>
		</div>
	{/if}

	<!-- Pull Model Modal -->
	{#if showPull}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
			<div class="w-full max-w-md rounded-xl border border-border-primary bg-bg-secondary p-6">
				<h2 class="mb-4 text-lg font-semibold">Pull Model</h2>

				<form onsubmit={handlePull} class="space-y-4">
					{#if pullError}
						<div class="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger">
							{pullError}
						</div>
					{/if}

					<div>
						<label for="pull-name" class="mb-1.5 block text-sm text-text-secondary">
							Model Name
						</label>
						<input
							id="pull-name"
							type="text"
							bind:value={pullName}
							placeholder="llama3.2 or namespace/model:tag"
							disabled={pullLoading}
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent disabled:opacity-50"
						/>
						<p class="mt-1 text-xs text-text-muted">
							Browse available models at
							<a
								href="https://ollama.com/library"
								target="_blank"
								rel="noopener noreferrer"
								class="text-accent hover:underline"
							>
								ollama.com/library
							</a>
						</p>
					</div>

					{#if pullLoading}
						<div class="space-y-2">
							<p class="text-sm text-text-secondary">{pullStatus}</p>
							{#if pullTotal > 0}
								<div class="h-2 w-full overflow-hidden rounded-full bg-bg-tertiary">
									<div
										class="h-full rounded-full bg-accent transition-all duration-300"
										style="width: {Math.round((pullProgress / pullTotal) * 100)}%"
									></div>
								</div>
								<p class="text-xs text-text-muted">
									{formatBytes(pullProgress)} / {formatBytes(pullTotal)}
								</p>
							{/if}
						</div>
					{/if}

					<div class="flex justify-end gap-2 pt-2">
						<button
							type="button"
							onclick={() => (showPull = false)}
							class="rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
						>
							{pullLoading ? 'Close' : 'Cancel'}
						</button>
						{#if !pullLoading}
							<button
								type="submit"
								class="flex items-center gap-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
							>
								<Download class="h-4 w-4" />
								Pull
							</button>
						{/if}
					</div>
				</form>
			</div>
		</div>
	{/if}

	<!-- Delete Model Confirm -->
	<ConfirmDialog
		bind:open={showDeleteConfirm}
		title="Delete Model"
		message="Are you sure you want to delete {deleteTarget?.name ?? 'this model'}? This will remove the model files from the Ollama server."
		confirmLabel="Delete"
		variant="danger"
		onconfirm={handleDelete}
		oncancel={() => (deleteTarget = null)}
	/>
</div>
