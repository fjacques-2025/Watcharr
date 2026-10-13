<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import { page } from "$app/state";
	import BackButton from "@/lib/generic/BackButton.svelte";
	import Error from "@/lib/Error.svelte";
	import Icon from "@/lib/Icon.svelte";
	import Spinner from "@/lib/Spinner.svelte";
	import LivePoster from "@/lib/live/LivePoster.svelte";
	import LiveShowForm from "@/lib/live/LiveShowForm.svelte";
	import {
		deleteShow,
		errorText,
		getShow,
		toRequest,
		updateShow,
	} from "@/lib/live/api";
	import { formatShowDate } from "@/lib/live/date";
	import { notify } from "@/lib/util/notify";
	import type {
		LiveCredit,
		LiveShow,
		LiveShowRequest,
		LiveShowState,
	} from "@/types";

	const stateLabels: Record<LiveShowState, string> = {
		upcoming: "Upcoming",
		seen: "Seen",
		missed: "Didn't go",
	};

	let id = $derived(Number(page.params.id));
	let show: LiveShow | undefined = $state();
	let loadError: unknown = $state();
	let editing = $state(false);
	let confirmingDelete = $state(false);
	let busy = $state(false);

	// Credits by role, roles in the order they first appear, names in order.
	let creditGroups = $derived.by(() => {
		const groups: [string, LiveCredit[]][] = [];
		for (const c of show?.credits ?? []) {
			const g = groups.find(([role]) => role === c.role);
			if (g) g[1].push(c);
			else groups.push([c.role, [c]]);
		}
		return groups;
	});

	async function load(forId: number) {
		loadError = undefined;
		try {
			const s = await getShow(forId);
			// Ignore a late answer for a show we've navigated away from.
			if (forId === id) show = s;
		} catch (err) {
			console.error("live: load show failed", err);
			if (forId === id) loadError = err;
		}
	}

	// Also covers moving from one show to another without leaving the route.
	$effect(() => {
		const forId = id;
		show = undefined;
		editing = false;
		confirmingDelete = false;
		load(forId);
	});

	async function save(body: LiveShowRequest) {
		if (!show) return;
		// The form doesn't own `missed`; keep whatever it was.
		show = await updateShow(show.id, { ...body, missed: show.missed });
		editing = false;
	}

	async function toggleMissed() {
		if (!show || busy) return;
		busy = true;
		try {
			show = await updateShow(show.id, {
				...toRequest(show),
				missed: !show.missed,
			});
		} catch (err) {
			console.error("live: toggle missed failed", err);
			notify({ type: "error", text: `Failed to update: ${errorText(err)}` });
		} finally {
			busy = false;
		}
	}

	async function remove() {
		if (!show || busy) return;
		busy = true;
		try {
			await deleteShow(show.id);
			notify({ type: "success", text: "Show deleted" });
			// Replace: Back must not lead to the page of a show that's gone.
			goto(resolve("/live"), { replaceState: true });
		} catch (err) {
			console.error("live: delete failed", err);
			notify({ type: "error", text: `Failed to delete: ${errorText(err)}` });
		} finally {
			busy = false;
			confirmingDelete = false;
		}
	}
</script>

<svelte:head>
	<title>{show?.title ?? "Live show"}</title>
</svelte:head>

