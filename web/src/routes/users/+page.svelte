<script lang="ts">
	import { get, post, put, del } from '$lib/api';
	import DataTable from '$lib/components/data-table.svelte';
	import { Pencil, Trash2, Plus, X, ShieldOff, Search } from 'lucide-svelte';

	interface User {
		id: string;
		email: string;
		display_name: string;
		role: string;
		status: string;
		force_password_change: boolean;
		created_at: string;
	}

	let users = $state<User[]>([]);
	let loading = $state(true);
	let error = $state('');

	let searchQuery = $state('');
	let sortKey = $state<string>('created_at');
	let sortDir = $state<'asc' | 'desc'>('desc');

	let filteredUsers = $derived.by(() => {
		if (!searchQuery.trim()) return users;
		const q = searchQuery.toLowerCase();
		return users.filter(
			(u) =>
				u.email.toLowerCase().includes(q) ||
				u.display_name.toLowerCase().includes(q) ||
				u.role.toLowerCase().includes(q)
		);
	});

	let sortedUsers = $derived.by(() => {
		const sorted = [...filteredUsers];
		sorted.sort((a, b) => {
			let cmp: number;
			if (sortKey === 'created_at') {
				cmp = new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
			} else {
				const aVal = String(a[sortKey as keyof User] ?? '');
				const bVal = String(b[sortKey as keyof User] ?? '');
				cmp = aVal.localeCompare(bVal);
			}
			return sortDir === 'asc' ? cmp : -cmp;
		});
		return sorted;
	});

	function handleSort(key: string) {
		if (sortKey === key) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
			return;
		}
		sortKey = key;
		sortDir = 'asc';
	}

	let showForm = $state(false);
	let editingUser = $state<User | null>(null);
	let formEmail = $state('');
	let formName = $state('');
	let formRole = $state('user');
	let formPassword = $state('');
	let formForcePasswordChange = $state(true);
	let formError = $state('');
	let formLoading = $state(false);

	let showDeleteConfirm = $state<User | null>(null);
	let showReset2FA = $state<User | null>(null);

	const columns = [
		{ key: 'email' as const, label: 'Email' },
		{ key: 'display_name' as const, label: 'Name' },
		{
			key: 'role' as const,
			label: 'Role',
			render: (v: unknown) => {
				const val = v as string;
				return val.charAt(0).toUpperCase() + val.slice(1);
			}
		},
		{
			key: 'status' as const,
			label: 'Status',
			render: (v: unknown) => {
				const val = v as string;
				return val === 'active' ? 'Active' : 'Disabled';
			}
		},
		{
			key: 'created_at' as const,
			label: 'Created',
			render: (v: unknown) => new Date(v as string).toLocaleDateString()
		}
	];

	$effect(() => {
		loadUsers();
	});

	async function loadUsers() {
		loading = true;
		error = '';
		try {
			users = await get<User[]>('/users');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load users';
		} finally {
			loading = false;
		}
	}

	function openCreate() {
		editingUser = null;
		formEmail = '';
		formName = '';
		formRole = 'user';
		formPassword = '';
		formForcePasswordChange = true;
		formError = '';
		showForm = true;
	}

	function openEdit(user: User) {
		editingUser = user;
		formEmail = user.email;
		formName = user.display_name;
		formRole = user.role;
		formPassword = '';
		formForcePasswordChange = user.force_password_change;
		formError = '';
		showForm = true;
	}

	function closeForm() {
		showForm = false;
		editingUser = null;
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		if (!formEmail || !formName) {
			formError = 'Email and name are required';
			return;
		}

		if (!editingUser && !formPassword) {
			formError = 'Password is required for new users';
			return;
		}

		formLoading = true;
		formError = '';

		try {
			const body: Record<string, string | boolean> = {
				email: formEmail,
				display_name: formName,
				role: formRole,
				force_password_change: formForcePasswordChange
			};
			if (formPassword) body.password = formPassword;

			if (editingUser) {
				await put(`/users/${editingUser.id}`, body);
			}

			if (!editingUser) {
				await post('/users', body);
			}

			closeForm();
			await loadUsers();
		} catch (err) {
			formError = err instanceof Error ? err.message : 'Failed to save user';
		} finally {
			formLoading = false;
		}
	}

	async function confirmDelete() {
		if (!showDeleteConfirm) return;

		try {
			await del(`/users/${showDeleteConfirm.id}`);
			showDeleteConfirm = null;
			await loadUsers();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete user';
			showDeleteConfirm = null;
		}
	}

	async function confirmReset2FA() {
		if (!showReset2FA) return;

		try {
			await post(`/users/${showReset2FA.id}/reset-2fa`);
			showReset2FA = null;
			await loadUsers();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to reset 2FA';
			showReset2FA = null;
		}
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-2xl font-bold">Users</h1>
		<button
			onclick={openCreate}
			class="flex items-center gap-2 rounded-lg bg-accent px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
		>
			<Plus class="h-4 w-4" />
			Add User
		</button>
	</div>

	{#if error}
		<div class="mb-4 rounded-lg border border-danger/30 bg-danger/10 p-4 text-danger">{error}</div>
	{/if}

	<div class="mb-4">
		<div class="relative">
			<Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted" />
			<input
				type="text"
				placeholder="Search users..."
				bind:value={searchQuery}
				class="w-full rounded-lg border border-border-primary bg-bg-primary py-2 pl-10 pr-3 text-sm outline-none focus:border-accent"
			/>
		</div>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-20 text-text-muted">Loading...</div>
	{/if}

	{#if !loading}
		<DataTable {columns} rows={sortedUsers} {sortKey} {sortDir} onSort={handleSort}>
			{#snippet actions(user)}
				<div class="flex items-center justify-end gap-1">
					<button
						onclick={() => openEdit(user)}
						class="rounded p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-text-primary"
						aria-label="Edit user"
					>
						<Pencil class="h-4 w-4" />
					</button>
					<button
						onclick={() => (showReset2FA = user)}
						class="rounded p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-warning"
						aria-label="Reset 2FA"
						title="Reset 2FA"
					>
						<ShieldOff class="h-4 w-4" />
					</button>
					<button
						onclick={() => (showDeleteConfirm = user)}
						class="rounded p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-danger"
						aria-label="Delete user"
					>
						<Trash2 class="h-4 w-4" />
					</button>
				</div>
			{/snippet}
		</DataTable>
	{/if}

	<!-- Create/Edit Form Modal -->
	{#if showForm}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
			<div class="w-full max-w-md rounded-xl border border-border-primary bg-bg-secondary p-6">
				<div class="mb-4 flex items-center justify-between">
					<h2 class="text-lg font-semibold">{editingUser ? 'Edit User' : 'Create User'}</h2>
					<button onclick={closeForm} class="text-text-muted hover:text-text-primary">
						<X class="h-5 w-5" />
					</button>
				</div>

				<form onsubmit={handleSubmit} class="space-y-4">
					{#if formError}
						<div class="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger">
							{formError}
						</div>
					{/if}

					<div>
						<label for="user-email" class="mb-1.5 block text-sm text-text-secondary">Email</label
						>
						<input
							id="user-email"
							type="email"
							bind:value={formEmail}
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<div>
						<label for="user-name" class="mb-1.5 block text-sm text-text-secondary">Name</label>
						<input
							id="user-name"
							type="text"
							bind:value={formName}
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<div>
						<label for="user-role" class="mb-1.5 block text-sm text-text-secondary">Role</label>
						<select
							id="user-role"
							bind:value={formRole}
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						>
							<option value="user">User</option>
							<option value="admin">Admin</option>
						</select>
					</div>

					<div>
						<label for="user-password" class="mb-1.5 block text-sm text-text-secondary">
							Password {editingUser ? '(leave blank to keep current)' : ''}
						</label>
						<input
							id="user-password"
							type="password"
							bind:value={formPassword}
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<div class="flex items-center gap-2">
						<input
							id="user-force-password-change"
							type="checkbox"
							bind:checked={formForcePasswordChange}
							class="h-4 w-4 rounded border-border-primary accent-accent"
						/>
						<label for="user-force-password-change" class="text-sm text-text-secondary">
							Require password change on next login
						</label>
					</div>

					<div class="flex justify-end gap-2 pt-2">
						<button
							type="button"
							onclick={closeForm}
							class="rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
						>
							Cancel
						</button>
						<button
							type="submit"
							disabled={formLoading}
							class="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
						>
							{formLoading ? 'Saving...' : 'Save'}
						</button>
					</div>
				</form>
			</div>
		</div>
	{/if}

	<!-- Delete Confirmation Modal -->
	{#if showDeleteConfirm}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
			<div class="w-full max-w-sm rounded-xl border border-border-primary bg-bg-secondary p-6">
				<h2 class="mb-2 text-lg font-semibold">Delete User</h2>
				<p class="mb-4 text-sm text-text-secondary">
					Are you sure you want to delete <strong>{showDeleteConfirm.email}</strong>? This action
					cannot be undone.
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

	<!-- Reset 2FA Confirmation Modal -->
	{#if showReset2FA}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
			<div class="w-full max-w-sm rounded-xl border border-border-primary bg-bg-secondary p-6">
				<h2 class="mb-2 text-lg font-semibold">Reset 2FA</h2>
				<p class="mb-4 text-sm text-text-secondary">
					Are you sure you want to reset all two-factor authentication for <strong
						>{showReset2FA.email}</strong
					>? This will remove their TOTP setup, security keys, and recovery codes.
				</p>
				<div class="flex justify-end gap-2">
					<button
						onclick={() => (showReset2FA = null)}
						class="rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
					>
						Cancel
					</button>
					<button
						onclick={confirmReset2FA}
						class="rounded-lg bg-warning px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-warning/80"
					>
						Reset 2FA
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
