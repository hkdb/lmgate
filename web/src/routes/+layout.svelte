<script lang="ts">
	import '../app.css';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { base } from '$app/paths';
	import { auth } from '$lib/stores.svelte';
	import {
		LayoutDashboard,
		Users,
		UsersRound,
		KeyRound,
		Box,
		ScrollText,
		BarChart3,
		Settings,
		LogOut,
		CircleUser,
		Menu,
		X
	} from 'lucide-svelte';
	import type { Snippet } from 'svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	let sidebarOpen = $state(false);

	const adminNavItems = [
		{ href: `${base}/`, label: 'Dashboard', icon: LayoutDashboard },
		{ href: `${base}/users`, label: 'Users', icon: Users },
		{ href: `${base}/groups`, label: 'Groups', icon: UsersRound },
		{ href: `${base}/tokens`, label: 'API Tokens', icon: KeyRound },
		{ href: `${base}/models`, label: 'Models', icon: Box },
		{ href: `${base}/logs`, label: 'Logs', icon: ScrollText },
		{ href: `${base}/metrics`, label: 'Metrics', icon: BarChart3 },
		{ href: `${base}/settings`, label: 'Settings', icon: Settings }
	];

	const userNavItems = [
		{ href: `${base}/tokens`, label: 'API Tokens', icon: KeyRound },
		{ href: `${base}/models`, label: 'Models', icon: Box }
	];

	let navItems = $derived(auth.isAdmin ? adminNavItems : userNavItems);

	$effect(() => {
		auth.initialize();
	});

	$effect(() => {
		if (!auth.checked) return;

		if (!auth.isAuthenticated && !$page.url.pathname.endsWith('/login')) {
			goto(`${base}/login`);
			return;
		}

		if (auth.isAuthenticated && $page.url.pathname.endsWith('/login')) {
			goto(auth.isAdmin ? `${base}/` : `${base}/models`);
			return;
		}

		if (
			(auth.user?.force_password_change || auth.user?.password_expired) &&
			!$page.url.pathname.endsWith('/change-password')
		) {
			goto(`${base}/change-password`);
			return;
		}

		if (
			auth.user?.enforce_2fa &&
			!$page.url.pathname.endsWith('/account') &&
			!$page.url.pathname.endsWith('/change-password') &&
			!$page.url.pathname.endsWith('/login')
		) {
			goto(`${base}/account`);
		}
	});

	function isActive(href: string, currentPath: string): boolean {
		if (href === `${base}/`) return currentPath === `${base}/` || currentPath === base;
		return currentPath.startsWith(href);
	}

	function handleLogout() {
		auth.logout();
	}

	function closeSidebar() {
		sidebarOpen = false;
	}
</script>

{#if !auth.checked}
	<!-- Loading auth state -->
{:else if !auth.isAuthenticated}
	{@render children()}
{:else}
	<div class="flex h-screen overflow-hidden">
		<!-- Mobile overlay -->
		{#if sidebarOpen}
			<button
				class="fixed inset-0 z-40 bg-black/50 lg:hidden"
				onclick={closeSidebar}
				aria-label="Close sidebar"
			></button>
		{/if}

		<!-- Sidebar -->
		<aside
			class="fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r border-border-primary bg-bg-secondary transition-transform duration-200 lg:static lg:translate-x-0
			{sidebarOpen ? 'translate-x-0' : '-translate-x-full'}"
		>
			<div class="flex h-14 items-center gap-2 border-b border-border-primary px-4">
				<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" class="h-6 w-6 text-accent" aria-hidden="true">
					<defs>
						<mask id="sidebar-keymask">
							<rect width="100" height="100" fill="white"/>
							<circle cx="50" cy="37" r="7.5" fill="black"/>
							<path d="M46.5,42 L46.5,59 Q46.5,61 48.5,61 L51.5,61 Q53.5,61 53.5,59 L53.5,42 Z" fill="black"/>
						</mask>
					</defs>
					<path d="M50,8 L88,22 L88,52 Q88,82 50,94 Q12,82 12,52 L12,22 Z" fill="none" stroke="currentColor" stroke-width="4" stroke-linejoin="round"/>
					<g mask="url(#sidebar-keymask)">
						<circle cx="50" cy="37" r="15" fill="currentColor"/>
						<path d="M41,42 L41,71 Q41,74 44,74 L56,74 Q59,74 59,71 L59,42 Z" fill="currentColor"/>
					</g>
					<g opacity="0.6" transform="translate(50, 37)">
						<line x1="0" y1="-4" x2="0" y2="4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
						<line x1="-4" y1="0" x2="4" y2="0" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
						<line x1="-2.8" y1="-2.8" x2="2.8" y2="2.8" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
						<line x1="2.8" y1="-2.8" x2="-2.8" y2="2.8" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
					</g>
				</svg>
				<span class="text-lg font-semibold">LM Gate</span>
				<span class="ml-auto text-xs text-text-muted">{auth.user?.role ?? ''}</span>
			</div>

			<nav class="flex-1 space-y-1 overflow-y-auto p-3">
				{#each navItems as item}
					<a
						href={item.href}
						onclick={closeSidebar}
						class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors
						{isActive(item.href, $page.url.pathname)
							? 'bg-accent/10 text-accent'
							: 'text-text-secondary hover:bg-bg-tertiary hover:text-text-primary'}"
					>
						<item.icon class="h-4 w-4" />
						{item.label}
					</a>
				{/each}
			</nav>

			<div class="border-t border-border-primary p-3">
				<a
					href="{base}/account"
					onclick={closeSidebar}
					class="mb-2 flex items-center gap-2 truncate rounded-lg px-3 py-1.5 text-xs text-text-muted transition-colors hover:bg-bg-tertiary hover:text-accent"
				>
					<CircleUser class="h-4 w-4 shrink-0" />
					{auth.user?.email ?? ''}
				</a>
				<button
					onclick={handleLogout}
					class="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-text-secondary transition-colors hover:bg-bg-tertiary hover:text-danger"
				>
					<LogOut class="h-4 w-4" />
					Logout
				</button>
			</div>
		</aside>

		<!-- Main content -->
		<div class="flex flex-1 flex-col overflow-hidden">
			<!-- Top bar (mobile) -->
			<header class="flex h-14 items-center gap-4 border-b border-border-primary px-4 lg:hidden">
				<button
					onclick={() => (sidebarOpen = !sidebarOpen)}
					class="rounded-lg p-1.5 text-text-secondary hover:bg-bg-tertiary"
					aria-label="Toggle sidebar"
				>
					{#if sidebarOpen}
						<X class="h-5 w-5" />
					{/if}
					{#if !sidebarOpen}
						<Menu class="h-5 w-5" />
					{/if}
				</button>
				<span class="font-semibold">LM Gate</span>
			</header>

			<main class="flex-1 overflow-y-auto p-4 md:p-6">
				{@render children()}
			</main>
		</div>
	</div>
{/if}
