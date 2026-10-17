<script lang="ts">
	// People read from a page or a pasted list, offered for the Artists
	// editor: one line each, ticked when the server thinks they belong there
	// (technical and admin people come unticked). Roles can be dropped before
	// adding — for everyone at once from the row at the top ("création et
	// interprétation" credits every performer with "Création"), or one person
	// at a time.
	import type { LiveCreditSuggestion } from "@/types";
	import { defaultRoles } from "./LiveCreditsEditor.svelte";

	interface Props {
		suggestions: LiveCreditSuggestion[];
		onAdd: (picked: { name: string; roles: string[] }[]) => void;
		onDismiss: () => void;
	}

	let { suggestions, onAdd, onDismiss }: Props = $props();

	const uid = $props.id();

	// All three start over when a new list comes in.
	// Starts from the server's choice; ticking overrides it.
	let checked = $derived(suggestions.map((s) => s.suggested));
	// Roles switched off for everyone.
	let offForAll: Record<string, boolean> = $derived.by(() => {
		void suggestions;
		return {};
	});
	// One person's own choice for a role, which beats offForAll.
	let personRole: Record<number, Record<string, boolean>> = $derived.by(() => {
		void suggestions;
		return {};
	});

	// Every role found, usual order first (Performer, Writer…), then the
	// others as they appear, with how many people hold each.
	let allRoles = $derived.by(() => {
		const seen: string[] = [];
		const n: Record<string, number> = {};
		for (const s of suggestions)
			for (const r of s.roles) {
				if (!(r in n)) seen.push(r);
				n[r] = (n[r] ?? 0) + 1;
			}
		const known = defaultRoles.filter((r) => r in n);
		const other = seen.filter((r) => !defaultRoles.includes(r));
		return [...known, ...other].map((role) => ({ role, n: n[role] }));
	});

	// A label the parser didn't recognise, kept as written ("Régie
	// générale"): coloured apart so it can be checked before adding.
	const isFree = (role: string) => !defaultRoles.includes(role);
	const FREE_HINT = " (not a usual role, kept as written)";

	function roleOn(i: number, role: string) {
		return personRole[i]?.[role] ?? !offForAll[role];
	}

	function rolesOf(i: number) {
		return suggestions[i].roles.filter((r) => roleOn(i, r));
	}

	// Someone left with no role can't be added.
	let addable = $derived(
		suggestions.map((_, i) => checked[i] && rolesOf(i).length > 0),
	);
	let count = $derived(addable.filter(Boolean).length);

	function setAll(value: boolean) {
		checked = suggestions.map(() => value);
	}

	function toggle(i: number, value: boolean) {
		checked = checked.map((c, j) => (j === i ? value : c));
	}

	function toggleForAll(role: string) {
		const off = !offForAll[role];
		offForAll = { ...offForAll, [role]: off };
		// A global switch resets the individual choices for that role.
		const next: Record<number, Record<string, boolean>> = {};
		for (const [i, m] of Object.entries(personRole)) {
			const copy = { ...m };
			delete copy[role];
			next[Number(i)] = copy;
		}
		personRole = next;
	}

	function togglePersonRole(i: number, role: string) {
		personRole = {
			...personRole,
			[i]: { ...personRole[i], [role]: !roleOn(i, role) },
		};
	}

	function add() {
		const picked = suggestions
			.map((s, i) => ({ name: s.name, roles: rolesOf(i), ok: addable[i] }))
			.filter((p) => p.ok)
			.map(({ name, roles }) => ({ name, roles }));
		if (picked.length) onAdd(picked);
	}
</script>

