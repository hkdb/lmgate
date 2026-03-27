<script lang="ts">
	import { get, post, put, del } from '$lib/api';
	import DataTable from '$lib/components/data-table.svelte';
	import { Pencil, Trash2, Plus, X, Search, UserPlus, UserMinus } from 'lucide-svelte';

	interface Group {
		id: string;
		name: string;
		description: string;
		source: string;
		source_id: string;
		admin_role: boolean;
		member_count: number;
		created_at: string;
		updated_at: string;
	}

	interface GroupMember {
		user_id: string;
		email: string;
		display_name: string;
	}

	let groups = $state<Group[]>([]);
	let loading = $state(true);
	let error = $state('');

	let searchQuery = $state('');
	let sortKey = $state<string>('created_at');
	let sortDir = $state<'asc' | 'desc'>('desc');

	let filteredGroups = $derived.by(() => {
		if (!searchQuery.trim()) return groups;
		const q = searchQuery.toLowerCase();
		return groups.filter(
			(g) =>
				g.name.toLowerCase().includes(q) ||
				g.description.toLowerCase().includes(q) ||
				g.source.toLowerCase().includes(q)
		);
	});

	let sortedGroups = $derived.by(() => {
		const sorted = [...filteredGroups];
		sorted.sort((a, b) => {
			let cmp: number;
			switch (sortKey) {
				case 'created_at':
					cmp = new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
					break;
				case 'member_count':
					cmp = a.member_count - b.member_count;
					break;
				default: {
					const aVal = String(a[sortKey as keyof Group] ?? '');
					const bVal = String(b[sortKey as keyof Group] ?? '');
					cmp = aVal.localeCompare(bVal);
					break;
				}
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

	// Create/Edit form
	let showForm = $state(false);
	let editingGroup = $state<Group | null>(null);
	let formName = $state('');
	let formDescription = $state('');
	let formAdminRole = $state(false);
	let formError = $state('');
	let formLoading = $state(false);

	// Delete
	let showDeleteConfirm = $state<Group | null>(null);

	// Detail / members
	let selectedGroup = $state<Group | null>(null);
	let members = $state<GroupMember[]>([]);
	let membersLoading = $state(false);
	let showAddMember = $state(false);
	let addMemberEmail = $state('');
	let addMemberError = $state('');
	let addMemberLoading = $state(false);

	const columns = [
		{ key: 'name' as const, label: 'Name' },
		{ key: 'description' as const, label: 'Description' },
		{
			key: 'admin_role' as const,
			label: 'Admin Role',
			render: (v: unknown) => (v ? 'Admin' : '')
		},
		{ key: 'member_count' as const, label: 'Members' },
		{
			key: 'created_at' as const,
			label: 'Created',
			render: (v: unknown) => new Date(v as string).toLocaleDateString()
		}
	];

	$effect(() => {
		loadGroups();
	});

	async function loadGroups() {
		loading = true;
		error = '';
		try {
			groups = await get<Group[]>('/groups');
			if (!groups) groups = [];
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load groups';
		} finally {
			loading = false;
		}
	}

	function openCreate() {
		editingGroup = null;
		formName = '';
		formDescription = '';
		formAdminRole = false;
		formError = '';
		showForm = true;
	}

	function openEdit(group: Group) {
		editingGroup = group;
		formName = group.name;
		formDescription = group.description;
		formAdminRole = group.admin_role;
		formError = '';
		showForm = true;
	}

	function closeForm() {
		showForm = false;
		editingGroup = null;
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		if (!formName) {
			formError = 'Name is required';
			return;
		}

		formLoading = true;
		formError = '';

		try {
			const body = { name: formName, description: formDescription, admin_role: formAdminRole };

			if (editingGroup) {
				await put(`/groups/${editingGroup.id}`, body);
			}

			if (!editingGroup) {
				await post('/groups', body);
			}

			closeForm();
			await loadGroups();
		} catch (err) {
			formError = err instanceof Error ? err.message : 'Failed to save group';
		} finally {
			formLoading = false;
		}
	}

	async function confirmDelete() {
		if (!showDeleteConfirm) return;

		try {
			await del(`/groups/${showDeleteConfirm.id}`);
			showDeleteConfirm = null;
			if (selectedGroup?.id === showDeleteConfirm?.id) {
				selectedGroup = null;
			}
			await loadGroups();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete group';
			showDeleteConfirm = null;
		}
	}

	async function openDetail(group: Group) {
		selectedGroup = group;
		membersLoading = true;
		try {
			const data = await get<{ group: Group; members: GroupMember[] }>(`/groups/${group.id}`);
			members = data.members ?? [];
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load group details';
		} finally {
			membersLoading = false;
		}
	}

	function closeDetail() {
		selectedGroup = null;
		members = [];
	}

	function openAddMember() {
		addMemberEmail = '';
		addMemberError = '';
		showAddMember = true;
	}

	async function handleAddMember(e: Event) {
		e.preventDefault();
		if (!addMemberEmail || !selectedGroup) {
			addMemberError = 'Email is required';
			return;
		}

		addMemberLoading = true;
		addMemberError = '';
		try {
			await post(`/groups/${selectedGroup.id}/members`, { email: addMemberEmail });
			showAddMember = false;
			await openDetail(selectedGroup);
			await loadGroups();
		} catch (err) {
			addMemberError = err instanceof Error ? err.message : 'Failed to add member';
		} finally {
			addMemberLoading = false;
		}
	}

	async function removeMember(member: GroupMember) {
		if (!selectedGroup) return;
		try {
			await del(`/groups/${selectedGroup.id}/members/${member.user_id}`);
			await openDetail(selectedGroup);
			await loadGroups();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to remove member';
		}
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-2xl font-bold">Groups</h1>
		<button
			onclick={openCreate}
			class="flex items-center gap-2 rounded-lg bg-accent px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
		>
			<Plus class="h-4 w-4" />
			Add Group
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
				placeholder="Search groups..."
				bind:value={searchQuery}
				class="w-full rounded-lg border border-border-primary bg-bg-primary py-2 pl-10 pr-3 text-sm outline-none focus:border-accent"
			/>
		</div>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-20 text-text-muted">Loading...</div>
	{/if}

	{#if !loading}
		<DataTable {columns} rows={sortedGroups} {sortKey} {sortDir} onSort={handleSort}>
			{#snippet actions(group)}
				<div class="flex items-center justify-end gap-1">
					<button
						onclick={() => openDetail(group)}
						class="rounded p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-accent"
						aria-label="View members"
						title="View members"
					>
						<UserPlus class="h-4 w-4" />
					</button>
					<button
						onclick={() => openEdit(group)}
						class="rounded p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-text-primary"
						aria-label="Edit group"
					>
						<Pencil class="h-4 w-4" />
					</button>
					<button
						onclick={() => (showDeleteConfirm = group)}
						class="rounded p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-danger"
						aria-label="Delete group"
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
					<h2 class="text-lg font-semibold">{editingGroup ? 'Edit Group' : 'Create Group'}</h2>
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
						<label for="group-name" class="mb-1.5 block text-sm text-text-secondary">Name</label>
						<input
							id="group-name"
							type="text"
							bind:value={formName}
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<div>
						<label for="group-description" class="mb-1.5 block text-sm text-text-secondary">Description</label>
						<textarea
							id="group-description"
							bind:value={formDescription}
							rows="3"
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						></textarea>
					</div>

					<div class="flex items-center gap-2">
						<input
							id="group-admin-role"
							type="checkbox"
							bind:checked={formAdminRole}
							class="h-4 w-4 rounded border-border-primary accent-accent"
						/>
						<label for="group-admin-role" class="text-sm text-text-secondary">
							Admin Role
						</label>
						<p class="text-xs text-text-muted">Members of this group are automatically granted admin role</p>
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
				<h2 class="mb-2 text-lg font-semibold">Delete Group</h2>
				<p class="mb-4 text-sm text-text-secondary">
					Are you sure you want to delete <strong>{showDeleteConfirm.name}</strong>? This will
					remove all memberships and associated access rules.
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

	<!-- Group Detail / Members Modal -->
	{#if selectedGroup}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
			<div class="max-h-[80vh] w-full max-w-lg overflow-y-auto rounded-xl border border-border-primary bg-bg-secondary p-6">
				<div class="mb-4 flex items-center justify-between">
					<div>
						<h2 class="text-lg font-semibold">{selectedGroup.name}</h2>
						{#if selectedGroup.admin_role}
							<span class="mt-1 inline-block rounded-full bg-accent/10 px-2 py-0.5 text-xs font-medium text-accent">
								Admin Role
							</span>
						{/if}
					</div>
					<button onclick={closeDetail} class="text-text-muted hover:text-text-primary">
						<X class="h-5 w-5" />
					</button>
				</div>

				{#if selectedGroup.description}
					<p class="mb-4 text-sm text-text-secondary">{selectedGroup.description}</p>
				{/if}

				<div class="mb-3 flex items-center justify-between">
					<h3 class="text-sm font-medium text-text-secondary">Members</h3>
					<button
						onclick={openAddMember}
						class="flex items-center gap-1 rounded-lg bg-accent px-2 py-1 text-xs font-medium text-white transition-colors hover:bg-accent-hover"
					>
						<UserPlus class="h-3 w-3" />
						Add Member
					</button>
				</div>

				{#if membersLoading}
					<div class="py-4 text-center text-text-muted">Loading...</div>
				{/if}

				{#if !membersLoading && members.length === 0}
					<div class="rounded-lg border border-border-primary bg-bg-primary p-4 text-center text-sm text-text-muted">
						No members
					</div>
				{/if}

				{#if !membersLoading && members.length > 0}
					<div class="space-y-2">
						{#each members as member}
							<div class="flex items-center justify-between rounded-lg border border-border-primary bg-bg-primary px-3 py-2">
								<div>
									<span class="text-sm font-medium">{member.email}</span>
									{#if member.display_name}
										<span class="ml-2 text-xs text-text-muted">{member.display_name}</span>
									{/if}
								</div>
								<button
									onclick={() => removeMember(member)}
									class="rounded p-1 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-danger"
									aria-label="Remove member"
								>
									<UserMinus class="h-4 w-4" />
								</button>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	{/if}

	<!-- Add Member Modal -->
	{#if showAddMember}
		<div class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4">
			<div class="w-full max-w-sm rounded-xl border border-border-primary bg-bg-secondary p-6">
				<h2 class="mb-4 text-lg font-semibold">Add Member</h2>

				<form onsubmit={handleAddMember} class="space-y-4">
					{#if addMemberError}
						<div class="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger">
							{addMemberError}
						</div>
					{/if}

					<div>
						<label for="member-email" class="mb-1.5 block text-sm text-text-secondary">
							User Email
						</label>
						<input
							id="member-email"
							type="email"
							bind:value={addMemberEmail}
							placeholder="user@example.com"
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<div class="flex justify-end gap-2 pt-2">
						<button
							type="button"
							onclick={() => (showAddMember = false)}
							class="rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
						>
							Cancel
						</button>
						<button
							type="submit"
							disabled={addMemberLoading}
							class="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
						>
							{addMemberLoading ? 'Adding...' : 'Add'}
						</button>
					</div>
				</form>
			</div>
		</div>
	{/if}
</div>
