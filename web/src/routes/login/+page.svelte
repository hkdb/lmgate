<script lang="ts">
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { post, getCsrfToken, initCsrf } from '$lib/api';
	import { auth, type AuthUser } from '$lib/stores.svelte';
	import { LogIn, KeyRound, Shield } from 'lucide-svelte';
	import {
		startAuthentication,
		type PublicKeyCredentialRequestOptionsJSON
	} from '@simplewebauthn/browser';

	interface LoginResponse {
		user?: AuthUser;
		requires_2fa?: boolean;
		twofa_token?: string;
		methods?: string[];
	}

	interface TwoFALoginResponse {
		user: AuthUser;
	}

	interface OIDCProvider {
		id: string;
		name: string;
		auth_url: string;
	}

	interface WebAuthnBeginResponse {
		options: { publicKey: PublicKeyCredentialRequestOptionsJSON };
		twofa_token: string;
	}

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);
	let providers = $state<OIDCProvider[]>([]);

	// 2FA state
	let twofaRequired = $state(false);
	let twofaToken = $state('');
	let twofaMethods = $state<string[]>([]);
	let twofaMode = $state<'totp' | 'webauthn' | 'recovery'>('totp');
	let totpCode = $state('');
	let recoveryCode = $state('');

	$effect(() => {
		if (!auth.checked) return;
		if (!auth.isAuthenticated) {
			loadProviders();
			initCsrf();
		}
	});

	async function loadProviders() {
		try {
			const res = await fetch('/admin/api/auth/providers', { credentials: 'same-origin' });
			if (res.ok) {
				providers = await res.json();
			}
		} catch {
			// No providers available
		}
	}

	async function handleLogin(e: Event) {
		e.preventDefault();
		if (!email || !password) {
			error = 'Email and password are required';
			return;
		}

		loading = true;
		error = '';
		try {
			const res = await post<LoginResponse>('/login', { email, password });
			if (res.requires_2fa) {
				twofaRequired = true;
				twofaToken = res.twofa_token!;
				twofaMethods = res.methods!;
				twofaMode = res.methods!.includes('totp') ? 'totp' : 'webauthn';
				return;
			}
			auth.setUser(res.user!);
			const destination = res.user!.force_password_change
				? `${base}/change-password`
				: res.user!.role === 'admin'
					? `${base}/`
					: `${base}/models`;
			await goto(destination);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Login failed';
		} finally {
			loading = false;
		}
	}

	async function handleTOTPSubmit(e: Event) {
		e.preventDefault();
		if (!totpCode) {
			error = 'Enter your 6-digit code';
			return;
		}
		loading = true;
		error = '';
		try {
			const res = await post<TwoFALoginResponse>('/2fa/totp/login', {
				twofa_token: twofaToken,
				code: totpCode
			});
			auth.setUser(res.user);
			const destination = res.user.force_password_change
				? `${base}/change-password`
				: res.user.role === 'admin'
					? `${base}/`
					: `${base}/models`;
			await goto(destination);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Invalid code';
		} finally {
			loading = false;
		}
	}

	async function handleWebAuthn() {
		loading = true;
		error = '';
		try {
			const beginRes = await post<WebAuthnBeginResponse>('/2fa/webauthn/login/begin', {
				twofa_token: twofaToken
			});

			const assertion = await startAuthentication({
				optionsJSON: beginRes.options.publicKey
			});

			const res = await fetch('/admin/api/2fa/webauthn/login/finish', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'X-TwoFA-Token': twofaToken,
					'X-CSRF-Token': getCsrfToken() ?? ''
				},
				body: JSON.stringify(assertion),
				credentials: 'same-origin'
			});

			if (!res.ok) {
				const body = await res.json();
				throw new Error(body.error || 'Authentication failed');
			}

			const data: TwoFALoginResponse = await res.json();
			auth.setUser(data.user);
			const destination = data.user.force_password_change
				? `${base}/change-password`
				: data.user.role === 'admin'
					? `${base}/`
					: `${base}/models`;
			await goto(destination);
		} catch (err) {
			error = err instanceof Error ? err.message : 'WebAuthn authentication failed';
		} finally {
			loading = false;
		}
	}

	async function handleRecoverySubmit(e: Event) {
		e.preventDefault();
		if (!recoveryCode) {
			error = 'Enter your recovery code';
			return;
		}
		loading = true;
		error = '';
		try {
			const res = await post<TwoFALoginResponse>('/2fa/recovery/login', {
				twofa_token: twofaToken,
				code: recoveryCode
			});
			auth.setUser(res.user);
			const destination = res.user.force_password_change
				? `${base}/change-password`
				: res.user.role === 'admin'
					? `${base}/`
					: `${base}/models`;
			await goto(destination);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Invalid recovery code';
		} finally {
			loading = false;
		}
	}

	function handleOIDC(provider: OIDCProvider) {
		const url = provider.auth_url;
		if (!url.startsWith('/') && !url.startsWith('https://')) {
			error = 'Invalid authentication provider URL';
			return;
		}
		window.location.href = url;
	}

	function backToLogin() {
		twofaRequired = false;
		twofaToken = '';
		twofaMethods = [];
		totpCode = '';
		recoveryCode = '';
		error = '';
		password = '';
	}
</script>

