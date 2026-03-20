<script lang="ts">
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { get, post, del, getCsrfToken } from '$lib/api';
	import { auth } from '$lib/stores.svelte';
	import { Shield, KeyRound, Plus, Trash2, X, RefreshCw, Copy, Check } from 'lucide-svelte';
	import {
		startRegistration,
		type PublicKeyCredentialCreationOptionsJSON
	} from '@simplewebauthn/browser';

	interface TwoFAStatus {
		totp_enabled: boolean;
		webauthn_credentials: { id: string; name: string; created_at: string }[];
		recovery_codes_remaining: number;
	}

	interface SetupTOTPResponse {
		qr_code: string;
		manual_key: string;
	}

	interface VerifyTOTPResponse {
		status: string;
		recovery_codes: string[];
	}

	// Password change
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let passwordError = $state('');
	let passwordSuccess = $state('');
	let passwordLoading = $state(false);

	// 2FA status
	let status = $state<TwoFAStatus | null>(null);
	let statusLoading = $state(true);

	// TOTP setup
	let totpSetup = $state<SetupTOTPResponse | null>(null);
	let totpVerifyCode = $state('');
	let totpError = $state('');
	let totpLoading = $state(false);
	let recoveryCodes = $state<string[] | null>(null);

	// TOTP disable
	let showDisableTOTP = $state(false);
	let disableCode = $state('');
	let disableError = $state('');
	let disableLoading = $state(false);

	// WebAuthn
	let webauthnError = $state('');
	let webauthnLoading = $state(false);
	let showAddKey = $state(false);
	let keyName = $state('');
	let showDeleteKey = $state<string | null>(null);

	// Recovery
	let showRegenerate = $state(false);
	let regenCode = $state('');
	let regenError = $state('');
	let regenLoading = $state(false);
	let copiedCodes = $state(false);
	let wasEnforce2fa = $state(false);

	$effect(() => {
		loadStatus();
	});

	$effect(() => {
		if (!auth.user?.enforce_2fa || statusLoading) return;
		document.getElementById('totp-section')?.scrollIntoView({ behavior: 'smooth' });
	});

	async function loadStatus() {
		statusLoading = true;
		try {
			status = await get<TwoFAStatus>('/2fa/status');
		} catch {
			// User might not have 2FA
			status = { totp_enabled: false, webauthn_credentials: [], recovery_codes_remaining: 0 };
		} finally {
			statusLoading = false;
		}
	}

	// --- Password ---

	async function handlePasswordChange(e: Event) {
		e.preventDefault();
		passwordError = '';
		passwordSuccess = '';

		if (!currentPassword || !newPassword) {
			passwordError = 'All fields are required';
			return;
		}
		if (newPassword !== confirmPassword) {
			passwordError = 'Passwords do not match';
			return;
		}

		passwordLoading = true;
		try {
			await post('/change-password', {
				current_password: currentPassword,
				new_password: newPassword
			});
			passwordSuccess = 'Password updated successfully';
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
		} catch (err) {
			passwordError = err instanceof Error ? err.message : 'Failed to change password';
		} finally {
			passwordLoading = false;
		}
	}

	// --- TOTP ---

	async function startTOTPSetup() {
		totpError = '';
		totpLoading = true;
		try {
			totpSetup = await post<SetupTOTPResponse>('/2fa/totp/setup');
		} catch (err) {
			totpError = err instanceof Error ? err.message : 'Failed to start TOTP setup';
		} finally {
			totpLoading = false;
		}
	}

	async function verifyTOTP(e: Event) {
		e.preventDefault();
		if (!totpVerifyCode) {
			totpError = 'Enter the 6-digit code from your authenticator';
			return;
		}

		totpLoading = true;
		totpError = '';
		try {
			const res = await post<VerifyTOTPResponse>('/2fa/totp/verify', { code: totpVerifyCode });
			recoveryCodes = res.recovery_codes;
			totpSetup = null;
			totpVerifyCode = '';
			await loadStatus();
			wasEnforce2fa = auth.user?.enforce_2fa ?? false;
			await auth.initialize();
		} catch (err) {
			totpError = err instanceof Error ? err.message : 'Invalid code';
		} finally {
			totpLoading = false;
		}
	}

	async function disableTOTP(e: Event) {
		e.preventDefault();
		if (!disableCode) {
			disableError = 'Enter your current TOTP code';
			return;
		}
		disableLoading = true;
		disableError = '';
		try {
			await post('/2fa/totp/disable', { code: disableCode });
			showDisableTOTP = false;
			disableCode = '';
			await loadStatus();
		} catch (err) {
			disableError = err instanceof Error ? err.message : 'Failed to disable TOTP';
		} finally {
			disableLoading = false;
		}
	}

	function cancelTOTPSetup() {
		totpSetup = null;
		totpVerifyCode = '';
		totpError = '';
	}

	// --- WebAuthn ---

	async function addWebAuthnKey() {
		webauthnError = '';
		webauthnLoading = true;
		try {
			const options = await post<{ publicKey: PublicKeyCredentialCreationOptionsJSON }>('/2fa/webauthn/register/begin');
			const credential = await startRegistration({
				optionsJSON: options.publicKey
			});

			const name = keyName || 'Security Key';
			const res = await fetch(
				`/admin/api/2fa/webauthn/register/finish?name=${encodeURIComponent(name)}`,
				{
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
						'X-CSRF-Token': getCsrfToken() ?? ''
					},
					body: JSON.stringify(credential),
					credentials: 'same-origin'
				}
			);

			if (!res.ok) {
				const body = await res.json();
				throw new Error(body.error || 'Registration failed');
			}

			const data = await res.json();
			if (data.recovery_codes) {
				recoveryCodes = data.recovery_codes;
			}

			showAddKey = false;
			keyName = '';
			await loadStatus();
			wasEnforce2fa = auth.user?.enforce_2fa ?? false;
			await auth.initialize();
		} catch (err) {
			webauthnError = err instanceof Error ? err.message : 'WebAuthn registration failed';
		} finally {
			webauthnLoading = false;
		}
	}

	async function deleteWebAuthnKey() {
		if (!showDeleteKey) return;
		try {
			await del(`/2fa/webauthn/credentials/${showDeleteKey}`);
			showDeleteKey = null;
			await loadStatus();
		} catch (err) {
			webauthnError = err instanceof Error ? err.message : 'Failed to delete key';
			showDeleteKey = null;
		}
	}

	// --- Recovery ---

	async function regenerateRecoveryCodes(e: Event) {
		e.preventDefault();
		if (!regenCode) {
			regenError = 'Enter your current TOTP code to regenerate';
			return;
		}
		regenLoading = true;
		regenError = '';
		try {
			const res = await post<{ recovery_codes: string[] }>('/2fa/recovery/regenerate', {
				code: regenCode
			});
			recoveryCodes = res.recovery_codes;
			showRegenerate = false;
			regenCode = '';
			await loadStatus();
		} catch (err) {
			regenError = err instanceof Error ? err.message : 'Failed to regenerate codes';
		} finally {
			regenLoading = false;
		}
	}

	function copyRecoveryCodes() {
		if (!recoveryCodes) return;
		navigator.clipboard.writeText(recoveryCodes.join('\n'));
		copiedCodes = true;
		setTimeout(() => (copiedCodes = false), 2000);
	}

	function dismissRecoveryCodes() {
		recoveryCodes = null;
		copiedCodes = false;
		if (wasEnforce2fa) {
			goto(auth.isAdmin ? `${base}/` : `${base}/models`);
		}
	}
