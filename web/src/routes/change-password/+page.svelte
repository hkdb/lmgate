<script lang="ts">
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { post } from '$lib/api';
	import { auth, type AuthUser } from '$lib/stores.svelte';
	import { KeyRound } from 'lucide-svelte';

	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let error = $state('');
	let loading = $state(false);

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = '';

		if (!currentPassword || !newPassword || !confirmPassword) {
			error = 'All fields are required';
			return;
		}

		if (newPassword !== confirmPassword) {
			error = 'New passwords do not match';
			return;
		}

		loading = true;
		try {
			const res = await post<AuthUser>('/change-password', {
				current_password: currentPassword,
				new_password: newPassword
			});
			auth.setUser(res);
			if (res.enforce_2fa) {
				await goto(`${base}/account`);
				return;
			}
			await goto(auth.isAdmin ? `${base}/` : `${base}/models`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to change password';
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center px-4">
	<div class="w-full max-w-sm">
		<div class="mb-8 text-center">
			<h1 class="text-2xl font-bold">Change Password</h1>
			<p class="mt-1 text-sm text-text-muted">
				{auth.user?.force_password_change
					? 'You must change your password before continuing.'
					: 'Enter your current password and choose a new one.'}
			</p>
		</div>

		<div class="rounded-xl border border-border-primary bg-bg-secondary p-6">
			<form onsubmit={handleSubmit} class="space-y-4">
				{#if error}
					<div class="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger">
						{error}
					</div>
				{/if}

				<div>
					<label for="current-password" class="mb-1.5 block text-sm text-text-secondary">
						Current Password
					</label>
					<input
						id="current-password"
						type="password"
						bind:value={currentPassword}
						class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none transition-colors placeholder:text-text-muted focus:border-accent"
						autocomplete="current-password"
					/>
				</div>

				<div>
					<label for="new-password" class="mb-1.5 block text-sm text-text-secondary">
						New Password
					</label>
					<input
						id="new-password"
						type="password"
						bind:value={newPassword}
						class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none transition-colors placeholder:text-text-muted focus:border-accent"
						autocomplete="new-password"
					/>
				</div>

				<div>
					<label for="confirm-password" class="mb-1.5 block text-sm text-text-secondary">
						Confirm New Password
					</label>
					<input
						id="confirm-password"
						type="password"
						bind:value={confirmPassword}
						class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none transition-colors placeholder:text-text-muted focus:border-accent"
						autocomplete="new-password"
					/>
				</div>

				<button
					type="submit"
					disabled={loading}
					class="flex w-full items-center justify-center gap-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
				>
					<KeyRound class="h-4 w-4" />
					{loading ? 'Changing...' : 'Change Password'}
				</button>
			</form>
		</div>
	</div>
</div>