<div class="flex min-h-screen items-center justify-center px-4">
	<div class="w-full max-w-sm">
		<div class="mb-8 text-center">
			<h1 class="text-2xl font-bold">LM Gate</h1>
			<p class="mt-1 text-sm text-text-muted">Admin Dashboard</p>
		</div>

		<div class="rounded-xl border border-border-primary bg-bg-secondary p-6">
			{#if error}
				<div class="mb-4 rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger">
					{error}
				</div>
			{/if}

			{#if !twofaRequired}
				<!-- Password login form -->
				<form onsubmit={handleLogin} class="space-y-4">
					<div>
						<label for="email" class="mb-1.5 block text-sm text-text-secondary">Email</label>
						<input
							id="email"
							type="email"
							bind:value={email}
							placeholder="admin@example.com"
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none transition-colors placeholder:text-text-muted focus:border-accent"
							autocomplete="email"
						/>
					</div>

					<div>
						<label for="password" class="mb-1.5 block text-sm text-text-secondary"
							>Password</label
						>
						<input
							id="password"
							type="password"
							bind:value={password}
							placeholder="••••••••"
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none transition-colors placeholder:text-text-muted focus:border-accent"
							autocomplete="current-password"
						/>
					</div>

					<button
						type="submit"
						disabled={loading}
						class="flex w-full items-center justify-center gap-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
					>
						<LogIn class="h-4 w-4" />
						{loading ? 'Signing in...' : 'Sign In'}
					</button>
				</form>

				{#if providers.length > 0}
					<div class="mt-5">
						<div class="relative mb-4">
							<div class="absolute inset-0 flex items-center">
								<div class="w-full border-t border-border-primary"></div>
							</div>
							<div class="relative flex justify-center text-xs">
								<span class="bg-bg-secondary px-2 text-text-muted">or continue with</span>
							</div>
						</div>

						<div class="space-y-2">
							{#each providers as provider}
								<button
									onclick={() => handleOIDC(provider)}
									class="flex w-full items-center justify-center gap-2 rounded-lg border border-border-primary px-4 py-2 text-sm transition-colors hover:bg-bg-tertiary"
								>
									{provider.name}
								</button>
							{/each}
						</div>
					</div>
				{/if}
			{:else}
				<!-- 2FA challenge -->
				<div class="space-y-4">
					<div class="text-center">
						<Shield class="mx-auto mb-2 h-8 w-8 text-accent" />
						<h2 class="text-lg font-semibold">Two-Factor Authentication</h2>
						<p class="mt-1 text-sm text-text-muted">Verify your identity to continue</p>
					</div>

					{#if twofaMethods.length > 1 || twofaMode === 'recovery'}
						<div class="flex gap-1 rounded-lg border border-border-primary bg-bg-primary p-1">
							{#if twofaMethods.includes('totp')}
								<button
									onclick={() => {
										twofaMode = 'totp';
										error = '';
									}}
									class="flex-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors {twofaMode ===
									'totp'
										? 'bg-accent text-white'
										: 'text-text-secondary hover:text-text-primary'}"
								>
									Authenticator
								</button>
							{/if}
							{#if twofaMethods.includes('webauthn')}
								<button
									onclick={() => {
										twofaMode = 'webauthn';
										error = '';
									}}
									class="flex-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors {twofaMode ===
									'webauthn'
										? 'bg-accent text-white'
										: 'text-text-secondary hover:text-text-primary'}"
								>
									Security Key
								</button>
							{/if}
							<button
								onclick={() => {
									twofaMode = 'recovery';
									error = '';
								}}
								class="flex-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors {twofaMode ===
								'recovery'
									? 'bg-accent text-white'
									: 'text-text-secondary hover:text-text-primary'}"
							>
								Recovery
							</button>
						</div>
					{/if}

					{#if twofaMode === 'totp'}
						<form onsubmit={handleTOTPSubmit} class="space-y-4">
							<div>
								<label for="totp-code" class="mb-1.5 block text-sm text-text-secondary"
									>Authentication Code</label
								>
								<input
									id="totp-code"
									type="text"
									bind:value={totpCode}
									placeholder="000000"
									maxlength="6"
									pattern="[0-9]*"
									inputmode="numeric"
									autocomplete="one-time-code"
									class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-center text-lg tracking-widest outline-none transition-colors placeholder:text-text-muted focus:border-accent"
								/>
							</div>
							<button
								type="submit"
								disabled={loading}
								class="flex w-full items-center justify-center gap-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
							>
								{loading ? 'Verifying...' : 'Verify'}
							</button>
						</form>
					{:else if twofaMode === 'webauthn'}
						<div class="space-y-4">
							<p class="text-center text-sm text-text-secondary">
								Use your security key to authenticate.
							</p>
							<button
								onclick={handleWebAuthn}
								disabled={loading}
								class="flex w-full items-center justify-center gap-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
							>
								<KeyRound class="h-4 w-4" />
								{loading ? 'Waiting for key...' : 'Use Security Key'}
							</button>
						</div>
					{:else}
						<form onsubmit={handleRecoverySubmit} class="space-y-4">
							<div>
								<label for="recovery-code" class="mb-1.5 block text-sm text-text-secondary"
									>Recovery Code</label
								>
								<input
									id="recovery-code"
									type="text"
									bind:value={recoveryCode}
									placeholder="Enter recovery code"
									class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none transition-colors placeholder:text-text-muted focus:border-accent"
								/>
							</div>
							<button
								type="submit"
								disabled={loading}
								class="flex w-full items-center justify-center gap-2 rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
							>
								{loading ? 'Verifying...' : 'Verify'}
							</button>
						</form>
					{/if}

					{#if twofaMode !== 'recovery'}
						<button
							onclick={() => {
								twofaMode = 'recovery';
								error = '';
							}}
							class="w-full text-center text-xs text-text-muted hover:text-text-secondary"
						>
							Use a recovery code instead
						</button>
					{/if}

					<button
						onclick={backToLogin}
						class="w-full text-center text-xs text-text-muted hover:text-text-secondary"
					>
						Back to login
					</button>
				</div>
			{/if}
		</div>
	</div>
</div>
