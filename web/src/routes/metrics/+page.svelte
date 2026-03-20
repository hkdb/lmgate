<script lang="ts">
	import { get } from '$lib/api';
	import { ChartContainer, type ChartConfig } from '$lib/components/ui/chart';
	import { BarChart, LineChart } from 'layerchart';
	import { curveNatural } from 'd3-shape';
	import { RefreshCw, Radio } from 'lucide-svelte';

	interface DailyTiming {
		date: string;
		streaming: number;
		avg_gateway_overhead_ms: number;
		avg_upstream_ttfb_ms: number;
		avg_response_time_ms: number;
	}

	interface UsageMetrics {
		daily_requests: { date: string; count: number }[];
		model_usage: { model: string; count: number }[];
		user_usage: { email: string; count: number }[];
		token_usage: { name: string; count: number }[];
		daily_timings: DailyTiming[];
		avg_gateway_overhead_ms: number;
		avg_upstream_ttfb_ms: number;
		avg_streaming_gateway_overhead_ms: number;
		avg_upstream_streaming_ttfb_ms: number;
		avg_response_time_ms: number;
		avg_response_time_non_streaming_ms: number;
		avg_response_time_streaming_ms: number;
		total_tokens_used: number;
		total_requests: number;
		streaming_requests: number;
		non_streaming_requests: number;
		error_requests: number;
		streaming_error_requests: number;
		non_streaming_error_requests: number;
	}

	let metrics = $state<UsageMetrics | null>(null);
	let loading = $state(true);
	let error = $state('');
	let period = $state('7d');

	let refreshing = $state(false);
	let liveMode = $state(false);
	let eventSource = $state<EventSource | null>(null);
	let debounceTimer = $state<ReturnType<typeof setTimeout> | null>(null);
	let prevMetrics = $state<UsageMetrics | null>(null);
	let changedKeys = $state<Set<string>>(new Set());

	$effect(() => {
		loadMetrics();
		return () => {
			if (eventSource) {
				eventSource.close();
				eventSource = null;
			}
		};
	});

	async function loadMetrics() {
		loading = true;
		error = '';
		try {
			metrics = await get<UsageMetrics>(`/metrics?period=${period}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load metrics';
		} finally {
			loading = false;
		}
	}

	async function refreshMetrics() {
		try {
			const newMetrics = await get<UsageMetrics>(`/metrics?period=${period}`);
			if (metrics && newMetrics) {
				const keys: string[] = [
					'total_requests', 'streaming_requests', 'non_streaming_requests',
					'error_requests', 'streaming_error_requests', 'non_streaming_error_requests',
					'total_tokens_used', 'non_streaming_round_trip', 'streaming_round_trip',
					'avg_gateway_overhead_ms', 'avg_upstream_ttfb_ms', 'avg_response_time_non_streaming_ms',
					'avg_streaming_gateway_overhead_ms', 'avg_upstream_streaming_ttfb_ms', 'avg_response_time_streaming_ms'
				];
				const changed = new Set<string>();
				for (const key of keys) {
					if (getMetricValue(metrics, key) !== getMetricValue(newMetrics, key)) {
						changed.add(key);
					}
				}
				changedKeys = changed;
				if (changed.size > 0) {
					setTimeout(() => { changedKeys = new Set(); }, 500);
				}
			}
			prevMetrics = metrics;
			metrics = newMetrics;
		} catch {
			// silent fail for live refresh
		}
	}

	function getMetricValue(m: UsageMetrics, key: string): number {
		switch (key) {
			case 'total_requests': return m.total_requests;
			case 'streaming_requests': return m.streaming_requests;
			case 'non_streaming_requests': return m.non_streaming_requests;
			case 'error_requests': return m.error_requests;
			case 'streaming_error_requests': return m.streaming_error_requests;
			case 'non_streaming_error_requests': return m.non_streaming_error_requests;
			case 'total_tokens_used': return m.total_tokens_used;
			case 'non_streaming_round_trip': return m.avg_gateway_overhead_ms + m.avg_upstream_ttfb_ms + m.avg_response_time_non_streaming_ms;
			case 'streaming_round_trip': return m.avg_streaming_gateway_overhead_ms + m.avg_upstream_streaming_ttfb_ms + m.avg_response_time_streaming_ms;
			case 'avg_gateway_overhead_ms': return m.avg_gateway_overhead_ms;
			case 'avg_upstream_ttfb_ms': return m.avg_upstream_ttfb_ms;
			case 'avg_response_time_non_streaming_ms': return m.avg_response_time_non_streaming_ms;
			case 'avg_streaming_gateway_overhead_ms': return m.avg_streaming_gateway_overhead_ms;
			case 'avg_upstream_streaming_ttfb_ms': return m.avg_upstream_streaming_ttfb_ms;
			case 'avg_response_time_streaming_ms': return m.avg_response_time_streaming_ms;
			default: return 0;
		}
	}

	async function handleRefresh() {
		refreshing = true;
		await refreshMetrics();
		refreshing = false;
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
		const es = new EventSource('/admin/api/metrics/stream');
		es.onmessage = () => {
			if (debounceTimer) clearTimeout(debounceTimer);
			debounceTimer = setTimeout(() => {
				refreshMetrics();
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

	function changePeriod(newPeriod: string) {
		period = newPeriod;
		loadMetrics();
	}

	// Bar chart data
	let dailyChartData = $derived(
		metrics?.daily_requests.map((d) => ({
			label: new Date(d.date).toLocaleDateString('en', { month: 'short', day: 'numeric' }),
			value: d.count
		})) ?? []
	);

	let modelChartData = $derived(
		metrics?.model_usage.map((m) => ({
			label: m.model.length > 12 ? m.model.slice(0, 12) + '...' : m.model,
			value: m.count
		})) ?? []
	);

	let userChartData = $derived(
		metrics?.user_usage.map((u) => ({
			label: u.email.split('@')[0],
			value: u.count
		})) ?? []
	);

	// Chart configs for bar charts
	const dailyChartConfig: ChartConfig = {
		value: { label: 'Requests', color: '#3b82f6' }
	};

	const modelChartConfig: ChartConfig = {
		value: { label: 'Requests', color: '#8b5cf6' }
	};

	const userChartConfig: ChartConfig = {
		value: { label: 'Requests', color: '#22c55e' }
	};

	// Overhead trend data (ms scale)
	let overheadDailyData = $derived.by(() => {
		if (!metrics?.daily_timings) return [];
		const byDate = new Map<string, { ns_overhead: number; s_overhead: number }>();
		for (const d of metrics.daily_timings) {
			const entry = byDate.get(d.date) ?? { ns_overhead: 0, s_overhead: 0 };
			if (d.streaming === 0) {
				entry.ns_overhead = Math.round(d.avg_gateway_overhead_ms * 100) / 100;
			} else {
				entry.s_overhead = Math.round(d.avg_gateway_overhead_ms * 100) / 100;
			}
			byDate.set(d.date, entry);
		}
		return Array.from(byDate.entries()).map(([date, v]) => ({ date, ...v }));
	});

	// Timing trend data (converted to seconds)
	let timingDailyData = $derived.by(() => {
		if (!metrics?.daily_timings) return [];
		const byDate = new Map<string, { ns_ttfb: number; ns_response_time: number; ns_round_trip: number; s_ttfb: number; s_response_time: number; s_round_trip: number }>();
		for (const d of metrics.daily_timings) {
			const entry = byDate.get(d.date) ?? { ns_ttfb: 0, ns_response_time: 0, ns_round_trip: 0, s_ttfb: 0, s_response_time: 0, s_round_trip: 0 };
			if (d.streaming === 0) {
				entry.ns_ttfb = Math.round(d.avg_upstream_ttfb_ms / 10) / 100;
				entry.ns_response_time = Math.round(d.avg_response_time_ms / 10) / 100;
				entry.ns_round_trip = Math.round((d.avg_gateway_overhead_ms + d.avg_upstream_ttfb_ms + d.avg_response_time_ms) / 10) / 100;
			} else {
				entry.s_ttfb = Math.round(d.avg_upstream_ttfb_ms / 10) / 100;
				entry.s_response_time = Math.round(d.avg_response_time_ms / 10) / 100;
				entry.s_round_trip = Math.round((d.avg_gateway_overhead_ms + d.avg_upstream_ttfb_ms + d.avg_response_time_ms) / 10) / 100;
			}
			byDate.set(d.date, entry);
		}
		return Array.from(byDate.entries()).map(([date, v]) => ({ date, ...v }));
	});

	const overheadChartConfig: ChartConfig = {
		ns_overhead: { label: 'Non-Streaming', color: '#22c55e' },
		s_overhead: { label: 'Streaming', color: '#f97316' }
	};

	const overheadSeries = [
		{ key: 'ns_overhead', label: 'Non-Streaming', color: '#22c55e' },
		{ key: 's_overhead', label: 'Streaming', color: '#f97316' }
	];

	const timingChartConfig: ChartConfig = {
		ns_round_trip: { label: 'NS Round Trip', color: '#3b82f6' },
		ns_ttfb: { label: 'NS LLM TTFB', color: '#a78bfa' },
		ns_response_time: { label: 'NS Response Time', color: '#38bdf8' },
		s_round_trip: { label: 'S Round Trip', color: '#f97316' },
		s_ttfb: { label: 'S LLM TTFB', color: '#f472b6' },
		s_response_time: { label: 'S Response Time', color: '#fb923c' }
	};

	const timingSeries = [
		{ key: 'ns_round_trip', label: 'NS Round Trip', color: '#3b82f6' },
		{ key: 'ns_ttfb', label: 'NS LLM TTFB', color: '#a78bfa' },
		{ key: 'ns_response_time', label: 'NS Response Time', color: '#38bdf8' },
		{ key: 's_round_trip', label: 'S Round Trip', color: '#f97316' },
		{ key: 's_ttfb', label: 'S LLM TTFB', color: '#f472b6' },
		{ key: 's_response_time', label: 'S Response Time', color: '#fb923c' }
	];
</script>

<div>
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-2xl font-bold">Usage Metrics</h1>
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
						<span class="relative inline-flex h-2 w-2 rounded-full bg-green-400"></span>
					</span>
				{/if}
				<Radio class="h-4 w-4" />
				Live
			</button>
			<div class="flex rounded-lg border border-border-primary">
				{#each [
					{ value: '24h', label: '24h' },
					{ value: '7d', label: '7d' },
					{ value: '30d', label: '30d' },
					{ value: '90d', label: '90d' }
				] as opt}
					<button
						onclick={() => changePeriod(opt.value)}
						class="px-3 py-1.5 text-sm transition-colors first:rounded-l-lg last:rounded-r-lg
						{period === opt.value
							? 'bg-accent text-white'
							: 'text-text-secondary hover:bg-bg-tertiary'}"
					>
						{opt.label}
					</button>
				{/each}
			</div>
		</div>
	</div>

	{#if error}
		<div class="mb-4 rounded-lg border border-danger/30 bg-danger/10 p-4 text-danger">{error}</div>
	{/if}

	{#if loading}
		<div class="flex items-center justify-center py-20 text-text-muted">Loading...</div>
	{/if}

	{#if metrics}
		<!-- Gateway Overhead trend chart -->
		{#if overheadDailyData.length > 0}
			<div class="mb-6 rounded-xl border border-border-primary bg-bg-secondary p-5">
				<h2 class="mb-4 text-sm font-medium text-text-secondary">Gateway Overhead Trends (ms)</h2>
				<ChartContainer config={overheadChartConfig} class="h-[280px] w-full">
					<LineChart
						data={overheadDailyData}
						x="date"
						series={overheadSeries}
						legend
						tooltip={{ mode: 'bisect-x' }}
						props={{
							spline: { curve: curveNatural },
							yAxis: { format: (v: number) => `${v.toFixed(1)}` },
							xAxis: { format: (v: string) => new Date(v).toLocaleDateString('en', { month: 'short', day: 'numeric' }) }
						}}
					/>
				</ChartContainer>
			</div>
		{/if}

		<!-- Summary cards -->
		<div class="mb-6 grid gap-4 sm:grid-cols-3">
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Total Requests</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('total_requests') ? 'animate-metric-pulse' : ''}">{metrics.total_requests.toLocaleString()}</div>
			</div>
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Streaming Requests</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('streaming_requests') ? 'animate-metric-pulse' : ''}">{metrics.streaming_requests.toLocaleString()}</div>
			</div>
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Non-Streaming Requests</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('non_streaming_requests') ? 'animate-metric-pulse' : ''}">{metrics.non_streaming_requests.toLocaleString()}</div>
			</div>
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Total Error(s)</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('error_requests') ? 'animate-metric-pulse' : ''}">{metrics.error_requests.toLocaleString()}</div>
			</div>
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Streaming Error(s)</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('streaming_error_requests') ? 'animate-metric-pulse' : ''}">{metrics.streaming_error_requests.toLocaleString()}</div>
			</div>
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Non-Streaming Error(s)</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('non_streaming_error_requests') ? 'animate-metric-pulse' : ''}">{metrics.non_streaming_error_requests.toLocaleString()}</div>
			</div>
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Total Tokens Used</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('total_tokens_used') ? 'animate-metric-pulse' : ''}">{metrics.total_tokens_used.toLocaleString()}</div>
			</div>
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Avg Non-Streaming Round Trip</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('non_streaming_round_trip') ? 'animate-metric-pulse' : ''}">{((metrics.avg_gateway_overhead_ms + metrics.avg_upstream_ttfb_ms + metrics.avg_response_time_non_streaming_ms) / 1000).toFixed(2)}s</div>
			</div>
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Avg Streaming Round Trip</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('streaming_round_trip') ? 'animate-metric-pulse' : ''}">{((metrics.avg_streaming_gateway_overhead_ms + metrics.avg_upstream_streaming_ttfb_ms + metrics.avg_response_time_streaming_ms) / 1000).toFixed(2)}s</div>
			</div>
		</div>

		<!-- Non-streaming timing -->
		<div class="mb-6 grid gap-4 sm:grid-cols-3">
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Avg Non-Streaming Gateway Overhead</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('avg_gateway_overhead_ms') ? 'animate-metric-pulse' : ''}">{metrics.avg_gateway_overhead_ms.toFixed(2)}ms</div>
			</div>
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Avg Non-Streaming LLM TTFB</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('avg_upstream_ttfb_ms') ? 'animate-metric-pulse' : ''}">{(metrics.avg_upstream_ttfb_ms / 1000).toFixed(2)}s</div>
			</div>
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Avg Non-Streaming LLM Response Time</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('avg_response_time_non_streaming_ms') ? 'animate-metric-pulse' : ''}">{(metrics.avg_response_time_non_streaming_ms / 1000).toFixed(2)}s</div>
			</div>
		</div>

		<!-- Streaming timing -->
		<div class="mb-6 grid gap-4 sm:grid-cols-3">
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Avg Streaming Gateway Overhead</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('avg_streaming_gateway_overhead_ms') ? 'animate-metric-pulse' : ''}">{metrics.avg_streaming_gateway_overhead_ms.toFixed(2)}ms</div>
			</div>
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Avg Streaming LLM TTFB</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('avg_upstream_streaming_ttfb_ms') ? 'animate-metric-pulse' : ''}">{(metrics.avg_upstream_streaming_ttfb_ms / 1000).toFixed(2)}s</div>
			</div>
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<div class="text-sm text-text-secondary">Avg Streaming Response Time</div>
				<div class="mt-1 text-3xl font-bold {changedKeys.has('avg_response_time_streaming_ms') ? 'animate-metric-pulse' : ''}">{(metrics.avg_response_time_streaming_ms / 1000).toFixed(2)}s</div>
			</div>
		</div>

		<!-- Combined Timing Trends chart -->
		{#if timingDailyData.length > 0}
			<div class="mb-6 rounded-xl border border-border-primary bg-bg-secondary p-5">
				<h2 class="mb-4 text-sm font-medium text-text-secondary">Timing Trends (s)</h2>
				<ChartContainer config={timingChartConfig} class="h-[280px] w-full">
					<LineChart
						data={timingDailyData}
						x="date"
						series={timingSeries}
						legend
						tooltip={{ mode: 'bisect-x' }}
						props={{
							spline: { curve: curveNatural },
							yAxis: { format: (v: number) => `${v.toFixed(1)}s` },
							xAxis: { format: (v: string) => new Date(v).toLocaleDateString('en', { month: 'short', day: 'numeric' }) }
						}}
					/>
				</ChartContainer>
			</div>
		{/if}

		<!-- Daily requests chart -->
		{#if dailyChartData.length > 0}
			<div class="mb-6 rounded-xl border border-border-primary bg-bg-secondary p-5">
				<h2 class="mb-4 text-sm font-medium text-text-secondary">Daily Requests</h2>
				<ChartContainer config={dailyChartConfig} class="h-[220px] w-full">
					<BarChart
						data={dailyChartData}
						x="label"
						series={[{ key: 'value', label: 'Requests', color: '#3b82f6' }]}
						tooltip={{ mode: 'band' }}
						bandPadding={0.7}
					/>
				</ChartContainer>
			</div>
		{/if}

		<div class="grid gap-6 md:grid-cols-2">
			<!-- Model usage chart -->
			{#if modelChartData.length > 0}
				<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
					<h2 class="mb-4 text-sm font-medium text-text-secondary">By Model</h2>
					<ChartContainer config={modelChartConfig} class="h-[200px] w-full">
						<BarChart
							data={modelChartData}
							x="label"
							series={[{ key: 'value', label: 'Requests', color: '#8b5cf6' }]}
							tooltip={{ mode: 'band' }}
							bandPadding={0.7}
						/>
					</ChartContainer>
				</div>
			{/if}

			<!-- User usage chart -->
			{#if userChartData.length > 0}
				<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
					<h2 class="mb-4 text-sm font-medium text-text-secondary">By User</h2>
					<ChartContainer config={userChartConfig} class="h-[200px] w-full">
						<BarChart
							data={userChartData}
							x="label"
							series={[{ key: 'value', label: 'Requests', color: '#22c55e' }]}
							tooltip={{ mode: 'band' }}
							bandPadding={0.7}
						/>
					</ChartContainer>
				</div>
			{/if}
		</div>
	{/if}
</div>
