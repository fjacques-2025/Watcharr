import { resolve } from "$app/paths";
import { MediaTypeE, type Media } from "@/types";

// Ordered list of navigable items currently shown on the watched list, so
// detail pages can offer previous/next navigation (arrows + swipe) matching
// the list order.
export type ListNavItem = { type: "tv" | "movie" | "game"; id: number };

const s = $state<{ items: ListNavItem[] }>({ items: [] });

/** Publish the current watched-list order (from the list of Media items). */
export function setListOrder(medias: Media[]) {
	const items: ListNavItem[] = [];
	for (const m of medias ?? []) {
		if (!m) continue;
		if (m.type === MediaTypeE.tmdbShow && m.ids?.tmdb) {
			items.push({ type: "tv", id: m.ids.tmdb });
		} else if (m.type === MediaTypeE.tmdbMovie && m.ids?.tmdb) {
			items.push({ type: "movie", id: m.ids.tmdb });
		} else if (m.type === MediaTypeE.igdbGame && m.ids?.igdb) {
			items.push({ type: "game", id: m.ids.igdb });
		}
	}
	s.items = items;
}

/** Previous/next item around the given one, or empty when it isn't in the list. */
export function adjacentItem(
	type: string,
	id: number,
): { prev?: ListNavItem; next?: ListNavItem } {
	const idx = s.items.findIndex((it) => it.type === type && it.id === id);
	if (idx < 0) return {};
	return {
		prev: idx > 0 ? s.items[idx - 1] : undefined,
		next: idx < s.items.length - 1 ? s.items[idx + 1] : undefined,
	};
}

export function itemHref(it: ListNavItem) {
	return resolve(`/${it.type}/${it.id}`);
}

// Fresh content (Media) seen on detail pages, keyed by "type:id". Used to patch
// just that item in the restored (cached) list — e.g. after a title's metadata
// was translated on its detail page — with no extra request.
// A plain object, not a (Svelte)Map: a cache read on demand, never rendered.
const contentUpdates: Record<string, Media> = {};

export function markContentRefreshed(type: "tv" | "movie", media: Media) {
	const id = media?.ids?.tmdb;
	if (id != null) contentUpdates[`${type}:${id}`] = media;
}

/** Return the list with any refreshed items patched in place (new array). */
export function applyContentUpdates(medias: Media[]): Media[] {
	if (Object.keys(contentUpdates).length === 0) return medias;
	return (medias ?? []).map((m) => {
		if (!m?.ids) return m;
		const rt =
			m.type === MediaTypeE.tmdbShow
				? "tv"
				: m.type === MediaTypeE.tmdbMovie
					? "movie"
					: null;
		if (!rt) return m;
		const fresh = contentUpdates[`${rt}:${m.ids.tmdb}`];
		if (!fresh) return m;
		// Patch only language-dependent display fields; keep list-specific state
		// (watched status, rating, etc.).
		const patch = { ...m };
		if (fresh.name != null) patch.name = fresh.name;
		if (fresh.summary != null) patch.summary = fresh.summary;
		if (fresh.extPosterPath != null) patch.extPosterPath = fresh.extPosterPath;
		if ("poster" in fresh) patch.poster = fresh.poster;
		return patch;
	});
}