<div class="content">
	<div class="inner">
		<BackButton />

		{#if loadError}
			<Error
				pretty="Failed to load this show!"
				error={loadError}
				onRetry={() => {
					load(id);
				}}
			/>
		{:else if !show}
			<Spinner />
		{:else if editing}
			<h2 class="title">Edit show</h2>
			<LiveShowForm
				initial={show}
				submitLabel="Save"
				onSubmit={save}
				onCancel={() => (editing = false)}
			/>
		{:else}
			<div class="top">
				{#if show.poster}
					<div class="poster"><LivePoster poster={show.poster} /></div>
				{/if}
				<div class="info">
					<div class="state" data-state={show.state}>
						{stateLabels[show.state]}
					</div>
					<h2 class="title">{show.title}</h2>
					<p class="when">{formatShowDate(show.date)}</p>
					{#if show.venue}
						<p class="venue">{show.venue}</p>
					{/if}
					{#if show.url}
						<p class="link">
							<a
								href={show.url}
								target="_blank"
								rel="external noreferrer noopener"
							>
								<Icon i="website" wh={16} />
								<span>{show.url}</span>
							</a>
						</p>
					{/if}
				</div>
			</div>
			{#if creditGroups.length}
				<dl class="credits">
					{#each creditGroups as [role, people] (role)}
						<div>
							<dt>{role}</dt>
							<dd>
								{#each people as c, i (`${c.personId}-${i}`)}
									<a href={resolve(`/live/people/${c.personId}`)}>{c.name}</a
									>{#if i < people.length - 1}<span class="sep">,</span>{/if}
								{/each}
							</dd>
						</div>
					{/each}
				</dl>
			{/if}
			{#if show.description}
				<p class="description">{show.description}</p>
			{/if}

			<div class="actions">
				<button onclick={() => (editing = true)} disabled={busy}>
					<Icon i="pencil" wh={16} /> Edit
				</button>
				<!-- Only once the date has passed: not going to a show still to
				  come isn't a thing yet. -->
				{#if show.state !== "upcoming"}
					<button
						onclick={toggleMissed}
						disabled={busy}
						aria-pressed={show.missed}
						title="For a show you booked but didn't attend (cancelled, couldn't make it…)"
					>
						{show.missed ? "Undo “Didn't go”" : "Didn't go"}
					</button>
				{/if}
				{#if confirmingDelete}
					<button class="danger" onclick={remove} disabled={busy}>
						Confirm delete
					</button>
					<button class="secondary" onclick={() => (confirmingDelete = false)}>
						Cancel
					</button>
				{:else}
					<button
						class="danger"
						onclick={() => (confirmingDelete = true)}
						disabled={busy}
					>
						<Icon i="trash" wh={16} /> Delete
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
			max-width: 600px;
			padding: 0 15px 60px 15px;
		}
	}

	.top {
		display: flex;
		flex-flow: row;
		align-items: flex-start;
		gap: 16px;

		.poster {
			flex: 0 0 140px;
			width: 140px;

			@media (max-width: 420px) {
				flex-basis: 100px;
				width: 100px;
			}
		}

		.info {
			flex: 1 1 auto;
			min-width: 0;
		}
	}

	.state {
		display: inline-block;
		margin-bottom: 6px;
		padding: 1px 8px;
		border: 1px solid $bg-color-accent;
		border-radius: 10px;
		font-size: 12px;
		color: $text-color-accent;
	}

	.title {
		margin-bottom: 6px;
		overflow-wrap: anywhere;
	}

	.when {
		font-weight: bold;
	}

	.venue {
		color: $text-color-accent;
	}

	.link a {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		margin-top: 8px;
		fill: currentColor;
		text-decoration: underline;
		overflow-wrap: anywhere;
	}

	.credits {
		display: flex;
		flex-flow: column;
		gap: 6px;
		margin: 16px 0 0 0;

		div {
			display: flex;
			flex-flow: row wrap;
			gap: 0 8px;
		}

		dt {
			color: $text-color-accent;
		}

		dd {
			margin: 0;
			overflow-wrap: anywhere;

			a {
				color: $text-color;
				text-decoration: underline;
				text-decoration-color: $bg-color-accent;
				text-underline-offset: 3px;

				@media (hover: hover) {
					&:hover {
						text-decoration-color: currentColor;
					}
				}
			}
		}
	}

	.description {
		margin-top: 14px;
		// Keep the paragraphs the description was typed with.
		white-space: pre-line;
		overflow-wrap: anywhere;
	}

	.actions {
		display: flex;
		flex-flow: row wrap;
		gap: 10px;
		margin-top: 22px;

		button {
			gap: 6px;
			width: auto;
		}
	}
</style>