<div class="suggestions">
	{#if allRoles.length > 1}
		<div class="all-roles" role="group" aria-label="Roles for everyone">
			<span class="hint">Roles:</span>
			{#each allRoles as { role, n } (role)}
				<button
					type="button"
					class="chip"
					class:free={isFree(role)}
					aria-pressed={!offForAll[role]}
					title={(offForAll[role]
						? `Give "${role}" back to everyone`
						: `Drop "${role}" for everyone`) + (isFree(role) ? FREE_HINT : "")}
					onclick={() => toggleForAll(role)}
				>
					{role} <span class="n">{n}</span>
				</button>
			{/each}
		</div>
	{/if}

	<div class="bulk">
		<span class="count">{count} of {suggestions.length} selected</span>
		<button
			type="button"
			class="plain link"
			disabled={checked.every(Boolean)}
			onclick={() => setAll(true)}
		>
			Select all
		</button>
		<button
			type="button"
			class="plain link"
			disabled={!checked.some(Boolean)}
			onclick={() => setAll(false)}
		>
			None
		</button>
	</div>

	<ul>
		{#each suggestions as s, i (i)}
			<li class:off={!addable[i]}>
				<input
					id={`${uid}-${i}`}
					type="checkbox"
					checked={checked[i]}
					onchange={(ev) => toggle(i, ev.currentTarget.checked)}
				/>
				<div class="who">
					<label for={`${uid}-${i}`} class="name">{s.name}</label>
					<!-- Outside the label: a click on a role must not tick the person. -->
					<span class="roles">
						{#each s.roles as r (r)}
							<button
								type="button"
								class="chip"
								class:free={isFree(r)}
								aria-pressed={roleOn(i, r)}
								title={(roleOn(i, r) ? `Drop "${r}"` : `Keep "${r}"`) +
									(isFree(r) ? FREE_HINT : "")}
								onclick={() => togglePersonRole(i, r)}
							>
								{r}
							</button>
						{/each}
					</span>
				</div>
			</li>
		{/each}
	</ul>

	<div class="actions">
		<button type="button" class="secondary" onclick={onDismiss}>Dismiss</button>
		<button type="button" disabled={count === 0} onclick={add}>
			Add {count}
			{count === 1 ? "artist" : "artists"}
		</button>
	</div>
</div>

<style lang="scss">
	.suggestions {
		display: flex;
		flex-flow: column;
		gap: 8px;
		min-width: 0;
	}

	.bulk {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 4px 12px;
		font-size: 13px;

		.count {
			color: $text-color-accent;
			margin-right: auto;
		}

		.link {
			padding: 2px 0;
			font-size: 13px;
			text-decoration: underline;

			&:disabled {
				opacity: 0.4;
				cursor: default;
			}

			@media (hover: hover) {
				&:not(:disabled):hover {
					color: $text-color-accent;
				}
			}
		}
	}

	ul {
		display: flex;
		flex-flow: column;
		max-height: 360px;
		overflow-y: auto;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	li {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		padding: 6px 0;
		border-bottom: 1px solid $bg-color-accent;

		&:last-child {
			border-bottom: 0;
		}

		input {
			flex: 0 0 auto;
			width: 16px;
			height: 16px;
			margin: 2px 0 0;
			padding: 0;
		}
	}

	li.off .name {
		opacity: 0.6;
	}

	.who {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 4px 8px;
		min-width: 0;
	}

	.name {
		font-size: 14px;
		font-weight: bold;
		overflow-wrap: anywhere;
		cursor: pointer;
	}

	.all-roles {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 4px;
		padding-bottom: 6px;
		border-bottom: 1px solid $bg-color-accent;

		.hint {
			margin-right: 4px;
			font-size: 13px;
			color: $text-color-accent;
		}

		.n {
			font-size: 10px;
			opacity: 0.75;
		}
	}

	.roles {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}

	// Kept roles look like the editor's held ones (filled); dropped ones are
	// outlined and struck through.
	.chip {
		width: auto;
		padding: 1px 8px;
		border: 1px solid $text-color-accent;
		border-radius: 10px;
		background-color: transparent;
		color: $text-color-accent;
		font-size: 12px;
		text-decoration: line-through;

		@media (hover: hover) {
			&:hover {
				border-color: $text-color;
			}
		}

		&[aria-pressed="true"] {
			border-color: $text-color;
			background-color: $text-color;
			color: $bg-color;
			text-decoration: none;
		}

		&.free {
			border-color: $warn;
			color: $warn;

			&[aria-pressed="true"] {
				background-color: $warn;
				color: black;
			}
		}
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		justify-content: flex-end;
		gap: 8px;

		button {
			width: auto;
			min-width: 100px;
		}
	}
</style>
