<script lang="ts">
	import { onMount } from "svelte";
	import { SvelteSet } from "svelte/reactivity";
	import { resolve } from "$app/paths";
	import BackButton from "@/lib/generic/BackButton.svelte";
	import PerformancePicker from "@/lib/live/PerformancePicker.svelte";
	import {
		createShow,
		errorText,
		listShows,
		prefillFromLink,
	} from "@/lib/live/api";
	import { fromLocalInput } from "@/lib/live/date";
	import { eachLimited, linkKey, parsePastedLinks } from "@/lib/live/links";

	// How many pages are read at once: enough to go quickly through a season's
	// list, few enough to stay polite with the theatre's site.
	const PARALLEL_PREFILLS = 3;

	type RowStatus = "loading" | "ready" | "error" | "saving" | "saved";

	interface Row {
		key: number;
		/** As pasted. */
		link: string;
		status: RowStatus;
		/** The prefill's or the save's error, from the server. */
		error: string;
		title: string;
		venue: string;
		url: string;
		description: string;
		posterUrl: string;
		/** datetime-local value. */
		date: string;
		performances: string[];
		dateRange: { start: string; end: string } | null;
		include: boolean;
		/** A show with the same link is already in the notebook. */
		known: boolean;
		posterFailed: boolean;
	}

	let text = $state("");
	let rows: Row[] = $state([]);
	let ignored: string[] = $state([]);
	let duplicates = $state(0);
	let filling = $state(false);
	let saving = $state(false);

	// Links of the shows already in the notebook, by linkKey. Loaded once; a
	// failure only loses the "already there" marks.
	const known = new SvelteSet<string>();
	let knownLoaded: Promise<void> | undefined;
	// Row keys stay unique across fills, so a new list never reuses the
	// previous one's elements.
	let nextKey = 0;

	let saveable = $derived(rows.filter((r) => isSaveable(r)));
	let incomplete = $derived(
		rows.filter(
			(r) =>
				r.include &&
				r.status !== "saved" &&
				r.status !== "loading" &&
				!isSaveable(r),
		).length,
	);
	let savedCount = $derived(rows.filter((r) => r.status === "saved").length);
	let done = $derived(savedCount > 0 && saveable.length === 0 && !saving);

	// Hoisted, so the $derived above may call it.
	function isSaveable(r: Row) {
		return (
			r.include &&
			(r.status === "ready" || r.status === "error") &&
			r.title.trim() !== "" &&
			fromLocalInput(r.date) !== undefined
		);
	}

	function loadKnown() {
		knownLoaded ??= listShows()
			.then((shows) => {
				for (const s of shows) {
					const k = linkKey(s.url);
					if (k) known.add(k);
				}
			})
			.catch((err) => {
				console.error("live bulk: list shows failed", err);
			});
		return knownLoaded;
	}

	onMount(() => {
		loadKnown();
	});

	function isKnown(...links: string[]) {
		return links.some((l) => {
			const k = linkKey(l);
			return k !== "" && known.has(k);
		});
	}

	async function fill() {
		if (filling || saving) return;
		const parsed = parsePastedLinks(text);
		ignored = parsed.ignored;
		duplicates = parsed.duplicates;
		rows = parsed.links.map((link) => ({
			key: nextKey++,
			link,
			status: "loading",
			error: "",
			title: "",
			venue: "",
			url: link,
			description: "",
			posterUrl: "",
			date: "",
			performances: [],
			dateRange: null,
			include: true,
			known: false,
			posterFailed: false,
		}));
		if (!rows.length) return;
		filling = true;
		try {
			await loadKnown();
			// Indexes, so each answer lands on its own row whatever the order
			// they come back in.
			await eachLimited(
				rows.map((_, i) => i),
				PARALLEL_PREFILLS,
				prefillRow,
			);
		} finally {
			filling = false;
		}
	}

	async function prefillRow(i: number) {
		const row = rows[i];
		try {
			const r = await prefillFromLink(row.link);
			row.title = r.title;
			row.venue = r.venue;
			row.url = r.url || row.link;
			row.description = r.description;
			row.posterUrl = r.posterUrl;
			row.performances = r.performances ?? [];
			row.dateRange = r.dateRange;
			row.status = "ready";
		} catch (err) {
			console.error("live bulk: prefill failed", row.link, err);
			// The row stays usable: the title and date can be typed by hand.
			row.error = errorText(err);
			row.status = "error";
		}
		row.known = isKnown(row.link, row.url);
		// Seeing a show twice happens, so it's only unchecked, not removed.
		if (row.known) row.include = false;
	}

	async function save() {
		if (saving || filling) return;
		saving = true;
		try {
			// One at a time: each save downloads a poster on the server.
			for (const row of saveable) {
				row.status = "saving";
				row.error = "";
				try {
					const s = await createShow({
						title: row.title.trim(),
						venue: row.venue.trim(),
						url: row.url.trim(),
						description: row.description.trim(),
						date: fromLocalInput(row.date)!,
						posterUrl: row.posterUrl.trim(),
					});
					row.status = "saved";
					const k = linkKey(s.url);
					if (k) known.add(k);
				} catch (err) {
					console.error("live bulk: save failed", row.link, err);
					row.error = errorText(err);
					// "ready" puts it back among the rows to save.
					row.status = "ready";
				}
			}
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Add several shows</title>
</svelte:head>

<div class="content">
	<div class="inner">
		<div class="head">
			<BackButton />
			<h2>Add several shows</h2>
		</div>

		<div class="paste">
			<label for="live-bulk-links">One link per line</label>
			<textarea
				id="live-bulk-links"
				bind:value={text}
				rows={6}
				placeholder="https://"
				autocomplete="off"
				spellcheck="false"></textarea>
			<button
				type="button"
				class="fill"
				onclick={fill}
				disabled={filling || saving || !text.trim()}
				aria-busy={filling}
			>
				{#if filling}<span class="mini-spinner" aria-hidden="true"></span>{/if}
				Fill
			</button>
			{#if ignored.length}
				<div class="note">
					Not an http(s) link, ignored:
					<ul>
						{#each ignored as line, i (i)}
							<li>{line}</li>
						{/each}
					</ul>
				</div>
			{/if}
			{#if duplicates}
				<p class="help">
					{duplicates} repeated link{duplicates > 1 ? "s" : ""} skipped.
				</p>
			{/if}
		</div>

		{#if rows.length}
			<ol class="rows">
				{#each rows as row (row.key)}
					{@const locked =
						row.status === "loading" ||
						row.status === "saving" ||
						row.status === "saved"}
					<li class="row" data-status={row.status} class:off={!row.include}>
						<div class="poster">
							{#if row.posterUrl && !row.posterFailed}
								<img
									src={row.posterUrl}
									alt=""
									referrerpolicy="no-referrer"
									onerror={() => (row.posterFailed = true)}
								/>
							{/if}
						</div>
						<div class="fields">
							<div class="top">
								<label class="include">
									<input
										type="checkbox"
										bind:checked={row.include}
										disabled={locked}
									/>
									Include
								</label>
								<a
									class="link"
									href={row.url || row.link}
									target="_blank"
									rel="external noreferrer noopener">{row.url || row.link}</a
								>
							</div>

							{#if row.known}
								<p class="known">Already in your notebook</p>
							{/if}

							<label>
								<span>Title</span>
								<input
									type="text"
									bind:value={row.title}
									maxlength={300}
									disabled={locked}
								/>
							</label>
							<div class="pair">
								<label>
									<span>Theatre</span>
									<input
										type="text"
										bind:value={row.venue}
										maxlength={200}
										disabled={locked}
									/>
								</label>
								<div class="date-field">
									<label>
										<span>Date and time</span>
										<input
											type="datetime-local"
											bind:value={row.date}
											required
											disabled={locked}
										/>
									</label>
									{#if !locked}
										<PerformancePicker
											performances={row.performances}
											dateRange={row.dateRange}
											bind:date={row.date}
										/>
									{/if}
								</div>
							</div>

							{#if row.status === "loading"}
								<p class="status">
									<span class="mini-spinner" aria-hidden="true"></span>
									Reading the page…
								</p>
							{:else if row.status === "saving"}
								<p class="status">
									<span class="mini-spinner" aria-hidden="true"></span>
									Saving…
								</p>
							{:else if row.status === "saved"}
								<p class="status ok">Saved</p>
							{:else if row.error}
								<p class="status err" role="alert">
									{row.error}
									{#if row.posterUrl}
										<button
											type="button"
											class="plain drop-poster"
											onclick={() => {
												row.posterUrl = "";
												row.error = "";
											}}
											disabled={saving}>Save without the poster</button
										>
									{/if}
								</p>
							{:else if row.include && !row.title.trim()}
								<p class="status">A title is required.</p>
							{:else if row.include && !fromLocalInput(row.date)}
								<p class="status">Pick a date and time.</p>
							{/if}
						</div>
					</li>
				{/each}
			</ol>

			<div class="actions">
				{#if incomplete}
					<p class="help">
						{incomplete} checked show{incomplete > 1 ? "s" : ""} still need{incomplete >
						1
							? ""
							: "s"} a title and a date.
					</p>
				{/if}
				{#if done}
					<p class="help">
						{savedCount} show{savedCount > 1 ? "s" : ""} added.
					</p>
					<a class="btn" href={resolve("/live")}>Back to my shows</a>
				{:else}
					<button
						type="button"
						onclick={save}
						disabled={saving || filling || !saveable.length}
						aria-busy={saving}
					>
						{#if saving}<span class="mini-spinner" aria-hidden="true"
							></span>{/if}
						Save {saveable.length} show{saveable.length === 1 ? "" : "s"}
					</button>
				{/if}
			</div>
		{/if}
	</div>
</div>

<style lang="scss">
	.content {
		display: flex;
		width: 100%;
		justify-content: center;
		margin-top: -18px;

		.inner {
			width: 100%;
			max-width: 800px;
			padding: 0 15px 60px 15px;
		}
	}

	.head {
		display: flex;
		align-items: center;
		gap: 10px;
		margin-bottom: 16px;

		:global(button.back-button) {
			margin-bottom: 0;
			flex: 0 0 auto;
		}
	}

	// Global inputs are black-bordered on any theme; keep them visible in
	// dark mode.
	input[type="text"],
	input[type="datetime-local"] {
		color: $text-color;
		background-color: transparent;
		border-color: $text-color;

		:global(:root.theme-dark) & {
			color-scheme: dark;
		}

		&:disabled {
			opacity: 0.6;
		}
	}

	.help {
		font-size: 13px;
		color: $text-color-accent;
	}

	.paste {
		display: flex;
		flex-flow: column;
		gap: 6px;

		label {
			font-size: 14px;
			font-weight: bold;
		}

		textarea {
			resize: vertical;
			font: inherit;
			font-size: 14px;
		}

		.fill {
			gap: 6px;
			width: auto;
			min-width: 110px;
			align-self: flex-end;
		}

		.note {
			font-size: 13px;
			color: $warn;

			ul {
				margin: 2px 0 0 18px;
				word-break: break-all;
			}
		}
	}

	.mini-spinner {
		display: inline-block;
		flex: 0 0 auto;
		width: 12px;
		height: 12px;
		border: 2px solid currentColor;
		border-right-color: transparent;
		border-radius: 50%;
		vertical-align: -1px;
		animation: live-bulk-spin 700ms linear infinite;
	}

	@keyframes live-bulk-spin {
		to {
			transform: rotate(360deg);
		}
	}

	.rows {
		display: flex;
		flex-flow: column;
		gap: 10px;
		margin-top: 18px;
		padding: 0;
		list-style: none;
	}

	.row {
		display: flex;
		flex-flow: row;
		align-items: flex-start;
		gap: 12px;
		padding: 10px;
		border: 1px solid $bg-color-accent;
		border-radius: 8px;

		&.off .fields > :not(.top):not(.known):not(.status) {
			opacity: 0.6;
		}

		&[data-status="saved"] {
			border-color: $success;
		}

		.poster {
			flex: 0 0 auto;
			width: 70px;
			aspect-ratio: 2 / 3;
			border-radius: 6px;
			overflow: hidden;
			background-color: $bg-color-accent;

			img {
				display: block;
				width: 100%;
				height: 100%;
				object-fit: cover;
			}
		}

		.fields {
			display: flex;
			flex-flow: column;
			gap: 8px;
			flex: 1 1 auto;
			min-width: 0;

			label:not(.include) {
				display: flex;
				flex-flow: column;
				gap: 4px;

				span {
					font-size: 13px;
					font-weight: bold;
				}
			}
		}

		.top {
			display: flex;
			align-items: center;
			gap: 10px;
			min-width: 0;

			.include {
				display: flex;
				align-items: center;
				gap: 5px;
				flex: 0 0 auto;
				font-size: 14px;
				font-weight: bold;

				input {
					width: auto;
				}
			}

			.link {
				min-width: 0;
				overflow: hidden;
				text-overflow: ellipsis;
				white-space: nowrap;
				font-size: 13px;
				color: $text-color-accent;
			}
		}

		.known {
			align-self: flex-start;
			padding: 2px 8px;
			border-radius: 10px;
			font-size: 12px;
			font-weight: bold;
			color: black;
			background-color: $warn;
		}

		.pair {
			display: grid;
			grid-template-columns: 1fr 1fr;
			gap: 8px;
		}

		.date-field {
			display: flex;
			flex-flow: column;
			gap: 4px;
			min-width: 0;
		}

		.status {
			display: flex;
			flex-flow: row wrap;
			align-items: center;
			gap: 6px;
			font-size: 13px;
			color: $text-color-accent;

			&.ok {
				color: $success;
				font-weight: bold;
			}

			&.err {
				color: $error;
			}

			.drop-poster {
				padding: 0;
				font-size: 13px;
				text-decoration: underline;
			}
		}
	}

	.actions {
		display: flex;
		flex-flow: column;
		align-items: flex-end;
		gap: 8px;
		margin-top: 16px;

		button,
		.btn {
			gap: 6px;
			width: auto;
			min-width: 140px;
		}
	}

	// Narrow screens: poster above, fields full width, one field per line.
	@media (max-width: 560px) {
		.row {
			flex-flow: column;

			.poster {
				width: 60px;
			}

			.fields {
				width: 100%;
			}

			.pair {
				grid-template-columns: 1fr;
			}
		}

		.actions {
			align-items: stretch;
		}
	}
</style>
