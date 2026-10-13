<script lang="ts">
	import { onDestroy, tick } from "svelte";
	import Icon from "@/lib/Icon.svelte";
	import type { LiveFriendSummary } from "@/types";
	import { listFriends } from "./api";

	interface Props {
		friends: { id: number; name: string }[];
		/** Saves the full list of names; resolves false when it failed (the
		 *  caller has already said why). */
		onSave: (names: string[]) => Promise<boolean>;
	}

	let { friends, onSave }: Props = $props();

	const MAX_FRIENDS = 30;
	const DELAY_MS = 250;
	const uid = $props.id();
	const listId = `${uid}-list`;
	const inputId = `${uid}-input`;

	let value = $state("");
	let saving = $state(false);
	let open = $state(false);
	let loading = $state(false);
	let suggestions: LiveFriendSummary[] = $state([]);
	let active = $state(-1);
	let inputEl: HTMLInputElement | undefined = $state();

	let names = $derived(friends.map((f) => f.name));
	let full = $derived(friends.length >= MAX_FRIENDS);
	// Those already listed aren't offered again.
	let options = $derived(
		suggestions.filter((s) => !names.some((n) => same(n, s.name))),
	);
	let showList = $derived(open && value.trim().length > 0);

	let timer: ReturnType<typeof setTimeout> | undefined;
	let ctrl: AbortController | undefined;

	onDestroy(() => {
		clearTimeout(timer);
		ctrl?.abort();
	});

	/** Names are matched like the server does: accents and case ignored. */
	function fold(s: string) {
		return s.normalize("NFD").replace(/\p{M}/gu, "").toLocaleLowerCase().trim();
	}
	function same(a: string, b: string) {
		return fold(a) === fold(b);
	}

	function schedule() {
		clearTimeout(timer);
		ctrl?.abort();
		const q = value.trim();
		active = -1;
		if (!q) {
			open = false;
			suggestions = [];
			loading = false;
			return;
		}
		open = true;
		loading = true;
		timer = setTimeout(() => search(q), DELAY_MS);
	}

	function search(q: string) {
		ctrl?.abort();
		const c = new AbortController();
		ctrl = c;
		listFriends(q, c.signal)
			.then((r) => {
				if (!c.signal.aborted) suggestions = r ?? [];
			})
			.catch((err) => {
				if (c.signal.aborted) return;
				console.warn("LiveFriends: search failed", err);
				suggestions = [];
			})
			.finally(() => {
				if (ctrl === c) loading = false;
			});
	}

	function close() {
		open = false;
		active = -1;
	}

	async function add(name: string) {
		const n = name.trim();
		if (!n || saving) return;
		clearTimeout(timer);
		ctrl?.abort();
		loading = false;
		close();
		if (names.some((x) => same(x, n))) {
			value = "";
			return;
		}
		if (full) return;
		saving = true;
		try {
			if (await onSave([...names, n])) value = "";
		} finally {
			saving = false;
		}
		// Once the field is enabled again: ready for the next name.
		await tick();
		inputEl?.focus();
	}

	async function remove(i: number) {
		if (saving) return;
		saving = true;
		try {
			await onSave(names.filter((_, j) => j !== i));
		} finally {
			saving = false;
		}
	}

	function onkeydown(ev: KeyboardEvent) {
		if (ev.isComposing) return;
		switch (ev.key) {
			case "ArrowDown":
				if (!showList || !options.length) return;
				ev.preventDefault();
				active = (active + 1) % options.length;
				break;
			case "ArrowUp":
				if (!showList || !options.length) return;
				ev.preventDefault();
				active = active <= 0 ? options.length - 1 : active - 1;
				break;
			case "Enter":
				ev.preventDefault();
				add(active >= 0 && showList ? options[active].name : value);
				break;
			case "Escape":
				if (!showList) return;
				ev.preventDefault();
				ev.stopPropagation();
				close();
				break;
			case "Tab":
				close();
				break;
		}
	}

	function optionId(i: number) {
		return `${uid}-opt-${i}`;
	}
</script>

