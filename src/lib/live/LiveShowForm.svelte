<script lang="ts">
	import type { LiveShow, LiveShowRequest } from "@/types";
	import { errorText, prefillFromLink } from "./api";
	import {
		formatDayRange,
		formatShowDate,
		fromLocalInput,
		toLocalInput,
	} from "./date";

	interface Props {
		/** The show being edited; absent when adding one. */
		initial?: LiveShow;
		submitLabel: string;
		/** Throws on failure; its message is shown under the form. */
		onSubmit: (body: LiveShowRequest) => Promise<void>;
		onCancel?: () => void;
	}

	let { initial, submitLabel, onSubmit, onCancel }: Props = $props();

	// Fields are seeded once from `initial` and then owned by the form: an edit
	// in progress must not be overwritten if the parent's copy refreshes.
	// Declared before the $state below, which reads it.
	const seed = (() => initial)();

	let title = $state(seed?.title ?? "");
	let date = $state(seed ? toLocalInput(seed.date) : "");
	let venue = $state(seed?.venue ?? "");
	let url = $state(seed?.url ?? "");
	let description = $state(seed?.description ?? "");
	// Edit mode starts from the stored address: saving unchanged keeps the
	// poster, clearing it removes it.
	let posterUrl = $state(seed?.posterUrl ?? "");
	let saving = $state(false);
	let error: string | undefined = $state();

	// "Fill from link" — only offered when adding a show.
	let pageLink = $state("");
	let prefilling = $state(false);
	let prefillError: string | undefined = $state();
	// What the last fill read about dates: the performances to pick from, or
	// failing that the range they run over ("" when none). Cleared as soon as
	// the link changes, so they never describe another page.
	let performances: string[] = $state([]);
	let rangeHint = $state("");
	// Split once per fill, both halves kept chronological: upcoming first, as
	// that's what is usually booked, then the past ones.
	let upcoming = $derived(performances.filter((p) => !isPast(p)));
	let past = $derived(performances.filter((p) => isPast(p)));
	// The picker shows the performance the date field currently holds, if any;
	// a hand-typed date that matches none leaves it on its prompt.
	let pickedPerformance = $derived(
		(date && performances.find((p) => toLocalInput(p) === date)) || "",
	);

	// The preview hides itself once its address has failed to load; keyed by
	// address so editing the field gives the new one a chance.
	let failedPreview = $state("");
	let previewSrc = $derived(
		isHttpUrl(posterUrl.trim()) && failedPreview !== posterUrl.trim()
			? posterUrl.trim()
			: "",
	);

	// A function declaration (hoisted), so the $derived above may call it.
	function isHttpUrl(s: string) {
		try {
			const u = new URL(s);
			return u.protocol === "http:" || u.protocol === "https:";
		} catch {
			return false;
		}
	}

	// Hoisted, so the $derived above may call it.
	function isPast(rfc3339: string) {
		return new Date(rfc3339).getTime() < Date.now();
	}

	function clearDateChoices() {
		performances = [];
		rangeHint = "";
	}

	function pickPerformance(ev: Event) {
		const value = (ev.currentTarget as HTMLSelectElement).value;
		// An explicit choice, so it replaces whatever the field held.
		if (value) date = toLocalInput(value);
	}

	// Mirrors the server's obvious rules, so a typo is caught before a round
	// trip; the server still has the last word.
	function validate(): string | undefined {
		if (!title.trim()) return "A title is required.";
		if (!fromLocalInput(date)) return "A date and time are required.";
		if (url.trim() && !isHttpUrl(url.trim()))
			return "The link must be a full http(s) address.";
		if (posterUrl.trim() && !isHttpUrl(posterUrl.trim()))
			return "The poster image URL must be a full http(s) address.";
	}

	async function prefill() {
		if (prefilling) return;
		const link = pageLink.trim();
		if (!isHttpUrl(link)) {
			prefillError = "Paste the full http(s) address of the show's page.";
			return;
		}
		prefilling = true;
		prefillError = undefined;
		clearDateChoices();
		try {
			const r = await prefillFromLink(link);
			// Only empty fields: what the user typed wins over what was read.
			if (!title.trim() && r.title) title = r.title;
			if (!description.trim() && r.description) description = r.description;
			if (!venue.trim() && r.venue) venue = r.venue;
			if (!url.trim()) url = r.url || link;
			if (!posterUrl.trim() && r.posterUrl) posterUrl = r.posterUrl;
			// Pages often list many performances: offer them, the user picks
			// theirs. Only a lone one is taken as the date, and only into an
			// empty field. A range has no time, so it's just a hint.
			const perfs = (r.performances ?? []).filter(
				(p) => toLocalInput(p) !== "",
			);
			performances = perfs;
			if (perfs.length === 1 && !date) date = toLocalInput(perfs[0]);
			if (!perfs.length && r.dateRange)
				rangeHint = formatDayRange(r.dateRange.start, r.dateRange.end);
		} catch (err) {
			console.error("LiveShowForm: prefill failed", err);
			prefillError = errorText(err);
		} finally {
			prefilling = false;
		}
	}

	function prefillOnEnter(ev: KeyboardEvent) {
		if (ev.key !== "Enter" || ev.isComposing) return;
		ev.preventDefault();
		prefill();
	}

	async function submit(ev: SubmitEvent) {
		ev.preventDefault();
		if (saving) return;
		error = validate();
		if (error) return;
		saving = true;
		try {
			await onSubmit({
				title: title.trim(),
				venue: venue.trim(),
				url: url.trim(),
				description: description.trim(),
				date: fromLocalInput(date)!,
				posterUrl: posterUrl.trim(),
			});
		} catch (err) {
			console.error("LiveShowForm: submit failed", err);
			error = errorText(err);
		} finally {
			saving = false;
		}
	}