</script>

<div class="mx-auto max-w-2xl">
	<h1 class="mb-6 text-2xl font-bold">Account</h1>

	<!-- Password Section -->
	<section class="mb-6 rounded-xl border border-border-primary bg-bg-secondary p-6">
		<h2 class="mb-4 text-lg font-semibold">Change Password</h2>

		<form onsubmit={handlePasswordChange} class="space-y-4">
			{#if passwordError}
				<div class="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger">
					{passwordError}
				</div>
			{/if}
			{#if passwordSuccess}
				<div class="rounded-lg border border-success/30 bg-success/10 p-3 text-sm text-success">
					{passwordSuccess}
				</div>
			{/if}

			<div>
				<label for="current-pw" class="mb-1.5 block text-sm text-text-secondary"
					>Current Password</label
				>
				<input
					id="current-pw"
					type="password"
					bind:value={currentPassword}
					class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
					autocomplete="current-password"
				/>
			</div>
			<div>
				<label for="new-pw" class="mb-1.5 block text-sm text-text-secondary">New Password</label>
				<input
					id="new-pw"
					type="password"
					bind:value={newPassword}
					class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
					autocomplete="new-password"
				/>
			</div>
			<div>
				<label for="confirm-pw" class="mb-1.5 block text-sm text-text-secondary"
					>Confirm New Password</label
				>
				<input
					id="confirm-pw"
					type="password"
					bind:value={confirmPassword}
					class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
					autocomplete="new-password"
				/>
			</div>

			<button
				type="submit"
				disabled={passwordLoading}
				class="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
			>
				{passwordLoading ? 'Updating...' : 'Update Password'}
			</button>
		</form>
	</section>

	{#if statusLoading}
		<div class="flex items-center justify-center py-10 text-text-muted">Loading 2FA status...</div>
	{:else if status}
		<!-- TOTP Section -->
		{#if auth.user?.enforce_2fa}
			<div class="mb-4 rounded-lg border border-warning/30 bg-warning/10 p-4 text-sm text-warning">
				Two-factor authentication is required. Please set up a 2FA method below to continue.
			</div>
		{/if}
		<section id="totp-section" class="mb-6 rounded-xl border border-border-primary bg-bg-secondary p-6">
			<div class="mb-4 flex items-center justify-between">
				<div class="flex items-center gap-2">
					<Shield class="h-5 w-5 text-accent" />
					<h2 class="text-lg font-semibold">Authenticator App</h2>
				</div>
				{#if status.totp_enabled}
					<span
						class="rounded-full bg-success/10 px-2.5 py-0.5 text-xs font-medium text-success"
						>Enabled</span
					>
				{/if}
			</div>

			{#if status.totp_enabled && !totpSetup}
				<p class="mb-4 text-sm text-text-secondary">
					Your authenticator app is configured and active.
				</p>
				{#if !showDisableTOTP}
					<button
						onclick={() => (showDisableTOTP = true)}
						class="rounded-lg border border-danger/30 px-4 py-2 text-sm text-danger transition-colors hover:bg-danger/10"
					>
						Disable TOTP
					</button>
				{:else}
					<form onsubmit={disableTOTP} class="space-y-3">
						{#if disableError}
							<div
								class="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger"
							>
								{disableError}
							</div>
						{/if}
						<div>
							<label for="disable-code" class="mb-1.5 block text-sm text-text-secondary"
								>Enter current TOTP code to confirm</label
							>
							<input
								id="disable-code"
								type="text"
								bind:value={disableCode}
								maxlength="6"
								pattern="[0-9]*"
								inputmode="numeric"
								class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
							/>
						</div>
						<div class="flex gap-2">
							<button
								type="button"
								onclick={() => {
									showDisableTOTP = false;
									disableCode = '';
									disableError = '';
								}}
								class="rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
							>
								Cancel
							</button>
							<button
								type="submit"
								disabled={disableLoading}
								class="rounded-lg bg-danger px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-danger-hover disabled:opacity-50"
							>
								{disableLoading ? 'Disabling...' : 'Disable'}
							</button>
						</div>
					</form>
				{/if}
			{:else if totpSetup}
				<!-- TOTP Setup Flow -->
				<div class="space-y-4">
					<p class="text-sm text-text-secondary">
						Scan this QR code with your authenticator app (Google Authenticator, Authy, etc.)
					</p>
					<div class="flex justify-center rounded-lg bg-white p-4">
						<img src={totpSetup.qr_code} alt="TOTP QR Code" class="h-48 w-48" />
					</div>
					<div>
						<p class="mb-1 text-xs text-text-muted">Or enter this key manually:</p>
						<code
							class="block rounded-lg bg-bg-primary p-2 text-center text-sm tracking-wider"
							>{totpSetup.manual_key}</code
						>
					</div>
					<form onsubmit={verifyTOTP} class="space-y-3">
						{#if totpError}
							<div
								class="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger"
							>
								{totpError}
							</div>
						{/if}
						<div>
							<label for="verify-code" class="mb-1.5 block text-sm text-text-secondary"
								>Verification Code</label
							>
							<input
								id="verify-code"
								type="text"
								bind:value={totpVerifyCode}
								placeholder="000000"
								maxlength="6"
								pattern="[0-9]*"
								inputmode="numeric"
								class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-center text-lg tracking-widest outline-none focus:border-accent"
							/>
						</div>
						<div class="flex gap-2">
							<button
								type="button"
								onclick={cancelTOTPSetup}
								class="rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
							>
								Cancel
							</button>
							<button
								type="submit"
								disabled={totpLoading}
								class="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
							>
								{totpLoading ? 'Verifying...' : 'Verify & Enable'}
							</button>
						</div>
					</form>
				</div>
			{:else}
				<p class="mb-4 text-sm text-text-secondary">
					Add an extra layer of security by requiring a code from your authenticator app when
					signing in.
				</p>
				<button
					onclick={startTOTPSetup}
					disabled={totpLoading}
					class="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
				>
					{totpLoading ? 'Setting up...' : 'Set Up Authenticator'}
				</button>
			{/if}
		</section>

		<!-- Security Keys Section -->
		<section class="mb-6 rounded-xl border border-border-primary bg-bg-secondary p-6">
			<div class="mb-4 flex items-center justify-between">
				<div class="flex items-center gap-2">
					<KeyRound class="h-5 w-5 text-accent" />
					<h2 class="text-lg font-semibold">Security Keys</h2>
				</div>
				<button
					onclick={() => (showAddKey = true)}
					class="flex items-center gap-1 rounded-lg bg-accent px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-accent-hover"
				>
					<Plus class="h-3 w-3" />
					Add Key
				</button>
			</div>

			{#if webauthnError}
				<div class="mb-4 rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger">
					{webauthnError}
				</div>
			{/if}

			{#if status.webauthn_credentials.length === 0}
				<p class="text-sm text-text-muted">
					No security keys registered. Add a FIDO2/WebAuthn security key for passwordless 2FA.
				</p>
			{:else}
				<div class="space-y-2">
					{#each status.webauthn_credentials as cred}
						<div
							class="flex items-center justify-between rounded-lg border border-border-primary bg-bg-primary p-3"
						>
							<div>
								<p class="text-sm font-medium">{cred.name}</p>
								<p class="text-xs text-text-muted">
									Added {new Date(cred.created_at).toLocaleDateString()}
								</p>
							</div>
							<button
								onclick={() => (showDeleteKey = cred.id)}
								class="rounded p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-danger"
								aria-label="Delete key"
							>
								<Trash2 class="h-4 w-4" />
							</button>
						</div>
					{/each}
				</div>
			{/if}
		</section>

		<!-- Recovery Codes Display (shown after setup/regeneration) -->
		{#if recoveryCodes}
			<div class="mb-6 rounded-xl border border-warning/30 bg-warning/10 p-6">
				<h3 class="mb-2 font-semibold text-warning">Save Your Recovery Codes</h3>
				<p class="mb-4 text-sm text-text-secondary">
					Store these codes in a safe place. Each code can only be used once. You won't be able to
					see them again.
				</p>
				<div class="mb-4 grid grid-cols-2 gap-2 rounded-lg bg-bg-primary p-4 font-mono text-sm">
					{#each recoveryCodes as code}
						<div class="text-center">{code}</div>
					{/each}
				</div>
				<div class="flex gap-2">
					<button
						onclick={copyRecoveryCodes}
						class="flex items-center gap-2 rounded-lg border border-border-primary px-3 py-2 text-sm transition-colors hover:bg-bg-tertiary"
					>
						{#if copiedCodes}
							<Check class="h-4 w-4 text-success" />
							Copied
						{:else}
							<Copy class="h-4 w-4" />
							Copy
						{/if}
					</button>
					<button
						onclick={dismissRecoveryCodes}
						class="rounded-lg bg-accent px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
					>
						I've saved these codes
					</button>
				</div>
			</div>
		{/if}

		<!-- Recovery Codes Section -->
		{#if status.totp_enabled || status.webauthn_credentials.length > 0}
			<section class="mb-6 rounded-xl border border-border-primary bg-bg-secondary p-6">
				<div class="mb-4 flex items-center justify-between">
					<h2 class="text-lg font-semibold">Recovery Codes</h2>
					<span class="text-sm text-text-muted"
						>{status.recovery_codes_remaining} remaining</span
					>
				</div>

				<p class="mb-4 text-sm text-text-secondary">
					Recovery codes can be used to access your account if you lose your authenticator or
					security key.
				</p>

				{#if !showRegenerate}
					<button
						onclick={() => (showRegenerate = true)}
						class="flex items-center gap-2 rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
					>
						<RefreshCw class="h-4 w-4" />
						Regenerate Codes
					</button>
				{:else}
					<form onsubmit={regenerateRecoveryCodes} class="space-y-3">
						{#if regenError}
							<div
								class="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger"
							>
								{regenError}
							</div>
						{/if}
						<div>
							<label for="regen-code" class="mb-1.5 block text-sm text-text-secondary"
								>Enter current TOTP code to confirm</label
							>
							<input
								id="regen-code"
								type="text"
								bind:value={regenCode}
								maxlength="6"
								pattern="[0-9]*"
								inputmode="numeric"
								class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
							/>
						</div>
						<div class="flex gap-2">
							<button
								type="button"
								onclick={() => {
									showRegenerate = false;
									regenCode = '';
									regenError = '';
								}}
								class="rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
							>
								Cancel
							</button>
							<button
								type="submit"
								disabled={regenLoading}
								class="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
							>
								{regenLoading ? 'Generating...' : 'Regenerate'}
							</button>
						</div>
					</form>
				{/if}
			</section>
		{/if}
	{/if}
</div>

<!-- Add Key Modal -->
{#if showAddKey}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
		<div class="w-full max-w-sm rounded-xl border border-border-primary bg-bg-secondary p-6">
			<div class="mb-4 flex items-center justify-between">
				<h2 class="text-lg font-semibold">Add Security Key</h2>
				<button
					onclick={() => {
						showAddKey = false;
						keyName = '';
						webauthnError = '';
					}}
					class="text-text-muted hover:text-text-primary"
				>
					<X class="h-5 w-5" />
				</button>
			</div>

			{#if webauthnError}
				<div class="mb-4 rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger">
					{webauthnError}
				</div>
			{/if}

			<div class="space-y-4">
				<div>
					<label for="key-name" class="mb-1.5 block text-sm text-text-secondary"
						>Key Name</label
					>
					<input
						id="key-name"
						type="text"
						bind:value={keyName}
						placeholder="e.g., YubiKey 5"
						class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
					/>
				</div>
				<button
					onclick={addWebAuthnKey}
					disabled={webauthnLoading}
					class="flex w-full items-center justify-center gap-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
				>
					<KeyRound class="h-4 w-4" />
					{webauthnLoading ? 'Waiting for key...' : 'Register Key'}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Delete Key Confirmation -->
{#if showDeleteKey}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
		<div class="w-full max-w-sm rounded-xl border border-border-primary bg-bg-secondary p-6">
			<h2 class="mb-2 text-lg font-semibold">Remove Security Key</h2>
			<p class="mb-4 text-sm text-text-secondary">
				Are you sure you want to remove this security key? You won't be able to use it for
				authentication anymore.
			</p>
			<div class="flex justify-end gap-2">
				<button
					onclick={() => (showDeleteKey = null)}
					class="rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
				>
					Cancel
				</button>
				<button
					onclick={deleteWebAuthnKey}
					class="rounded-lg bg-danger px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-danger-hover"
				>
					Remove
				</button>
			</div>
		</div>
	</div>
{/if}
