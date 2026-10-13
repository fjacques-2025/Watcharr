<script lang="ts">
	import type { LiveShow, LiveShowRequest } from "@/types";
	import { errorText } from "./api";
	import { fromLocalInput, toLocalInput } from "./date";

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
	let saving = $state(false);
	let error: string | undefined = $state();

	function isHttpUrl(s: string) {
		try {
			const u = new URL(s);
			return u.protocol === "http:" || u.protocol === "https:";
		} catch {
			return false;
		}
	}

	// Mirrors the server's obvious rules, so a typo is caught before a round
	// trip; the server still has the last word.
	function validate(): string | undefined {
		if (!title.trim()) return "A title is required.";
		if (!fromLocalInput(date)) return "A date and time are required.";
		if (url.trim() && !isHttpUrl(url.trim()))
			return "The link must be a full http(s) address.";
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
			});
		} catch (err) {
			console.error("LiveShowForm: submit failed", err);
			error = errorText(err);
		} finally {
			saving = false;
		}
	}
</script>

<form class="live-form" onsubmit={submit} novalidate>
	<label>
		<span>Title</span>
		<input type="text" bind:value={title} maxlength={300} required />
	</label>
	<label>
		<span>Date and time</span>
		<input type="datetime-local" bind:value={date} required />
	</label>
	<label>
		<span>Venue</span>
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

		// Global inputs are black-bordered on any theme; follow the textarea's
		// lead so they stay visible in dark mode.
		input {
			color: $text-color;
			background-color: transparent;
			border-color: $text-color;
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

		.error {
			padding: 8px 10px;
			border-radius: 8px;
			color: white;
			background-color: $error;
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
