<script lang="ts">
	// What a prefill read about dates, offered under a date field: a picker
	// when the page lists performances, a "Performances: …" hint when it only
	// gives the days they run over, nothing otherwise. The date stays the
	// caller's field: this only writes it on an explicit choice, or when the
	// page lists exactly one performance and the field is still empty.
	import { untrack } from "svelte";
	import { formatDayRange, formatShowDate, toLocalInput } from "./date";

	interface Props {
		/** RFC3339 date-times from the prefill; [] when none. */
		performances: string[];
		/** Days without a time ("YYYY-MM-DD"), used only without performances. */
		dateRange?: { start: string; end: string } | null;
		/** The caller's datetime-local value ("YYYY-MM-DDTHH:mm"). */
		date: string;
	}

	let { performances, dateRange = null, date = $bindable() }: Props = $props();

	// Hoisted, so the $derived below may call it.
	function isPast(rfc3339: string) {
		return new Date(rfc3339).getTime() < Date.now();
	}

	// Anything the browser can't read would show as an empty option.
	let valid = $derived(performances.filter((p) => toLocalInput(p) !== ""));
	// Both halves chronological: upcoming first, as that's what is usually
	// booked, then the past ones.
	let upcoming = $derived(valid.filter((p) => !isPast(p)));
	let past = $derived(valid.filter((p) => isPast(p)));
	// The picker shows the performance the date field currently holds, if any;
	// a hand-typed date that matches none leaves it on its prompt.
	let picked = $derived(
		(date && valid.find((p) => toLocalInput(p) === date)) || "",
	);
	let rangeHint = $derived(
		!valid.length && dateRange
			? formatDayRange(dateRange.start, dateRange.end)
			: "",
	);

	// A lone performance is taken as the date, only into an empty field, and
	// only when the list changes: clearing the field later must not refill it.
	$effect(() => {
		const list = valid;
		untrack(() => {
			if (list.length === 1 && !date) date = toLocalInput(list[0]);
		});
	});

	function pick(ev: Event) {
		const value = (ev.currentTarget as HTMLSelectElement).value;
		// An explicit choice, so it replaces whatever the field held.
		if (value) date = toLocalInput(value);
	}
</script>

{#if valid.length}
	<label class="performance">
		<span>Choose your performance</span>
		<select value={picked} onchange={pick}>
			<option value="" disabled>
				{valid.length} performance{valid.length > 1 ? "s" : ""} on the page
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
				{#each valid as p (p)}
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

<style lang="scss">
	.help {
		font-size: 13px;
		color: $text-color-accent;
	}

	.performance {
		display: flex;
		flex-flow: column;
		gap: 5px;
		margin-top: 4px;

		span {
			font-size: 13px;
			font-weight: bold;
		}
	}

	// No global select style: match the inputs around it.
	select {
		width: 100%;
		padding: 7px 10px;
		border: 2px solid $text-color;
		border-radius: 5px;
		font: inherit;
		color: $text-color;
		background-color: transparent;

		:global(:root.theme-dark) & {
			color-scheme: dark;
		}
	}
</style>
