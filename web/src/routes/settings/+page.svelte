<script lang="ts">
	import { get, post, put, del } from '$lib/api';
	import { Plus, Pencil, Trash2, X, ChevronDown, ChevronRight } from 'lucide-svelte';

	interface OIDCProvider {
		id: string;
		provider_type: string;
		name: string;
		issuer_url: string;
		client_id: string;
		client_secret: string;
		scopes: string;
		groups_claim: string;
		required_group: string;
		enabled: boolean;
		created_at: string;
	}

	interface GeneralSettings {
		rate_limit_enabled: boolean;
		rate_limit_default_rpm: number;
		api_log_enabled: boolean;
		api_log_retention_days: number;
		admin_log_enabled: boolean;
		admin_log_retention_days: number;
		security_log_enabled: boolean;
		security_log_retention_days: number;
		audit_flush_interval: number;
		max_failed_logins: number;
		password_min_length: number;
		password_require_special: boolean;
		password_require_number: boolean;
		user_cache_ttl: number;
		enforce_2fa: boolean;
		password_expiry_days: number;
		admin_allowed_networks: string;
		gateway_allowed_networks: string;
	}

	// Section collapse state
	let generalOpen = $state(true);
	let securityOpen = $state(false);
	let gatewaySecurityOpen = $state(false);
	let oidcOpen = $state(false);

	// General settings state
	let generalSettings = $state<GeneralSettings>({
		rate_limit_enabled: true,
		rate_limit_default_rpm: 60,
		api_log_enabled: true,
		api_log_retention_days: 90,
		admin_log_enabled: true,
		admin_log_retention_days: 30,
		security_log_enabled: true,
		security_log_retention_days: 180,
		audit_flush_interval: 5,
		max_failed_logins: 5,
		password_min_length: 12,
		password_require_special: true,
		password_require_number: true,
		user_cache_ttl: 30,
		enforce_2fa: false,
		password_expiry_days: 0,
		admin_allowed_networks: '',
		gateway_allowed_networks: ''
	});
	let generalLoading = $state(true);
	let generalSaving = $state(false);
	let generalError = $state('');
	let generalSuccess = $state('');

	// OIDC state
	let providers = $state<OIDCProvider[]>([]);
	let loading = $state(true);
	let error = $state('');

	let showForm = $state(false);
	let editingProvider = $state<OIDCProvider | null>(null);
	let formProviderType = $state('google');
	let formCustomType = $state('');
	let formName = $state('');
	let formIssuer = $state('');
	let formClientId = $state('');
	let formClientSecret = $state('');
	let formScopes = $state('openid email profile');
	let formGroupsClaim = $state('groups');
	let formRequiredGroup = $state('');
	let formEnabled = $state(true);
	let formError = $state('');
	let formLoading = $state(false);

	let showDeleteConfirm = $state<OIDCProvider | null>(null);

	let appVersion = $state('');

	$effect(() => {
		loadGeneralSettings();
		loadProviders();
		loadVersion();
	});

	async function loadVersion() {
		try {
			const data = await get<{ version: string }>('/version');
			appVersion = data.version;
		} catch {
			appVersion = '';
		}
	}

	async function loadGeneralSettings() {
		generalLoading = true;
		generalError = '';
		try {
			const data = await get<GeneralSettings>('/settings/general');
			// Convert comma-separated to newline-separated for textarea display
			if (data.gateway_allowed_networks) {
				data.gateway_allowed_networks = data.gateway_allowed_networks.split(',').map(s => s.trim()).filter(Boolean).join('\n');
			}
			generalSettings = data;
		} catch (err) {
			generalError = err instanceof Error ? err.message : 'Failed to load settings';
		} finally {
			generalLoading = false;
		}
	}

	async function saveGeneralSettings() {
		generalSaving = true;
		generalError = '';
		generalSuccess = '';
		try {
			// Convert newline-separated gateway networks to comma-separated for storage
			const payload = { ...generalSettings };
			if (payload.gateway_allowed_networks) {
				payload.gateway_allowed_networks = payload.gateway_allowed_networks.split('\n').map(s => s.trim()).filter(Boolean).join(',');
			}
			const saved = await put<GeneralSettings>('/settings/general', payload);
			// Convert back to newline-separated for display
			if (saved.gateway_allowed_networks) {
				saved.gateway_allowed_networks = saved.gateway_allowed_networks.split(',').map(s => s.trim()).filter(Boolean).join('\n');
			}
			generalSettings = saved;
			generalSuccess = 'Settings saved successfully';
			setTimeout(() => (generalSuccess = ''), 3000);
		} catch (err) {
			generalError = err instanceof Error ? err.message : 'Failed to save settings';
		} finally {
			generalSaving = false;
		}
	}

	async function loadProviders() {
		loading = true;
		error = '';
		try {
			providers = await get<OIDCProvider[]>('/settings/oidc');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load providers';
		} finally {
			loading = false;
		}
	}

	const knownProviderTypes = ['google', 'github', 'azure', 'okta', 'auth0', 'keycloak', 'authentik'];

	function openCreate() {
		editingProvider = null;
		formProviderType = 'google';
		formCustomType = '';
		formName = '';
		formIssuer = '';
		formClientId = '';
		formClientSecret = '';
		formScopes = 'openid email profile';
		formGroupsClaim = 'groups';
		formRequiredGroup = '';
		formEnabled = true;
		formError = '';
		showForm = true;
	}

	function openEdit(provider: OIDCProvider) {
		editingProvider = provider;
		formProviderType = knownProviderTypes.includes(provider.provider_type) ? provider.provider_type : 'other';
		formCustomType = knownProviderTypes.includes(provider.provider_type) ? '' : provider.provider_type;
		formName = provider.name;
		formIssuer = provider.issuer_url;
		formClientId = provider.client_id;
		formClientSecret = '';
		formScopes = provider.scopes;
		formGroupsClaim = provider.groups_claim || 'groups';
		formRequiredGroup = provider.required_group || '';
		formEnabled = provider.enabled;
		formError = '';
		showForm = true;
	}

	function closeForm() {
		showForm = false;
		editingProvider = null;
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();

		const resolvedType = formProviderType === 'other' ? formCustomType.trim().toLowerCase() : formProviderType;

		if (!resolvedType || !formName || !formIssuer || !formClientId) {
			formError = 'Provider Type, Display Name, Issuer URL, and Client ID are required';
			return;
		}

		if (formProviderType === 'other' && !/^[a-z0-9_-]+$/.test(resolvedType)) {
			formError = 'Custom provider identifier must contain only lowercase letters, numbers, hyphens, and underscores';
			return;
		}

		if (!editingProvider && !formClientSecret) {
			formError = 'Client Secret is required for new providers';
			return;
		}

		formLoading = true;
		formError = '';

		try {
			const body: Record<string, unknown> = {
				provider_type: resolvedType,
				name: formName,
				issuer_url: formIssuer,
				client_id: formClientId,
				scopes: formScopes,
				groups_claim: formGroupsClaim,
				required_group: formRequiredGroup,
				enabled: formEnabled
			};
			if (formClientSecret) body.client_secret = formClientSecret;

			if (editingProvider) {
				await put(`/settings/oidc/${editingProvider.id}`, body);
			}

			if (!editingProvider) {
				await post('/settings/oidc', body);
			}

			closeForm();
			await loadProviders();
		} catch (err) {
			formError = err instanceof Error ? err.message : 'Failed to save provider';
		} finally {
			formLoading = false;
		}
	}

	async function confirmDelete() {
		if (!showDeleteConfirm) return;
		try {
			await del(`/settings/oidc/${showDeleteConfirm.id}`);
			showDeleteConfirm = null;
			await loadProviders();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete provider';
			showDeleteConfirm = null;
		}
	}