<div class="live-friends">
	{#if friends.length}
		<ul class="chips">
			{#each friends as f, i (f.id)}
				<li class="chip">
					<span class="name">{f.name}</span>
					<button
						class="plain remove"
						onclick={() => remove(i)}
						disabled={saving}
						aria-label="Remove {f.name}"
						title="Remove"
					>
						<Icon i="close" wh={14} />
					</button>
				</li>
			{/each}
		</ul>
	{/if}

	{#if !full}
		<div class="add">
			<label class="sr-only" for={inputId}>Add a friend</label>
			<div class="field">
				<input
					bind:this={inputEl}
					id={inputId}
					type="text"
					bind:value
					oninput={schedule}
					{onkeydown}
					onblur={close}
					disabled={saving}
					placeholder="Add a name"
					role="combobox"
					aria-autocomplete="list"
					aria-expanded={showList}
					aria-controls={listId}
					aria-activedescendant={showList && active >= 0
						? optionId(active)
						: undefined}
					autocomplete="off"
					autocapitalize="words"
					spellcheck="false"
					maxlength={100}
				/>
				{#if showList}
					<!-- mousedown is cancelled so the input keeps focus until the
					     click lands: works for taps too. -->
					<div
						class="list"
						id={listId}
						role="listbox"
						aria-label="Suggestions"
						tabindex="-1"
						onmousedown={(e) => e.preventDefault()}
					>
						{#each options as s, i (s.id)}
							<!-- Keyboard is handled by the input (combobox pattern). -->
							<!-- svelte-ignore a11y_click_events_have_key_events -->
							<div
								class="option"
								id={optionId(i)}
								role="option"
								tabindex="-1"
								aria-selected={active === i}
								onclick={() => add(s.name)}
							>
								<span class="name">{s.name}</span>
								<span class="meta">
									{s.showCount} show{s.showCount === 1 ? "" : "s"}
								</span>
							</div>
						{/each}
						{#if loading && !options.length}
							<div class="status" role="presentation">Searching…</div>
						{:else if !options.length}
							<div class="status" role="presentation">
								Enter adds “{value.trim()}”.
							</div>
						{/if}
					</div>
				{/if}
			</div>
			<button
				class="add-btn"
				onclick={() => add(value)}
				disabled={saving || !value.trim()}
			>
				Add
			</button>
		</div>
	{:else}
		<p class="note">That's the maximum ({MAX_FRIENDS}).</p>
	{/if}
</div>

<style lang="scss">
	.live-friends {
		display: flex;
		flex-flow: column;
		gap: 10px;
	}

	.chips {
		display: flex;
		flex-flow: row wrap;
		gap: 6px;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.chip {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		padding: 2px 2px 2px 10px;
		border: 1px solid $bg-color-accent;
		border-radius: 16px;
		font-size: 14px;

		.name {
			overflow-wrap: anywhere;
		}

		.remove {
			display: flex;
			align-items: center;
			justify-content: center;
			width: 28px;
			height: 28px;
			padding: 0;
			border-radius: 50%;
			color: $text-color-accent;
			fill: currentColor;

			@media (hover: hover) {
				&:hover:not(:disabled) {
					color: $text-color;
					background-color: $bg-color-accent;
				}
			}

			&:focus-visible {
				outline: 2px solid $text-color;
			}
		}
	}

	.add {
		display: flex;
		flex-flow: row;
		gap: 8px;

		.field {
			position: relative;
			flex: 1 1 auto;
			min-width: 0;
		}

		input {
			color: $text-color;
			background-color: transparent;
			border-color: $text-color;
		}

		.add-btn {
			width: auto;
			flex: 0 0 auto;
		}
	}

	.list {
		position: absolute;
		z-index: 30;
		top: calc(100% + 4px);
		left: 0;
		right: 0;
		max-height: 280px;
		overflow-y: auto;
		padding: 4px 0;
		border: 1px solid $bg-color-accent;
		border-radius: 8px;
		background-color: $bg-color;
		box-shadow: 0 6px 20px rgba(0, 0, 0, 0.25);
	}

	.option {
		display: flex;
		align-items: center;
		gap: 10px;
		min-height: 44px;
		padding: 6px 12px;
		cursor: pointer;
		color: $text-color;

		.name {
			flex: 1 1 auto;
			min-width: 0;
			overflow-wrap: anywhere;
		}

		.meta {
			flex: 0 0 auto;
			font-size: 13px;
			color: $text-color-accent;
		}

		@media (hover: hover) {
			&:hover {
				background-color: $bg-color-accent;
			}
		}

		// Keyboard highlight: an accent edge on top of the tint, distinct
		// from a mere hover.
		&[aria-selected="true"] {
			background-color: $bg-color-accent;
			box-shadow: inset 3px 0 0 $accent-color-hover;
		}
	}

	.status,
	.note {
		padding: 8px 12px;
		font-size: 13px;
		color: $text-color-accent;
	}

	.note {
		padding: 0;
	}

	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		margin: -1px;
		padding: 0;
		overflow: hidden;
		clip: rect(0 0 0 0);
		white-space: nowrap;
		border: 0;
	}
</style>
