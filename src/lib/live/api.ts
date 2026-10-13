import { req } from "@/lib/util/api";
import { ReqerError } from "@/lib/util/fetch";
import type {
	LivePersonDetail,
	LivePersonSummary,
	LiveShow,
	LiveShowPrefill,
	LiveShowRequest,
	Media,
	PaginationResponse,
	SearchResponseMeta,
} from "@/types";

export const listShows = () => req.get<LiveShow[]>("/live/shows");

export const getShow = (id: number) => req.get<LiveShow>(`/live/shows/${id}`);

export const createShow = (body: LiveShowRequest) =>
	req.post<LiveShow>("/live/shows", body);

export const updateShow = (id: number, body: LiveShowRequest) =>
	req.put<LiveShow>(`/live/shows/${id}`, body);

export const deleteShow = (id: number) => req.delete(`/live/shows/${id}`);

/** Reads a show's page on the server; what it can't find comes back "". */
export const prefillFromLink = (url: string) =>
	req.post<LiveShowPrefill>("/live/prefill", { url });

/** The notebook's artists, most credited first; with `q`, those whose name
 *  contains it (accents and case ignored), at most 20. */
export const listPeople = (q?: string, signal?: AbortSignal) =>
	req.get<LivePersonSummary[]>("/live/people", {
		params: q ? { q } : undefined,
		signal,
	});

export const getPerson = (id: number) =>
	req.get<LivePersonDetail>(`/live/people/${id}`);

/** Renames the artist and/or links (a TMDB id) or unlinks (null) them. */
export const updatePerson = (
	id: number,
	body: { name: string; tmdbPersonId: number | null },
) =>
	req.put<{ id: number; name: string; tmdbPersonId: number | null }>(
		`/live/people/${id}`,
		body,
	);

/** The notebook artist linked to this TMDB person, if any. */
export const getPersonByTmdb = (tmdbId: number) =>
	req.get<{ person: LivePersonDetail | null }>(
		`/live/people/by-tmdb/${tmdbId}`,
	);

/** A TMDB person, as the suggestions show them. */
export interface TmdbPersonHit {
	tmdbPersonId: number;
	name: string;
	/** Full URL of a small profile photo; "" when TMDB has none. */
	photo: string;
}

/** What picking a suggestion gives: the name, and the TMDB person it's
 *  linked to (null when none). */
export interface ArtistPick {
	name: string;
	tmdbPersonId: number | null;
}

/** The app's existing people search (TMDB), first page only. */
export async function searchTmdbPeople(
	query: string,
	signal?: AbortSignal,
): Promise<TmdbPersonHit[]> {
	const r = await req.get<PaginationResponse<Media, SearchResponseMeta>>(
		"/search",
		{ params: { type: "person", query, page: 1 }, signal },
	);
	const hits: TmdbPersonHit[] = [];
	for (const m of r.results ?? []) {
		if (!m.ids?.tmdb || !m.name) continue;
		hits.push({
			tmdbPersonId: m.ids.tmdb,
			name: m.name,
			photo: m.extPosterPath
				? `https://image.tmdb.org/t/p/w185${m.extPosterPath}`
				: "",
		});
	}
	return hits;
}

/** The full request for an existing show: PUT replaces every field, so a
 *  partial change (like flipping `missed`) must resend the rest unchanged. */
export function toRequest(s: LiveShow): LiveShowRequest {
	return {
		title: s.title,
		venue: s.venue,
		url: s.url,
		description: s.description,
		date: s.date,
		missed: s.missed,
		// Unchanged, so the stored poster is kept.
		posterUrl: s.posterUrl ?? "",
		// `credits` left out on purpose: absent means "unchanged" to the
		// server, so a partial change can't drop or reorder them.
	};
}

/** The server's own `{ error }` message when there is one: it's the authority
 *  on validation, and says what's actually wrong. */
export function errorText(err: unknown): string {
	if (ReqerError.withBody(err) && err.hasErrorInBody()) return err.body.error;
	if (err instanceof Error && err.message) return err.message;
	return "Something went wrong.";
}
