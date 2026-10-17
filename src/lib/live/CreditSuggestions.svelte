<script lang="ts">
	// People read from a page or a pasted list, offered for the Artists
	// editor: one line each, ticked when the server thinks they belong there
	// (technical and admin people come unticked). Read-only: names and roles
	// are fixed up in the editor once added.
	import type { LiveCreditSuggestion } from "@/types";

	interface Props {
		suggestions: LiveCreditSuggestion[];
		onAdd: (picked: { name: string; roles: string[] }[]) => void;
		onDismiss: () => void;
	}

	let { suggestions, onAdd, onDismiss }: Props = $props();

	const uid = $props.id();

	// Starts from the server's choice; ticking overrides it until a new list
	// comes in.
	let checked = $derived(suggestions.map((s) => s.suggested));
	let count = $derived(checked.filter(Boolean).length);

	function setAll(value: boolean) {
		checked = suggestions.map(() => value);
	}

	function toggle(i: number, value: boolean) {
		checked = checked.map((c, j) => (j === i ? value : c));
	}

	function add() {
		const picked = suggestions
			.filter((_, i) => checked[i])
			.map((s) => ({ name: s.name, roles: [...s.roles] }));
		if (picked.length) onAdd(picked);
	}
</script>

<div class="suggestions">
	<div class="bulk">
		<span class="count">{count} of {suggestions.length} selected</span>
		<button
			type="button"
			class="plain link"
			disabled={count === suggestions.length}
			onclick={() => setAll(true)}
		>
			Select all
		</button>
		<button
			type="button"
			class="plain link"
			disabled={count === 0}
			onclick={() => setAll(false)}
		>
			None
		</button>
	</div>

	<ul>
		{#each suggestions as s, i (i)}
			<li>
				<input
					id={`${uid}-${i}`}
					type="checkbox"
					checked={checked[i]}
					onchange={(ev) => toggle(i, ev.currentTarget.checked)}
				/>
				<label for={`${uid}-${i}`} class:off={!checked[i]}>
					<span class="name">{s.name}</span>
					<span class="roles">
						{#each s.roles as r, j (j)}
							<span class="chip">{r}</span>
						{/each}
					</span>
				</label>
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

	label {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 4px 8px;
		min-width: 0;
		cursor: pointer;

		&.off {
			opacity: 0.6;
		}
	}

	.name {
		font-size: 14px;
		font-weight: bold;
		overflow-wrap: anywhere;
	}

	.roles {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}

	// The editor's look for a held role: its pressed default chips and its
	// custom ones are both filled, so these are too.
	.chip {
		padding: 1px 8px;
		border: 1px solid $text-color;
		border-radius: 10px;
		background-color: $text-color;
		color: $bg-color;
		font-size: 12px;
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
