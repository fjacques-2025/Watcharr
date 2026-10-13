<script lang="ts" module>
	import type { LiveCreditRequest } from "@/types";

	/** One editable credit line. `key` only keeps the rows stable in #each. */
	export interface CreditRow {
		key: number;
		role: string;
		name: string;
		tmdbPersonId: number | null;
	}

	let nextKey = 0;

	export function newCreditRow(c?: Partial<Omit<CreditRow, "key">>): CreditRow {
		return {
			key: nextKey++,
			role: c?.role ?? "",
			name: c?.name ?? "",
			tmdbPersonId: c?.tmdbPersonId ?? null,
		};
	}

	/** Rows → request, in order. A row without a name is an unused line
	 *  (e.g. a role filled in by default) and is left out. */
	export function creditRowsToRequest(rows: CreditRow[]): LiveCreditRequest[] {
		return rows
			.filter((r) => r.name.trim())
			.map((r) => ({
				name: r.name.trim(),
				role: r.role.trim(),
				tmdbPersonId: r.tmdbPersonId,
			}));
	}

	/** A message when a named row has no role; undefined when all is well. */
	export function validateCreditRows(rows: CreditRow[]): string | undefined {
		if (rows.some((r) => r.name.trim() && !r.role.trim()))
			return "Each artist needs a role (Performer, Director…).";
	}
</script>

<script lang="ts">
	import { tick } from "svelte";
	import Icon from "@/lib/Icon.svelte";
	import ArtistSuggest from "./ArtistSuggest.svelte";

	interface Props {
		rows: CreditRow[];
	}

	let { rows = $bindable() }: Props = $props();

	const defaultRoles = [
		"Performer",
		"Writer",
		"Director",
		"Choreographer",
		"Company",
		"Music",
		"Translation",
	];
	const uid = $props.id();
	const rolesListId = `${uid}-roles`;

	const nameInputId = (key: number) => `${uid}-name-${key}`;

	async function add() {
		// Same role as the line above: performers usually come in a run.
		const role = rows.length ? rows[rows.length - 1].role : "Performer";
		const row = newCreditRow({ role });
		rows.push(row);
		await tick();
		document.getElementById(nameInputId(row.key))?.focus();
	}

	function remove(i: number) {
		rows.splice(i, 1);
	}

	function move(i: number, by: -1 | 1) {
		const j = i + by;
		if (j < 0 || j >= rows.length) return;
		const [r] = rows.splice(i, 1);
		rows.splice(j, 0, r);
	}
</script>

<fieldset class="credits-editor">
	<legend>Artists</legend>
	<datalist id={rolesListId}>
		{#each defaultRoles as r (r)}
			<option value={r}></option>
		{/each}
	</datalist>

	{#if rows.length}
		<ol>
			{#each rows as row, i (row.key)}
				<li>
					<input
						class="role"
						type="text"
						bind:value={row.role}
						list={rolesListId}
						placeholder="Role"
						aria-label="Role"
						autocomplete="off"
						maxlength={100}
					/>
					<div class="name">
						<ArtistSuggest
							id={nameInputId(row.key)}
							bind:value={row.name}
							placeholder="Name"
							ariaLabel="Name"
							onType={() => {
								// The link was to the person that was picked; a
								// name edited by hand may be someone else.
								row.tmdbPersonId = null;
							}}
							onPick={(p) => {
								row.name = p.name;
								row.tmdbPersonId = p.tmdbPersonId;
							}}
						/>
						{#if row.tmdbPersonId}
							<span class="tmdb-badge" title="Linked to a TMDB person">
								TMDB
								<button
									type="button"
									class="plain"
									aria-label="Unlink from TMDB"
									title="Unlink from TMDB"
									onclick={() => (row.tmdbPersonId = null)}
								>
									×
								</button>
							</span>
						{/if}
					</div>
					<div class="row-actions">
						<button
							type="button"
							class="plain"
							aria-label="Move up"
							title="Move up"
							disabled={i === 0}
							onclick={() => move(i, -1)}
						>
							<Icon i="chevron" wh={14} facing="up" />
						</button>
						<button
							type="button"
							class="plain"
							aria-label="Move down"
							title="Move down"
							disabled={i === rows.length - 1}
							onclick={() => move(i, 1)}
						>
							<Icon i="chevron" wh={14} facing="down" />
						</button>
						<button
							type="button"
							class="plain"
							aria-label="Remove"
							title="Remove"
							onclick={() => remove(i)}
						>
							<Icon i="close" wh={14} />
						</button>
					</div>
				</li>
			{/each}
		</ol>
	{/if}

	<button type="button" class="add" onclick={add}>
		<Icon i="add" wh={14} /> Add an artist
	</button>
</fieldset>

<style lang="scss">
	.credits-editor {
		display: flex;
		flex-flow: column;
		gap: 8px;
		min-width: 0;
		margin: 0;
		padding: 0;
		border: 0;

		legend {
			margin-bottom: 5px;
			padding: 0;
			font-size: 14px;
			font-weight: bold;
		}
	}

	ol {
		display: flex;
		flex-flow: column;
		gap: 8px;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	li {
		display: grid;
		grid-template-columns: minmax(0, 2fr) minmax(0, 3fr) auto;
		align-items: center;
		gap: 6px;

		// On a phone: role and name stacked, the buttons beside them.
		@media (max-width: 500px) {
			grid-template-columns: minmax(0, 1fr) auto;
			padding-bottom: 8px;
			border-bottom: 1px solid $bg-color-accent;

			.role {
				grid-column: 1;
				grid-row: 1;
			}
			.name {
				grid-column: 1;
				grid-row: 2;
			}
			.row-actions {
				grid-column: 2;
				grid-row: 1 / span 2;
				flex-flow: column;
			}
		}
	}

	.role {
		color: $text-color;
		background-color: transparent;
		border-color: $text-color;
	}

	.name {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
	}

	.tmdb-badge {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		flex: 0 0 auto;
		padding: 0 0 0 6px;
		border: 1px solid $bg-color-accent;
		border-radius: 8px;
		font-size: 11px;
		color: $text-color-accent;

		button {
			padding: 2px 6px;
			font-size: 14px;
			line-height: 1;
			color: $text-color-accent;

			@media (hover: hover) {
				&:hover {
					color: $text-color;
				}
			}
		}
	}

	.row-actions {
		display: flex;
		flex-flow: row;
		align-items: center;

		button {
			display: flex;
			align-items: center;
			justify-content: center;
			width: 30px;
			height: 30px;
			padding: 0;
			border-radius: 6px;
			fill: $text-color;

			&:disabled {
				opacity: 0.3;
				cursor: default;
			}

			@media (hover: hover) {
				&:not(:disabled):hover {
					background-color: $bg-color-accent;
				}
			}
		}
	}

	.add {
		gap: 6px;
		width: auto;
		align-self: flex-start;
		font-size: 14px;
	}
</style>