</script>

<div>
	<div class="mb-6">
		<h1 class="text-2xl font-bold">Settings</h1>
		<p class="mt-1 text-sm text-text-muted">Manage system settings</p>
	</div>

	<!-- General + Security Settings -->
	<div class="mb-8 rounded-xl border border-border-primary bg-bg-secondary p-5">
		{#if generalLoading}
			<div class="flex items-center justify-center py-8 text-text-muted">Loading...</div>
		{:else}
			{#if generalError}
				<div class="mb-4 rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger">
					{generalError}
				</div>
			{/if}

			{#if generalSuccess}
				<div class="mb-4 rounded-lg border border-success/30 bg-success/10 p-3 text-sm text-success">
					{generalSuccess}
				</div>
			{/if}

			<!-- General sub-section -->
			<div class="mb-2">
				<button
					onclick={() => (generalOpen = !generalOpen)}
					class="flex w-full items-center gap-2 text-lg font-semibold hover:text-accent transition-colors"
				>
					{#if generalOpen}
						<ChevronDown class="h-5 w-5" />
					{:else}
						<ChevronRight class="h-5 w-5" />
					{/if}
					General
				</button>
			</div>
			{#if generalOpen}
				<div class="space-y-4 pb-4">
					<div class="flex items-center gap-2">
						<input
							id="rate-limit-enabled"
							type="checkbox"
							bind:checked={generalSettings.rate_limit_enabled}
							class="h-4 w-4 rounded border-border-primary accent-accent"
						/>
						<label for="rate-limit-enabled" class="text-sm text-text-secondary">
							Rate limiting enabled
						</label>
					</div>

					<div>
						<label for="rate-limit-rpm" class="mb-1.5 block text-sm text-text-secondary">
							Default RPM (requests per minute)
						</label>
						<input
							id="rate-limit-rpm"
							type="number"
							min="1"
							bind:value={generalSettings.rate_limit_default_rpm}
							class="w-full max-w-xs rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<!-- Per-type log retention -->
					<div class="space-y-3 rounded-lg border border-border-primary bg-bg-primary p-4">
						<p class="text-sm font-medium text-text-secondary">Audit log retention</p>

						<div class="flex items-center gap-4">
							<div class="flex items-center gap-2 w-48">
								<input
									id="api-log-enabled"
									type="checkbox"
									bind:checked={generalSettings.api_log_enabled}
									class="h-4 w-4 rounded border-border-primary accent-accent"
								/>
								<label for="api-log-enabled" class="text-sm text-text-secondary">
									API logs (LLM inference)
								</label>
							</div>
							<div class="flex items-center gap-2">
								<label for="api-log-retention" class="text-sm text-text-muted">Retention:</label>
								<input
									id="api-log-retention"
									type="number"
									min="0"
									bind:value={generalSettings.api_log_retention_days}
									disabled={!generalSettings.api_log_enabled}
									class="w-20 rounded-lg border border-border-primary bg-bg-primary px-3 py-1.5 text-sm outline-none focus:border-accent disabled:opacity-40"
								/>
								<span class="text-xs text-text-muted">days (0 = keep forever)</span>
							</div>
						</div>

						<div class="flex items-center gap-4">
							<div class="flex items-center gap-2 w-48">
								<input
									id="admin-log-enabled"
									type="checkbox"
									bind:checked={generalSettings.admin_log_enabled}
									class="h-4 w-4 rounded border-border-primary accent-accent"
								/>
								<label for="admin-log-enabled" class="text-sm text-text-secondary">
									Admin logs
								</label>
							</div>
							<div class="flex items-center gap-2">
								<label for="admin-log-retention" class="text-sm text-text-muted">Retention:</label>
								<input
									id="admin-log-retention"
									type="number"
									min="0"
									bind:value={generalSettings.admin_log_retention_days}
									disabled={!generalSettings.admin_log_enabled}
									class="w-20 rounded-lg border border-border-primary bg-bg-primary px-3 py-1.5 text-sm outline-none focus:border-accent disabled:opacity-40"
								/>
								<span class="text-xs text-text-muted">days (0 = keep forever)</span>
							</div>
						</div>

						<div class="flex items-center gap-4">
							<div class="flex items-center gap-2 w-48">
								<input
									id="security-log-enabled"
									type="checkbox"
									bind:checked={generalSettings.security_log_enabled}
									class="h-4 w-4 rounded border-border-primary accent-accent"
								/>
								<label for="security-log-enabled" class="text-sm text-text-secondary">
									Security logs
								</label>
							</div>
							<div class="flex items-center gap-2">
								<label for="security-log-retention" class="text-sm text-text-muted">Retention:</label>
								<input
									id="security-log-retention"
									type="number"
									min="0"
									bind:value={generalSettings.security_log_retention_days}
									disabled={!generalSettings.security_log_enabled}
									class="w-20 rounded-lg border border-border-primary bg-bg-primary px-3 py-1.5 text-sm outline-none focus:border-accent disabled:opacity-40"
								/>
								<span class="text-xs text-text-muted">days (0 = keep forever)</span>
							</div>
						</div>
					</div>

					<div>
						<label for="audit-flush-interval" class="mb-1.5 block text-sm text-text-secondary">
							Audit log flush interval (seconds)
						</label>
						<input
							id="audit-flush-interval"
							type="number"
							min="1"
							bind:value={generalSettings.audit_flush_interval}
							class="w-full max-w-xs rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
						<p class="mt-1 text-xs text-text-muted">
							How often buffered logs are written to the database. Lower values show logs sooner but increase DB writes.
						</p>
					</div>

					<div>
						<label for="user-cache-ttl" class="mb-1.5 block text-sm text-text-secondary">
							User cache TTL (seconds)
						</label>
						<input
							id="user-cache-ttl"
							type="number"
							min="5"
							bind:value={generalSettings.user_cache_ttl}
							class="w-full max-w-xs rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>
				</div>
			{/if}

			<!-- Divider between General and Security -->
			<div class="border-t border-border-primary my-4"></div>

			<!-- Security sub-section -->
			<div class="mb-2">
				<button
					onclick={() => (securityOpen = !securityOpen)}
					class="flex w-full items-center gap-2 text-lg font-semibold hover:text-accent transition-colors"
				>
					{#if securityOpen}
						<ChevronDown class="h-5 w-5" />
					{:else}
						<ChevronRight class="h-5 w-5" />
					{/if}
					Security
				</button>
			</div>
			{#if securityOpen}
				<div class="space-y-4 pb-4">
					<div>
						<label for="max-failed-logins" class="mb-1.5 block text-sm text-text-secondary">
							Max failed login attempts
						</label>
						<input
							id="max-failed-logins"
							type="number"
							min="1"
							bind:value={generalSettings.max_failed_logins}
							class="w-full max-w-xs rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<div>
						<label for="password-min-length" class="mb-1.5 block text-sm text-text-secondary">
							Minimum password length
						</label>
						<input
							id="password-min-length"
							type="number"
							min="8"
							bind:value={generalSettings.password_min_length}
							class="w-full max-w-xs rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<div class="flex items-center gap-2">
						<input
							id="password-require-special"
							type="checkbox"
							bind:checked={generalSettings.password_require_special}
							class="h-4 w-4 rounded border-border-primary accent-accent"
						/>
						<label for="password-require-special" class="text-sm text-text-secondary">
							Require special character
						</label>
					</div>

					<div class="flex items-center gap-2">
						<input
							id="password-require-number"
							type="checkbox"
							bind:checked={generalSettings.password_require_number}
							class="h-4 w-4 rounded border-border-primary accent-accent"
						/>
						<label for="password-require-number" class="text-sm text-text-secondary">
							Require number
						</label>
					</div>

					<div class="flex items-center gap-2">
						<input
							id="enforce-2fa"
							type="checkbox"
							bind:checked={generalSettings.enforce_2fa}
							class="h-4 w-4 rounded border-border-primary accent-accent"
						/>
						<label for="enforce-2fa" class="text-sm text-text-secondary">
							Enforce two-factor authentication for all users
						</label>
					</div>

					<div>
						<label for="password-expiry" class="mb-1.5 block text-sm text-text-secondary">
							Password expiry (days, 0 = disabled)
						</label>
						<input
							id="password-expiry"
							type="number"
							min="0"
							bind:value={generalSettings.password_expiry_days}
							class="w-full max-w-xs rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<div>
						<label for="admin-allowed-networks" class="mb-1.5 block text-sm text-text-secondary">
							Admin allowed networks
						</label>
						<input
							id="admin-allowed-networks"
							type="text"
							bind:value={generalSettings.admin_allowed_networks}
							placeholder="e.g. 127.0.0.1,::1,10.0.0.0/24"
							class="w-full max-w-md rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
						<p class="mt-1 text-xs text-text-muted">
							Comma-separated IPs or CIDRs. Empty = unrestricted (all IPs allowed).
						</p>
					</div>

					</div>
			{/if}

			<!-- Divider between Security and Gateway Security -->
			<div class="border-t border-border-primary my-4"></div>

			<!-- Gateway Security sub-section -->
			<div class="mb-2">
				<button
					onclick={() => (gatewaySecurityOpen = !gatewaySecurityOpen)}
					class="flex w-full items-center gap-2 text-lg font-semibold hover:text-accent transition-colors"
				>
					{#if gatewaySecurityOpen}
						<ChevronDown class="h-5 w-5" />
					{:else}
						<ChevronRight class="h-5 w-5" />
					{/if}
					Gateway Security
				</button>
			</div>
			{#if gatewaySecurityOpen}
				<div class="space-y-4 pb-4">
					<div>
						<label for="gateway-allowed-networks" class="mb-1.5 block text-sm text-text-secondary">
							Gateway allowed networks
						</label>
						<textarea
							id="gateway-allowed-networks"
							bind:value={generalSettings.gateway_allowed_networks}
							placeholder={"e.g.\n10.0.0.0/24\n192.168.1.100\n::1"}
							rows="4"
							class="w-full max-w-md rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent font-mono"
						></textarea>
						<p class="mt-1 text-xs text-text-muted">
							One IP or CIDR subnet per line. Empty = unrestricted (all IPs allowed).
						</p>
					</div>
				</div>
			{/if}

			<!-- Save button -->
			<div class="pt-2">
				<button
					onclick={saveGeneralSettings}
					disabled={generalSaving}
					class="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-50"
				>
					{generalSaving ? 'Saving...' : 'Save Settings'}
				</button>
			</div>
		{/if}
	</div>

	<!-- OIDC Providers -->
	<div class="mb-4 flex items-center justify-between">
		<button
			onclick={() => (oidcOpen = !oidcOpen)}
			class="flex items-center gap-2 text-lg font-semibold hover:text-accent transition-colors"
		>
			{#if oidcOpen}
				<ChevronDown class="h-5 w-5" />
			{:else}
				<ChevronRight class="h-5 w-5" />
			{/if}
			OIDC Providers
		</button>
		{#if oidcOpen}
			<button
				onclick={openCreate}
				class="flex items-center gap-2 rounded-lg bg-accent px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
			>
				<Plus class="h-4 w-4" />
				Add Provider
			</button>
		{/if}
	</div>

	{#if oidcOpen}
		{#if error}
			<div class="mb-4 rounded-lg border border-danger/30 bg-danger/10 p-4 text-danger">{error}</div>
		{/if}

		{#if loading}
			<div class="flex items-center justify-center py-20 text-text-muted">Loading...</div>
		{/if}

		{#if !loading && providers.length === 0}
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-8 text-center">
				<p class="text-text-muted">No OIDC providers configured</p>
				<p class="mt-1 text-sm text-text-muted">
					Add a provider to enable single sign-on for your users
				</p>
			</div>
		{/if}

		{#if !loading && providers.length > 0}
			<div class="space-y-3">
				{#each providers as provider}
					<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
						<div class="flex items-start justify-between">
							<div>
								<div class="flex items-center gap-3">
									<h3 class="font-semibold">{provider.name}</h3>
									<span
										class="rounded-full px-2 py-0.5 text-xs font-medium
										{provider.enabled
											? 'bg-success/10 text-success'
											: 'bg-bg-tertiary text-text-muted'}"
									>
										{provider.enabled ? 'Enabled' : 'Disabled'}
									</span>
								</div>
								<div class="mt-2 space-y-1 text-sm text-text-secondary">
									<div>
										<span class="text-text-muted">Type:</span>
										{provider.provider_type}
									</div>
									<div>
										<span class="text-text-muted">Issuer:</span>
										{provider.issuer_url}
									</div>
									<div>
										<span class="text-text-muted">Client ID:</span>
										{provider.client_id}
									</div>
									<div>
										<span class="text-text-muted">Scopes:</span>
										{provider.scopes}
									</div>
									<div>
										<span class="text-text-muted">Groups Claim:</span>
										{provider.groups_claim || 'groups'}
									</div>
									{#if provider.required_group}
										<div>
											<span class="text-text-muted">Required Group:</span>
											{provider.required_group}
										</div>
									{/if}
								</div>
							</div>
							<div class="flex items-center gap-1">
								<button
									onclick={() => openEdit(provider)}
									class="rounded p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-text-primary"
									aria-label="Edit provider"
								>
									<Pencil class="h-4 w-4" />
								</button>
								<button
									onclick={() => (showDeleteConfirm = provider)}
									class="rounded p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-danger"
									aria-label="Delete provider"
								>
									<Trash2 class="h-4 w-4" />
								</button>
							</div>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	{/if}

	<!-- Provider Form Modal -->
	{#if showForm}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
			<div
				class="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-xl border border-border-primary bg-bg-secondary p-6"
			>
				<div class="mb-4 flex items-center justify-between">
					<h2 class="text-lg font-semibold">
						{editingProvider ? 'Edit Provider' : 'Add OIDC Provider'}
					</h2>
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
						<label for="oidc-provider-type" class="mb-1.5 block text-sm text-text-secondary">Provider Type</label>
						<select
							id="oidc-provider-type"
							bind:value={formProviderType}
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						>
							<option value="google">Google</option>
							<option value="github">GitHub</option>
							<option value="azure">Azure AD</option>
							<option value="okta">Okta</option>
							<option value="auth0">Auth0</option>
							<option value="keycloak">Keycloak</option>
							<option value="authentik">Authentik</option>
							<option value="other">Other</option>
						</select>
					</div>

					{#if formProviderType === 'other'}
						<div>
							<label for="oidc-custom-type" class="mb-1.5 block text-sm text-text-secondary">Custom Provider Identifier</label>
							<input
								id="oidc-custom-type"
								type="text"
								bind:value={formCustomType}
								placeholder="e.g. authelia, zitadel"
								pattern="[a-z0-9_-]+"
								class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
							/>
							<p class="mt-1 text-xs text-text-muted">
								Lowercase letters, numbers, hyphens, and underscores only.
							</p>
						</div>
					{/if}

					<div>
						<label for="oidc-name" class="mb-1.5 block text-sm text-text-secondary">Display Name</label>
						<input
							id="oidc-name"
							type="text"
							bind:value={formName}
							placeholder="e.g. Company Google SSO"
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<div>
						<label for="oidc-issuer" class="mb-1.5 block text-sm text-text-secondary">
							Issuer URL
						</label>
						<input
							id="oidc-issuer"
							type="url"
							bind:value={formIssuer}
							placeholder="https://accounts.google.com"
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<div>
						<label for="oidc-client-id" class="mb-1.5 block text-sm text-text-secondary">
							Client ID
						</label>
						<input
							id="oidc-client-id"
							type="text"
							bind:value={formClientId}
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<div>
						<label for="oidc-client-secret" class="mb-1.5 block text-sm text-text-secondary">
							Client Secret {editingProvider ? '(leave blank to keep current)' : ''}
						</label>
						<input
							id="oidc-client-secret"
							type="password"
							bind:value={formClientSecret}
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
					</div>

					<div>
						<label for="oidc-scopes" class="mb-1.5 block text-sm text-text-secondary">
							Scopes
						</label>
						<input
							id="oidc-scopes"
							type="text"
							bind:value={formScopes}
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
						<p class="mt-1 text-xs text-text-muted">Space-separated list of scopes</p>
					</div>

					<div>
						<label for="oidc-groups-claim" class="mb-1.5 block text-sm text-text-secondary">
							Groups Claim
						</label>
						<input
							id="oidc-groups-claim"
							type="text"
							bind:value={formGroupsClaim}
							placeholder="groups"
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
						<p class="mt-1 text-xs text-text-muted">JWT claim name containing user groups (default: groups)</p>
					</div>

					<div>
						<label for="oidc-required-group" class="mb-1.5 block text-sm text-text-secondary">
							Required Group
						</label>
						<input
							id="oidc-required-group"
							type="text"
							bind:value={formRequiredGroup}
							placeholder="e.g. LM Gate"
							class="w-full rounded-lg border border-border-primary bg-bg-primary px-3 py-2 text-sm outline-none focus:border-accent"
						/>
						<p class="mt-1 text-xs text-text-muted">
							If set, users must belong to this group (from the identity provider) to log in. Leave empty to allow all authenticated users.
						</p>
						<p class="mt-1 text-xs text-text-muted">
							If you are looking to gate by a Microsoft or Google group, see <a href="https://github.com/hkdb/lmgate/blob/main/docs/MGGROUPS.md" target="_blank" rel="noopener noreferrer" class="text-accent hover:underline">this guide</a>.
						</p>
					</div>

					<div class="flex items-center gap-2">
						<input
							id="oidc-enabled"
							type="checkbox"
							bind:checked={formEnabled}
							class="h-4 w-4 rounded border-border-primary accent-accent"
						/>
						<label for="oidc-enabled" class="text-sm text-text-secondary">Enabled</label>
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

	<!-- Delete Confirmation -->
	{#if showDeleteConfirm}
		<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
			<div class="w-full max-w-sm rounded-xl border border-border-primary bg-bg-secondary p-6">
				<h2 class="mb-2 text-lg font-semibold">Delete Provider</h2>
				<p class="mb-4 text-sm text-text-secondary">
					Are you sure you want to delete <strong>{showDeleteConfirm.name}</strong>? Users who sign
					in with this provider will no longer be able to authenticate.
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

	{#if appVersion}
		<div class="mt-8 text-center text-xs text-text-muted">LM Gate {appVersion}</div>
	{/if}
</div>
