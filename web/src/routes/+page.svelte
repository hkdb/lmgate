<script lang="ts">
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { get } from '$lib/api';
	import { auth } from '$lib/stores.svelte';
	import { Users, KeyRound, Box, Activity } from 'lucide-svelte';

	interface DashboardMetrics {
		total_users: number;
		active_tokens: number;
		available_models: number;
		requests_today: number;
		requests_this_week: number;
		error_rate: number;
	}

	let metrics = $state<DashboardMetrics | null>(null);
	let error = $state('');
	let loading = $state(true);

	$effect(() => {
		if (!auth.checked) return;
		if (!auth.isAdmin) {
			goto(`${base}/models`);
			return;
		}
		loadMetrics();
	});

	async function loadMetrics() {
		loading = true;
		error = '';
		try {
			metrics = await get<DashboardMetrics>('/dashboard');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load dashboard';
		} finally {
			loading = false;
		}
	}

	const cards = $derived(
		metrics
			? [
					{
						label: 'Total Users',
						value: metrics.total_users,
						icon: Users,
						color: 'text-blue-400'
					},
					{
						label: 'Active Tokens',
						value: metrics.active_tokens,
						icon: KeyRound,
						color: 'text-green-400'
					},
					{
						label: 'Available Models',
						value: metrics.available_models,
						icon: Box,
						color: 'text-purple-400'
					},
					{
						label: 'Requests Today',
						value: metrics.requests_today,
						icon: Activity,
						color: 'text-amber-400'
					}
				]
			: []
	);
</script>

<div>
	<h1 class="mb-6 text-2xl font-bold">Dashboard</h1>

	{#if loading}
		<div class="flex items-center justify-center py-20 text-text-muted">Loading...</div>
	{/if}

	{#if error}
		<div class="rounded-lg border border-danger/30 bg-danger/10 p-4 text-danger">{error}</div>
	{/if}

	{#if metrics}
		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
			{#each cards as card}
				<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
					<div class="flex items-center justify-between">
						<span class="text-sm text-text-secondary">{card.label}</span>
						<card.icon class="h-5 w-5 {card.color}" />
					</div>
					<div class="mt-2 text-3xl font-bold">{card.value}</div>
				</div>
			{/each}
		</div>

		<div class="mt-8 grid gap-4 md:grid-cols-2">
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<h2 class="mb-2 text-sm font-medium text-text-secondary">Requests This Week</h2>
				<div class="text-3xl font-bold">{metrics.requests_this_week}</div>
			</div>
			<div class="rounded-xl border border-border-primary bg-bg-secondary p-5">
				<h2 class="mb-2 text-sm font-medium text-text-secondary">Error Rate</h2>
				<div class="text-3xl font-bold">{(metrics.error_rate * 100).toFixed(1)}%</div>
			</div>
		</div>
	{/if}
</div>
