<script lang="ts">
	import { onDestroy, onMount } from "svelte";
	import Icon from "@/lib/Icon.svelte";
	import type { LivePersonSummary } from "@/types";
	import {
		listPeople,
		searchTmdbPeople,
		type ArtistPick,
		type TmdbPersonHit,
	} from "./api";

	interface Props {
		/** What the field holds; free text is fine, picking replaces it. */
		value: string;
		/** Only the TMDB group (linking an existing artist to TMDB). */
		tmdbOnly?: boolean;
		/** Focus on mount and search what's already in the field. */
		autofocus?: boolean;
		placeholder?: string;
		ariaLabel: string;
		id?: string;
		/** A suggestion was chosen. Never called without the user's choice:
		 *  homonyms are common, so nothing is ever linked automatically. */
		onPick: (p: ArtistPick) => void;
		/** The user typed (not a pick). */
		onType?: (value: string) => void;
	}

	let {
		value = $bindable(""),
		tmdbOnly = false,
		autofocus = false,
		placeholder = "",
		ariaLabel,
		id,
		onPick,
		onType,
	}: Props = $props();

	const MIN_CHARS = 2;
	const DELAY_MS = 300;
	const uid = $props.id();
	const listId = `${uid}-list`;

	type Option =
		| { kind: "notebook"; p: LivePersonSummary }
		| { kind: "tmdb"; p: TmdbPersonHit };

	let inputEl: HTMLInputElement | undefined = $state();
	let open = $state(false);
	let notebook: LivePersonSummary[] = $state([]);
	let tmdb: TmdbPersonHit[] = $state([]);
	let loadingNotebook = $state(false);
	let loadingTmdb = $state(false);
	let tmdbFailed = $state(false);
	let active = $state(-1);

	// A TMDB person already in the notebook is offered there, with its count:
	// listing it twice would only invite a second, unlinked pick.
	let tmdbShown = $derived(
		tmdb.filter(
			(h) => !notebook.some((n) => n.tmdbPersonId === h.tmdbPersonId),
		),
	);
	let options: Option[] = $derived([
		...notebook.map((p) => ({ kind: "notebook" as const, p })),
		...tmdbShown.map((p) => ({ kind: "tmdb" as const, p })),
	]);
	let loading = $derived(loadingNotebook || loadingTmdb);
	let showList = $derived(open && value.trim().length >= MIN_CHARS);

	let timer: ReturnType<typeof setTimeout> | undefined;
	let ctrl: AbortController | undefined;

	onMount(() => {
		if (!autofocus) return;
		inputEl?.focus();
		if (value.trim().length >= MIN_CHARS) {
			open = true;
			search(value.trim());
		}
	});

	onDestroy(() => {
		clearTimeout(timer);
		ctrl?.abort();
	});

	function schedule() {
		clearTimeout(timer);
		ctrl?.abort();
		const q = value.trim();
		if (q.length < MIN_CHARS) {
			open = false;
			notebook = [];
			tmdb = [];
			loadingNotebook = false;
			loadingTmdb = false;
			return;
		}
		open = true;
		timer = setTimeout(() => search(q), DELAY_MS);
	}

	function search(q: string) {
		ctrl?.abort();
		const c = new AbortController();
		ctrl = c;
		active = -1;
		// Two independent groups: a slow TMDB doesn't hold back the notebook.
		if (!tmdbOnly) {
			loadingNotebook = true;
			listPeople(q, c.signal)
				.then((r) => {
					if (!c.signal.aborted) notebook = r ?? [];
				})
				.catch((err) => {
					if (c.signal.aborted) return;
					console.warn("ArtistSuggest: notebook search failed", err);
					notebook = [];
				})
				.finally(() => {
					if (ctrl === c) loadingNotebook = false;
				});
		}
		loadingTmdb = true;
		tmdbFailed = false;
		searchTmdbPeople(q, c.signal)
			.then((r) => {
				if (!c.signal.aborted) tmdb = r.slice(0, 8);
			})
			.catch((err) => {
				if (c.signal.aborted) return;
				console.warn("ArtistSuggest: TMDB search failed", err);
				tmdb = [];
				tmdbFailed = true;
			})
			.finally(() => {
				if (ctrl === c) loadingTmdb = false;
			});
	}

	function close() {
		open = false;
		active = -1;
	}

	function pick(o: Option) {
		const p: ArtistPick = {
			name: o.p.name,
			tmdbPersonId: o.p.tmdbPersonId,
		};
		clearTimeout(timer);
		ctrl?.abort();
		loadingNotebook = false;
		loadingTmdb = false;
		value = p.name;
		close();
		onPick(p);
	}

	function oninput() {
		onType?.(value);
		schedule();
	}

	function onkeydown(ev: KeyboardEvent) {
		if (ev.isComposing) return;
		switch (ev.key) {
			case "ArrowDown":
				if (!showList) {
					if (value.trim().length >= MIN_CHARS) {
						open = true;
						if (!options.length && !loading) search(value.trim());
					}
					return;
				}
				ev.preventDefault();
				if (options.length) active = (active + 1) % options.length;
				break;
			case "ArrowUp":
				if (!showList) return;
				ev.preventDefault();
				if (options.length)
					active = active <= 0 ? options.length - 1 : active - 1;
				break;
			case "Enter":
				if (!showList) return;
				// Inside a form, Enter would submit it: with the list open it
				// picks the highlighted suggestion, or just closes the list.
				ev.preventDefault();
				if (active >= 0 && options[active]) pick(options[active]);
				else close();
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

	// Keep the highlighted option in view while arrowing through a long list.
	$effect(() => {
		if (active < 0) return;
		document.getElementById(optionId(active))?.scrollIntoView({
			block: "nearest",
		});
	});
</script>

<div class="artist-suggest">
	<input
		bind:this={inputEl}
		{id}
		type="text"
		bind:value
		{oninput}
		{onkeydown}
		onblur={close}
		{placeholder}
		aria-label={ariaLabel}
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
		maxlength={200}
	/>
	{#if showList}
		<!-- mousedown is cancelled so the input keeps focus (and the list stays
		     open) until the click lands: works for taps too. -->
		<div
			class="list"
			id={listId}
			role="listbox"
			aria-label="Suggestions"
			tabindex="-1"
			onmousedown={(e) => e.preventDefault()}
		>
			{#if !tmdbOnly && notebook.length}
				<div class="group" role="presentation">In your notebook</div>
				{#each notebook as p, i (p.id)}
					<!-- Keyboard is handled by the input (combobox pattern). -->
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<div
						class="option"
						id={optionId(i)}
						role="option"
						tabindex="-1"
						aria-selected={active === i}
						onclick={() => pick({ kind: "notebook", p })}
					>
						<span class="name">{p.name}</span>
						<span class="meta">
							{#if p.tmdbPersonId}<span class="tmdb-mark">TMDB</span>{/if}
							{p.showCount} show{p.showCount === 1 ? "" : "s"}
						</span>
					</div>
				{/each}
			{/if}
			{#if tmdbShown.length}
				<div class="group" role="presentation">TMDB</div>
				{#each tmdbShown as p, j (p.tmdbPersonId)}
					{@const i = notebook.length + j}
					<!-- Keyboard is handled by the input (combobox pattern). -->
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<div
						class="option"
						id={optionId(i)}
						role="option"
						tabindex="-1"
						aria-selected={active === i}
						onclick={() => pick({ kind: "tmdb", p })}
					>
						<span class="photo">
							{#if p.photo}
								<img src={p.photo} alt="" loading="lazy" />
							{:else}
								<Icon i="person" wh={18} />
							{/if}
						</span>
						<span class="name">{p.name}</span>
					</div>
				{/each}
			{/if}
			{#if loading}
				<div class="status" role="presentation">Searching…</div>
			{:else if tmdbFailed && !options.length}
				<div class="status" role="presentation">
					Couldn't search TMDB right now.
				</div>
			{:else if !options.length}
				<div class="status" role="presentation">
					{tmdbOnly
						? "No TMDB person found."
						: "No match: the name will be kept as typed."}
				</div>
			{/if}
		</div>
	{/if}
</div>

<style lang="scss">
	.artist-suggest {
		position: relative;
		width: 100%;
		min-width: 0;

		input {
			color: $text-color;
			background-color: transparent;
			border-color: $text-color;
		}
	}

	.list {
		position: absolute;
		z-index: 30;
		top: calc(100% + 4px);
		left: 0;
		right: 0;
		min-width: 220px;
		max-height: 320px;
		overflow-y: auto;
		padding: 4px 0;
		border: 1px solid $bg-color-accent;
		border-radius: 8px;
		background-color: $bg-color;
		box-shadow: 0 6px 20px rgba(0, 0, 0, 0.25);
	}

	.group {
		padding: 6px 12px 2px 12px;
		font-size: 12px;
		font-weight: bold;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: $text-color-accent;
	}

	.option {
		display: flex;
		align-items: center;
		gap: 10px;
		// Comfortable tap target on a phone.
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
			display: flex;
			align-items: center;
			gap: 6px;
			flex: 0 0 auto;
			font-size: 13px;
			color: $text-color-accent;
		}

		.photo {
			display: flex;
			align-items: center;
			justify-content: center;
			flex: 0 0 32px;
			width: 32px;
			height: 48px;
			overflow: hidden;
			border-radius: 4px;
			background-color: $bg-color-accent;
			fill: $text-color-accent;

			img {
				width: 100%;
				height: 100%;
				object-fit: cover;
			}
		}

		@media (hover: hover) {
			&:hover {
				background-color: $bg-color-accent;
			}
		}

		// The keyboard highlight: an accent edge on top of the tint, so it
		// stays distinct from a mere hover.
		&[aria-selected="true"] {
			background-color: $bg-color-accent;
			box-shadow: inset 3px 0 0 $accent-color-hover;
		}
	}

	.tmdb-mark {
		padding: 0 5px;
		border: 1px solid $bg-color-accent;
		border-radius: 6px;
		font-size: 11px;
	}

	.status {
		padding: 8px 12px;
		font-size: 13px;
		color: $text-color-accent;
	}
</style>
