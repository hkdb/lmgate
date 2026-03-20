<script lang="ts">
	import { untrack } from 'svelte';
	import { get, del } from '$lib/api';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { ChevronLeft, ChevronRight, Search, Filter, Trash2, Download, RefreshCw, Radio } from 'lucide-svelte';
	import ConfirmDialog from '$lib/components/confirm-dialog.svelte';

	interface AuditLog {
		id: string;
		timestamp: string;
		user_email: string;
		action: string;
		resource: string;
		detail: string;
		ip_address: string;
		x_real_ip: string;
		x_forwarded_for: string;
		status: string;
		streaming: boolean;
		prompt_tokens: number;
		completion_tokens: number;
		latency_ms: number;
		gateway_overhead_us: number;
		upstream_ttfb_ms: number;
		model: string;
		log_type: string;
	}

	interface LogsResponse {
		logs: AuditLog[];
		total: number;
		page: number;
		per_page: number;
	}

	// Initialize from URL params
	const urlParams = $page.url.searchParams;

	let logs = $state<AuditLog[]>([]);
	let total = $state(0);
	let currentPage = $state(Number(urlParams.get('page')) || 1);
	let perPage = $state(Number(urlParams.get('per_page')) || 25);
	let loading = $state(true);
	let error = $state('');
	let refreshing = $state(false);

	let searchQuery = $state(urlParams.get('q') ?? '');
	let filterAction = $state(urlParams.get('action') ?? '');
	let filterStatus = $state(urlParams.get('status') ?? '');
	let filterLogType = $state(urlParams.get('log_type') ?? '');
	let showDeleteConfirm = $state(false);
	let showExportMenu = $state(false);

	let liveMode = $state(false);
	let eventSource = $state<EventSource | null>(null);
	let maxId = $state(0);
	let newLogIds = $state<Set<string>>(new Set());
	let debounceTimer = $state<ReturnType<typeof setTimeout> | null>(null);

	function updateURL() {
		const params = new URLSearchParams();
		if (currentPage > 1) params.set('page', String(currentPage));
		if (perPage !== 25) params.set('per_page', String(perPage));
		if (searchQuery) params.set('q', searchQuery);
		if (filterAction) params.set('action', filterAction);
		if (filterStatus) params.set('status', filterStatus);
		if (filterLogType) params.set('log_type', filterLogType);
		const qs = params.toString();
		goto(qs ? `?${qs}` : '?', { replaceState: true, noScroll: true, keepFocus: true });
	}

	function toggleLive() {
		if (liveMode) {
			liveMode = false;
			if (eventSource) {
				eventSource.close();
				eventSource = null;
			}
			return;
		}
		liveMode = true;
		const es = new EventSource('/admin/api/logs/stream');
		es.onmessage = () => {
			if (debounceTimer) clearTimeout(debounceTimer);
			debounceTimer = setTimeout(() => {
				fetchNewLogs();
				debounceTimer = null;
			}, 200);
		};
		es.onerror = () => {
			if (es.readyState === EventSource.CLOSED) {
				liveMode = false;
				eventSource = null;
			}
		};
		eventSource = es;
	}

	async function handleRefresh() {
		refreshing = true;
		if (maxId > 0 && currentPage === 1) {
			await fetchNewLogs();
			refreshing = false;
			return;
		}
		await loadLogs();
		refreshing = false;
	}

	$effect(() => {
		untrack(() => loadLogs());
		return () => {
			if (eventSource) {
				eventSource.close();
				eventSource = null;
			}
		};
	});

	async function loadLogs() {
		if (debounceTimer) {
			clearTimeout(debounceTimer);
			debounceTimer = null;
		}
		loading = true;
		error = '';

		const params = new URLSearchParams();
		params.set('page', String(currentPage));
		params.set('per_page', String(perPage));
		if (searchQuery) params.set('q', searchQuery);
		if (filterAction) params.set('action', filterAction);
		if (filterStatus) params.set('status', filterStatus);
		if (filterLogType) params.set('log_type', filterLogType);

		try {
			const res = await get<LogsResponse>(`/logs?${params}`);
			logs = res.logs;
			total = res.total;
			maxId = res.logs.length > 0 ? Number(res.logs[0].id) : 0;
			newLogIds = new Set();
			updateURL();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load logs';
		} finally {
			loading = false;
		}
	}

	let filtersActive = $derived(searchQuery !== '' || filterAction !== '' || filterStatus !== '' || filterLogType !== '');

	async function fetchNewLogs() {
		if (maxId === 0 || currentPage !== 1) {
			await loadLogs();
			return;
		}

		try {
			const params = new URLSearchParams();
			params.set('page', '1');
			params.set('per_page', String(perPage));
			params.set('after_id', String(maxId));
			if (searchQuery) params.set('q', searchQuery);
			if (filterAction) params.set('action', filterAction);
			if (filterStatus) params.set('status', filterStatus);
			if (filterLogType) params.set('log_type', filterLogType);

			const res = await get<LogsResponse>(`/logs?${params}`);
			if (res.logs.length > 0) {
				maxId = Number(res.logs[0].id);
			}
			const filtered = res.logs.filter((l) => !l.resource.startsWith('/admin/api/logs'));
			if (filtered.length === 0) return;

			const incomingIds = new Set(filtered.map((l) => l.id));
			logs = [...filtered, ...logs].slice(0, perPage);
			total = total + filtered.length;
			newLogIds = incomingIds;

			setTimeout(() => {
				newLogIds = new Set();
			}, 400);
		} catch {
			await loadLogs();
		}
	}

	let totalPages = $derived(Math.ceil(total / perPage));

	function goToPage(page: number) {
		if (page < 1 || page > totalPages) return;
		currentPage = page;
		loadLogs();
	}

	function handleSearch(e: Event) {
		e.preventDefault();
		currentPage = 1;
		loadLogs();
	}

	async function deleteAllLogs() {
		try {
			await del('/logs?confirm=true');
			currentPage = 1;
			maxId = 0;
			await loadLogs();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete logs';
		}
	}

	function exportLogs(format: 'csv' | 'log') {
		const params = new URLSearchParams();
		params.set('format', format);
		if (searchQuery) params.set('q', searchQuery);
		if (filterAction) params.set('action', filterAction);
		if (filterStatus) params.set('status', filterStatus);
		if (filterLogType) params.set('log_type', filterLogType);
		window.location.href = `/admin/api/logs/export?${params}`;
		showExportMenu = false;
	}

	function statusBadgeClass(status: string): string {
		switch (status) {
			case 'success':
				return 'bg-success/10 text-success';
			case 'error':
				return 'bg-danger/10 text-danger';
			case 'warning':
				return 'bg-warning/10 text-warning';
			default:
				return 'bg-bg-tertiary text-text-secondary';
		}
	}

	function logTypeBadgeClass(logType: string): string {
		switch (logType) {
			case 'api':
				return 'bg-accent/10 text-accent';
			case 'admin':
				return 'bg-amber-500/10 text-amber-400';
			case 'security':
				return 'bg-danger/10 text-danger';
			default:
				return 'bg-bg-tertiary text-text-secondary';
		}
	}

	function logTypeLabel(logType: string): string {
		switch (logType) {
			case 'api':
				return 'API';
			case 'admin':
				return 'Admin';
			case 'security':
				return 'Security';
			default:
				return logType;
		}
	}

	function formatTimestamp(ts: string): string {
		const d = new Date(ts);
		return d.toLocaleString();
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-2xl font-bold">Audit Logs</h1>
		<div class="flex items-center gap-2">
			<button
				onclick={handleRefresh}
				class="flex items-center justify-center rounded-lg border border-border-primary bg-bg-secondary p-1.5 text-text-secondary transition-colors hover:bg-bg-tertiary"
				title="Refresh"
			>
				<RefreshCw class="h-4 w-4 {refreshing ? 'animate-spin' : ''}" />
			</button>
			<button
				onclick={toggleLive}
				class="flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors {liveMode ? 'border-green-500 bg-green-500/10 text-green-400 hover:bg-green-500/20' : 'border-border-primary bg-bg-secondary text-text-secondary hover:bg-bg-tertiary'}"
				title="Live mode"
			>
				{#if liveMode}
					<span class="relative flex h-2 w-2">
						<span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75"></span>
						<span class="relative inline-flex h-2 w-2 rounded-full bg-green-500"></span>
					</span>
				{/if}
				<Radio class="h-4 w-4" />
				Live
			</button>
			<div class="relative">
				<button
					onclick={() => { showExportMenu = !showExportMenu; }}
					class="flex items-center gap-1.5 rounded-lg border border-border-primary bg-bg-secondary px-3 py-1.5 text-sm font-medium text-text-secondary transition-colors hover:bg-bg-tertiary"
				>
					<Download class="h-4 w-4" />
					Export
				</button>
				{#if showExportMenu}
					<div class="absolute right-0 top-full z-10 mt-1 w-36 rounded-lg border border-border-primary bg-bg-secondary py-1 shadow-lg">
						<button
							onclick={() => exportLogs('csv')}
							class="w-full px-3 py-1.5 text-left text-sm text-text-secondary hover:bg-bg-tertiary"
						>
							Export CSV
						</button>
						<button
							onclick={() => exportLogs('log')}
							class="w-full px-3 py-1.5 text-left text-sm text-text-secondary hover:bg-bg-tertiary"
						>
							Export Log
						</button>
					</div>
				{/if}
			</div>
			<button
				onclick={() => { showDeleteConfirm = true; }}
				class="flex items-center gap-1.5 rounded-lg border border-danger/30 bg-danger/10 px-3 py-1.5 text-sm font-medium text-danger transition-colors hover:bg-danger/20"
			>
				<Trash2 class="h-4 w-4" />
				Delete All
			</button>
		</div>
	</div>

	{#if error}
		<div class="mb-4 rounded-lg border border-danger/30 bg-danger/10 p-4 text-danger">{error}</div>
	{/if}

	<!-- Filters -->
	<div class="mb-4 flex flex-wrap items-center gap-3">
		<form onsubmit={handleSearch} class="flex flex-1 items-center gap-2">
			<div class="relative flex-1">
				<Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted" />
				<input
					type="text"
					bind:value={searchQuery}
					placeholder="Search logs..."
					class="w-full rounded-lg border border-border-primary bg-bg-secondary py-2 pl-9 pr-3 text-sm outline-none focus:border-accent"
				/>
			</div>
			<button
				type="submit"
				class="rounded-lg bg-accent px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-accent-hover"
			>
				Search
			</button>
		</form>

		<div class="flex items-center gap-2">
			<Filter class="h-4 w-4 text-text-muted" />
			<select
				bind:value={filterAction}
				onchange={() => { currentPage = 1; loadLogs(); }}
				class="rounded-lg border border-border-primary bg-bg-secondary px-3 py-2 text-sm outline-none focus:border-accent"
			>
				<option value="">All Actions</option>
				<option value="login">Login</option>
				<option value="logout">Logout</option>
				<option value="create">Create</option>
				<option value="update">Update</option>
				<option value="delete">Delete</option>
				<option value="request">Request</option>
			</select>

			<select
				bind:value={filterStatus}
				onchange={() => { currentPage = 1; loadLogs(); }}
				class="rounded-lg border border-border-primary bg-bg-secondary px-3 py-2 text-sm outline-none focus:border-accent"
			>
				<option value="">All Statuses</option>
				<option value="success">Success</option>
				<option value="error">Error</option>
				<option value="warning">Warning</option>
			</select>

			<select
				bind:value={filterLogType}
				onchange={() => { currentPage = 1; loadLogs(); }}
				class="rounded-lg border border-border-primary bg-bg-secondary px-3 py-2 text-sm outline-none focus:border-accent"
			>
				<option value="">All Types</option>
				<option value="api">API (LLM)</option>
				<option value="admin">Admin</option>
				<option value="security">Security</option>
			</select>
		</div>
	</div>

	<!-- Log Entries -->
	{#if loading}
		<div class="flex items-center justify-center py-20 text-text-muted">Loading...</div>
	{/if}

	{#if !loading && logs.length === 0}
		<div class="rounded-lg border border-border-primary bg-bg-secondary p-8 text-center text-text-muted">
			No log entries found
		</div>
	{/if}

	{#if !loading && logs.length > 0}
		<div class="space-y-1">
			{#each logs as log}
				<div
					class="rounded-lg border border-border-primary bg-bg-secondary px-4 py-3 transition-colors hover:bg-bg-tertiary/50 {newLogIds.has(log.id) ? 'animate-log-slide-in' : ''}"
				>
					<div class="flex flex-wrap items-center gap-x-4 gap-y-1">
						<span class="text-xs text-text-muted">{formatTimestamp(log.timestamp)}</span>
						<span
							class="rounded-full px-2 py-0.5 text-xs font-medium {logTypeBadgeClass(log.log_type)}"
						>
							{logTypeLabel(log.log_type)}
						</span>
						<span
							class="rounded-full px-2 py-0.5 text-xs font-medium {statusBadgeClass(log.status)}"
						>
							{log.status}
						</span>
						{#if log.streaming}
							<span class="rounded-full bg-accent/10 px-2 py-0.5 text-xs font-medium text-accent">
								Streaming
							</span>
						{/if}
						<span class="text-sm font-medium">{log.action}</span>
						<span class="text-sm text-text-secondary">{log.resource}</span>
						<span class="ml-auto text-xs text-text-muted">{log.user_email}</span>
					</div>
					{#if log.model || log.upstream_ttfb_ms || log.latency_ms || (log.prompt_tokens > 0 || log.completion_tokens > 0)}
						<div class="my-3 flex flex-wrap items-center gap-1.5">
							{#if log.model}
								<span class="rounded-full bg-purple-500/10 px-2 py-0.5 text-xs font-medium text-purple-400">{log.model}</span>
							{/if}
							{#if log.upstream_ttfb_ms}
								<span class="rounded-full bg-blue-500/10 px-2 py-0.5 text-xs text-blue-400">TTFB: {log.upstream_ttfb_ms}ms</span>
							{/if}
							{#if log.gateway_overhead_us}
								<span class="rounded-full bg-orange-500/10 px-2 py-0.5 text-xs text-orange-400">Overhead: {(log.gateway_overhead_us / 1000).toFixed(1)}ms</span>
							{/if}
							{#if log.latency_ms}
								<span class="rounded-full bg-emerald-500/10 px-2 py-0.5 text-xs text-emerald-400">Total: {log.latency_ms}ms</span>
							{/if}
							{#if log.prompt_tokens > 0 || log.completion_tokens > 0}
								<span class="rounded-full bg-cyan-500/10 px-2 py-0.5 text-xs text-cyan-400">Tokens: {log.prompt_tokens}→{log.completion_tokens}</span>
							{/if}
						</div>
					{/if}
					{#if log.detail}
						<p class="mt-1 text-xs text-text-muted">{log.detail}</p>
					{/if}
					{#if log.ip_address || log.x_real_ip || log.x_forwarded_for}
						<div class="mt-1 flex flex-wrap gap-x-4 gap-y-1">
							{#if log.ip_address}
								<span class="text-xs text-text-muted">IP: {log.ip_address}</span>
							{/if}
							{#if log.x_real_ip}
								<span class="text-xs text-text-muted">X-Real-IP: {log.x_real_ip}</span>
							{/if}
							{#if log.x_forwarded_for}
								<span class="text-xs text-text-muted">X-Forwarded-For: {log.x_forwarded_for}</span>
							{/if}
						</div>
					{/if}
				</div>
			{/each}
		</div>

		<!-- Pagination -->
		<div class="mt-4 flex items-center justify-between">
			<span class="text-sm text-text-muted">
				Showing {(currentPage - 1) * perPage + 1}-{Math.min(currentPage * perPage, total)} of {total}
			</span>
			<div class="flex items-center gap-1">
				<button
					onclick={() => goToPage(currentPage - 1)}
					disabled={currentPage <= 1}
					class="rounded-lg p-2 text-text-muted transition-colors hover:bg-bg-tertiary disabled:opacity-30"
					aria-label="Previous page"
				>
					<ChevronLeft class="h-4 w-4" />
				</button>
				<span class="px-3 text-sm text-text-secondary">
					{currentPage} / {totalPages}
				</span>
				<button
					onclick={() => goToPage(currentPage + 1)}
					disabled={currentPage >= totalPages}
					class="rounded-lg p-2 text-text-muted transition-colors hover:bg-bg-tertiary disabled:opacity-30"
					aria-label="Next page"
				>
					<ChevronRight class="h-4 w-4" />
				</button>
			</div>
		</div>
	{/if}
</div>

<ConfirmDialog
	bind:open={showDeleteConfirm}
	title="Delete All Logs"
	message="This will permanently delete all audit logs and reset usage metrics. This cannot be undone."
	confirmLabel="Delete All"
	variant="danger"
	onconfirm={deleteAllLogs}
	oncancel={() => {}}
/>