</script>

{#if !seed}
	<div class="prefill">
		<label for="live-prefill-link">Link to the show's page</label>
		<div class="row">
			<input
				id="live-prefill-link"
				type="url"
				bind:value={pageLink}
				oninput={clearDateChoices}
				onkeydown={prefillOnEnter}
				placeholder="https://"
				inputmode="url"
				autocomplete="off"
			/>
			<button
				type="button"
				class="fill"
				onclick={prefill}
				disabled={prefilling}
				aria-busy={prefilling}
			>
				{#if prefilling}<span class="mini-spinner" aria-hidden="true"
					></span>{/if}
				Fill from link
			</button>
		</div>
		{#if prefillError}
			<p class="error" role="alert">{prefillError}</p>
		{:else}
			<p class="help">
				Optional: fills in what the page says, you can then adjust.
			</p>
		{/if}
	</div>
{/if}

<form class="live-form" onsubmit={submit} novalidate>
	<label>
		<span>Title</span>
		<input type="text" bind:value={title} maxlength={300} required />
	</label>
	<div class="date-field">
		<label>
			<span>Date and time</span>
			<input type="datetime-local" bind:value={date} required />
		</label>
		{#if performances.length}
			<label class="performance">
				<span>Choose your performance</span>
				<select value={pickedPerformance} onchange={pickPerformance}>
					<option value="" disabled>
						{performances.length} performance{performances.length > 1
							? "s"
							: ""} on the page
					</option>
					{#if upcoming.length && past.length}
						<optgroup label="Upcoming">
							{#each upcoming as p (p)}
								<option value={p}>{formatShowDate(p)}</option>
							{/each}
						</optgroup>
						<optgroup label="Past">
							{#each past as p (p)}
								<option value={p}>{formatShowDate(p)} (past)</option>
							{/each}
						</optgroup>
					{:else}
						{#each performances as p (p)}
							<option value={p}
								>{formatShowDate(p)}{isPast(p) ? " (past)" : ""}</option
							>
						{/each}
					{/if}
				</select>
			</label>
			<p class="help">Or type any other date above.</p>
		{:else if rangeHint}
			<p class="help">Performances: {rangeHint}</p>
		{/if}
	</div>
	<label>
		<span>Theatre</span>
		<input type="text" bind:value={venue} maxlength={200} />
	</label>
	<label>
		<span>Link</span>
		<input
			type="url"
			bind:value={url}
			placeholder="https://"
			inputmode="url"
			autocomplete="off"
		/>
	</label>
	<div class="poster-field">
		<label>
			<span>Poster image URL</span>
			<input
				type="url"
				bind:value={posterUrl}
				placeholder="https://"
				inputmode="url"
				autocomplete="off"
			/>
		</label>
		<p class="help">
			Tip: right-click the poster on the theatre's page → Copy image address.
		</p>
		{#if previewSrc}
			<img
				class="preview"
				src={previewSrc}
				alt="Poster preview"
				referrerpolicy="no-referrer"
				onerror={() => (failedPreview = previewSrc)}
			/>
		{/if}
	</div>
	<label>
		<span>Description</span>
		<textarea bind:value={description} rows={6} maxlength={5000}></textarea>
	</label>

	{#if error}
		<p class="error" role="alert">{error}</p>
	{/if}

	<div class="actions">
		{#if onCancel}
			<button type="button" class="secondary" onclick={onCancel}>
				Cancel
			</button>
		{/if}
		<button type="submit" disabled={saving}>{submitLabel}</button>
	</div>
</form>

<style lang="scss">
	// Global inputs are black-bordered on any theme; follow the textarea's
	// lead so they stay visible in dark mode.
	.prefill input,
	.live-form input,
	.live-form select {
		color: $text-color;
		background-color: transparent;
		border-color: $text-color;
	}

	.help {
		font-size: 13px;
		color: $text-color-accent;
	}

	.error {
		padding: 8px 10px;
		border-radius: 8px;
		color: white;
		background-color: $error;
	}

	.prefill {
		display: flex;
		flex-flow: column;
		gap: 5px;
		margin-bottom: 18px;
		padding-bottom: 18px;
		border-bottom: 1px solid $bg-color-accent;

		label {
			font-size: 14px;
			font-weight: bold;
		}

		.row {
			display: flex;
			flex-flow: row;
			gap: 8px;

			input {
				flex: 1 1 auto;
				min-width: 0;
			}

			.fill {
				flex: 0 0 auto;
				gap: 6px;
				width: auto;
				white-space: nowrap;
			}
		}
	}

	.mini-spinner {
		display: inline-block;
		width: 12px;
		height: 12px;
		border: 2px solid currentColor;
		border-right-color: transparent;
		border-radius: 50%;
		animation: live-spin 700ms linear infinite;
	}

	@keyframes live-spin {
		to {
			transform: rotate(360deg);
		}
	}

	.live-form {
		display: flex;
		flex-flow: column;
		gap: 14px;

		label {
			display: flex;
			flex-flow: column;
			gap: 5px;

			span {
				font-size: 14px;
				font-weight: bold;
			}
		}

		input {
			// The app's theme is a class, not the OS setting, so the native
			// date picker (icon and popup) must be told explicitly.
			:global(:root.theme-dark) & {
				color-scheme: dark;
			}
		}

		textarea {
			resize: vertical;
			font: inherit;
		}

		.date-field {
			display: flex;
			flex-flow: column;
			gap: 5px;

			.performance {
				margin-top: 4px;

				span {
					font-size: 13px;
				}
			}

			// No global select style: match the inputs around it.
			select {
				width: 100%;
				padding: 7px 10px;
				border: 2px solid;
				border-radius: 5px;
				font: inherit;
				:global(:root.theme-dark) & {
					color-scheme: dark;
				}
			}
		}

		.poster-field {
			display: flex;
			flex-flow: column;
			gap: 5px;

			.preview {
				width: 90px;
				aspect-ratio: 2 / 3;
				object-fit: cover;
				border-radius: 6px;
				background-color: $bg-color-accent;
			}
		}

		.actions {
			display: flex;
			flex-flow: row;
			gap: 10px;
			justify-content: flex-end;

			button {
				width: auto;
				min-width: 110px;
			}
		}
	}
</style>
