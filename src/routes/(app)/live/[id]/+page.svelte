<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import { page } from "$app/state";
	import BackButton from "@/lib/generic/BackButton.svelte";
	import Error from "@/lib/Error.svelte";
	import Icon from "@/lib/Icon.svelte";
	import Spinner from "@/lib/Spinner.svelte";
	import MyThoughts from "@/lib/content/MyThoughts.svelte";
	import LiveFriends from "@/lib/live/LiveFriends.svelte";
	import LivePhotos from "@/lib/live/LivePhotos.svelte";
	import LivePosterControl from "@/lib/live/LivePosterControl.svelte";
	import LiveShowForm from "@/lib/live/LiveShowForm.svelte";
	import {
		deleteShow,
		errorText,
		getShow,
		posterFromPhoto,
		sortPhotos,
		toRequest,
		updateShow,
		updateSouvenir,
	} from "@/lib/live/api";
	import { formatShowDate } from "@/lib/live/date";
	import Rating from "@/lib/rating/Rating.svelte";
	import { notify } from "@/lib/util/notify";
	import type {
		LivePhoto,
		LivePosterSize,
		LiveShow,
		LiveShowRequest,
		LiveShowState,
		LiveSouvenirRequest,
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
	let posterSize: LivePosterSize | undefined = $state();

	// Below this natural width a landscape poster would be blown up past its
	// pixels across the page; it keeps the side column instead.
	const WIDE_POSTER_MIN_WIDTH = 400;

	// A big enough landscape poster gets the page's width, above the title,
	// rather than a thumbnail-sized strip beside it.
	let widePoster = $derived(
		!!show?.poster &&
			posterSize?.path === show.poster.path &&
			posterSize.width > posterSize.height * 1.1 &&
			posterSize.width >= WIDE_POSTER_MIN_WIDTH,
	);

	// Credits as a theatre programme prints them: one line per role, in
	// programme order (writing and staging first, then the cast, the company
	// last), people in the order entered. Roles held by exactly the same people
	// share a line ("Writer · Director — X"), but the cast stays complete: a
	// writer-director who also plays is listed among the performers, with
	// their character.
	const roleRank = [
		"Writer",
		"Translation",
		"Director",
		"Choreographer",
		"Music",
		"Performer",
	];
	function rankOf(role: string) {
		const i = roleRank.indexOf(role);
		if (i >= 0) return i;
		return role === "Company" ? 1000 : 500;
	}

	let creditGroups = $derived.by(() => {
		type Entry = { personId: number; name: string; character: string };
		const byRole: Record<string, Entry[]> = {};
		const seen: string[] = [];
		for (const c of show?.credits ?? []) {
			if (!byRole[c.role]) {
				byRole[c.role] = [];
				seen.push(c.role);
			}
			if (!byRole[c.role].some((e) => e.personId === c.personId))
				byRole[c.role].push({
					personId: c.personId,
					name: c.name,
					character: c.role === "Performer" ? c.character : "",
				});
		}
		const roles = seen
			.map((r, i) => ({ r, i }))
			.sort((a, b) => rankOf(a.r) - rankOf(b.r) || a.i - b.i)
			.map((x) => x.r);
		const groups: [string, Entry[]][] = [];
		const key = (es: Entry[]) => es.map((e) => e.personId).join(",");
		for (const r of roles) {
			const people = byRole[r];
			const g = groups.find(([, es]) => key(es) === key(people));
			if (!g) {
				groups.push([r, people]);
				continue;
			}
			g[0] = `${g[0]} · ${r}`;
			// Same people: keep the entries that carry the characters.
			if (r === "Performer") g[1] = people;
		}
		return groups;
	});

	// Only the poster fields: the photos list is kept up to date here as uploads
	// finish, and a reply must not overwrite one still in flight.
	function posterChanged(r: LiveShow) {
		if (!show) return;
		show.poster = r.poster;
		show.posterUrl = r.posterUrl;
		show.updatedAt = r.updatedAt;
	}

	async function useAsPoster(photoId: number) {
		if (!show) return;
		try {
			posterChanged(await posterFromPhoto(show.id, photoId));
			notify({ type: "success", text: "Poster updated" });
		} catch (err) {
			console.error("live: use as poster failed", err);
			notify({ type: "error", text: errorText(err) });
		}
	}

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

	/** Saves rating, notes or friends (only the keys given). Resolves false
	 *  when it failed, after saying so. */
	async function saveSouvenir(body: LiveSouvenirRequest): Promise<boolean> {
		if (!show) return false;
		const forId = show.id;
		try {
			const s = await updateSouvenir(forId, body);
			// Only the souvenir fields: photos are kept up to date here as
			// they upload, and a reply sent before one finished would drop it.
			if (show?.id === forId) {
				show.rating = s.rating;
				show.comment = s.comment;
				show.friends = s.friends;
				show.updatedAt = s.updatedAt;
			}
			return true;
		} catch (err) {
			console.error("live: souvenir save failed", err);
			notify({ type: "error", text: `Failed to save: ${errorText(err)}` });
			return false;
		}
	}

	function setRating(r: number) {
		// 0 clears it.
		saveSouvenir({ rating: r > 0 ? r : null });
	}

	function photoAdded(p: LivePhoto) {
		if (show) show.photos = sortPhotos([...show.photos, p]);
	}

	function photoUpdated(p: LivePhoto) {
		if (show)
			show.photos = sortPhotos(show.photos.map((x) => (x.id === p.id ? p : x)));
	}

	function photoDeleted(photoId: number) {
		if (show) show.photos = show.photos.filter((x) => x.id !== photoId);
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
			<div class="top" class:wide={widePoster}>
				<div
					class="poster"
					style:max-width={widePoster ? `${posterSize?.width}px` : undefined}
				>
					<LivePosterControl
						showId={show.id}
						poster={show.poster}
						onChanged={posterChanged}
						onSize={(s) => (posterSize = s)}
					/>
				</div>
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
									>{#if c.character}<span class="character"
											>({c.character})</span
										>{/if}{#if i < people.length - 1}<span class="sep">,</span
										>{/if}
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

			<section class="souvenir" aria-labelledby="live-with">
				<h3 id="live-with">With</h3>
				<LiveFriends
					friends={show.friends ?? []}
					onSave={(names) => saveSouvenir({ friends: names })}
				/>
			</section>

			<!-- Rating and notes once the date has passed; friends and photos
			  (the ticket!) are known long before. -->
			{#if show.state !== "upcoming"}
				<section class="souvenir" aria-labelledby="live-rating">
					<h3 id="live-rating">My rating</h3>
					<Rating rating={show.rating ?? undefined} onChange={setRating} />
					{#if show.rating}
						<button class="plain clear" onclick={() => setRating(0)}>
							Clear rating
						</button>
					{/if}
				</section>

				<section class="souvenir" aria-labelledby="live-notes">
					<h3 id="live-notes">My notes</h3>
					<MyThoughts
						contentTitle={show.title}
						thoughts={show.comment ?? ""}
						onChange={(comment) => saveSouvenir({ comment })}
					/>
				</section>
			{/if}

			<section class="souvenir" aria-labelledby="live-photos">
				<h3 id="live-photos">Photos</h3>
				<LivePhotos
					showId={show.id}
					photos={show.photos ?? []}
					onAdded={photoAdded}
					onUpdated={photoUpdated}
					onDeleted={photoDeleted}
					onUseAsPoster={useAsPoster}
				/>
			</section>
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

		&.wide {
			flex-flow: column;
			align-items: stretch;

			.poster {
				flex-basis: auto;
				width: 100%;
			}
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

	.character {
		margin-left: 0.3em;
		font-style: italic;
		color: $text-color-accent;
	}

	.description {
		margin-top: 14px;
		// Keep the paragraphs the description was typed with.
		white-space: pre-line;
		overflow-wrap: anywhere;
	}

	.souvenir {
		margin-top: 28px;

		h3 {
			margin-bottom: 10px;
		}

		.clear {
			margin-top: 6px;
			padding: 4px 0;
			font-size: 14px;
			color: $text-color-accent;
			text-decoration: underline;
			text-underline-offset: 3px;

			@media (hover: hover) {
				&:hover {
					color: $text-color;
				}
			}
		}
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
