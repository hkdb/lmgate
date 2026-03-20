<script lang="ts" generics="T extends Record<string, any>">
	import type { Snippet } from 'svelte';

	interface Column<R> {
		key: keyof R & string;
		label: string;
		render?: (value: R[keyof R], row: R) => string;
		class?: string;
	}

	interface Props {
		columns: Column<T>[];
		rows: T[];
		emptyMessage?: string;
		actions?: Snippet<[T]>;
		sortKey?: string;
		sortDir?: 'asc' | 'desc';
		onSort?: (key: string) => void;
	}

	let { columns, rows, emptyMessage = 'No data found', actions, sortKey, sortDir, onSort }: Props = $props();

	function cellValue(row: T, col: Column<T>): string {
		const val = row[col.key];
		if (col.render) return col.render(val, row);
		if (val == null) return '';
		return String(val);
	}
</script>

<div class="overflow-x-auto rounded-lg border border-border-primary">
	<table class="w-full text-left text-sm">
		<thead class="border-b border-border-primary bg-bg-secondary text-text-secondary">
			<tr>
				{#each columns as col}
					<th class="px-4 py-3 font-medium {col.class ?? ''}">
						{#if onSort}
							<button
								type="button"
								class="inline-flex items-center gap-1 hover:text-text-primary"
								onclick={() => onSort(col.key)}
							>
								{col.label}
								{#if sortKey === col.key}
									<span class="text-xs">{sortDir === 'asc' ? '▲' : '▼'}</span>
								{/if}
							</button>
						{:else}
							{col.label}
						{/if}
					</th>
				{/each}
				{#if actions}
					<th class="px-4 py-3 font-medium text-right">Actions</th>
				{/if}
			</tr>
		</thead>
		<tbody class="divide-y divide-border-primary">
			{#if rows.length === 0}
				<tr>
					<td
						colspan={columns.length + (actions ? 1 : 0)}
						class="px-4 py-8 text-center text-text-muted"
					>
						{emptyMessage}
					</td>
				</tr>
			{/if}
			{#each rows as row}
				<tr class="hover:bg-bg-secondary/50 transition-colors">
					{#each columns as col}
						<td class="px-4 py-3 {col.class ?? ''}">{cellValue(row, col)}</td>
					{/each}
					{#if actions}
						<td class="px-4 py-3 text-right">
							{@render actions(row)}
						</td>
					{/if}
				</tr>
			{/each}
		</tbody>
	</table>
</div>
