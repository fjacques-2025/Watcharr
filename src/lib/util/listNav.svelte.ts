import { MediaTypeE } from "@/types";

// Ordered list of navigable items currently shown on the watched list, so
// detail pages can offer previous/next navigation (arrows + swipe) matching
// the list order.
export type ListNavItem = { type: "tv" | "movie" | "game"; id: number };

const s = $state<{ items: ListNavItem[] }>({ items: [] });

/** Publish the current watched-list order (from the list of Media items). */
export function setListOrder(medias: any[]) {
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

export function itemHref(it: ListNavItem): string {
	return `/${it.type}/${it.id}`;
}
