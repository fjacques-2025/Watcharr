import { req } from "@/lib/util/api";
import { ReqerError } from "@/lib/util/fetch";
import type {
	LiveFriendSummary,
	LivePersonDetail,
	LivePhoto,
	LivePersonSummary,
	LiveShow,
	LiveShowPrefill,
	LiveShowRequest,
	LiveSouvenirRequest,
	TmdbPersonCandidate,
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

/** Rating, notes and friends: send only the keys being changed. */
export const updateSouvenir = async (id: number, body: LiveSouvenirRequest) =>
	(
		await req.do<LiveShow>(`/live/shows/${id}/souvenir`, {
			method: "PATCH",
			body,
		})
	).body;

/** Friends already met at shows; with `q`, those whose name contains it
 *  (accents and case ignored), at most 20. */
export const listFriends = (q?: string, signal?: AbortSignal) =>
	req.get<LiveFriendSummary[]>("/live/friends", {
		params: q ? { q } : undefined,
		signal,
	});

/** Uploads an already resized JPEG (see `resizeImage`). */
export function uploadPhoto(
	showId: number,
	photo: Blob,
	isTicket: boolean,
	caption = "",
) {
	const fd = new FormData();
	fd.append("photo", photo, "photo.jpg");
	fd.append("isTicket", isTicket ? "true" : "false");
	if (caption) fd.append("caption", caption);
	return req.post<LivePhoto>(`/live/shows/${showId}/photos`, fd);
}

/** A poster chosen by the user, for pages that declare none. */
export function uploadPoster(showId: number, poster: Blob) {
	const fd = new FormData();
	fd.append("poster", poster, "poster.jpg");
	return req.post<LiveShow>(`/live/shows/${showId}/poster`, fd);
}

/** Use one of the show's photos (a snap of the poster or programme). */
export const posterFromPhoto = (showId: number, photoId: number) =>
	req.post<LiveShow>(`/live/shows/${showId}/poster/from-photo/${photoId}`);

export const removePoster = (showId: number) =>
	req.delete<LiveShow>(`/live/shows/${showId}/poster`);

export const updatePhoto = (
	showId: number,
	photoId: number,
	body: { isTicket: boolean; caption: string },
) => req.put<LivePhoto>(`/live/shows/${showId}/photos/${photoId}`, body);

export const deletePhoto = (showId: number, photoId: number) =>
	req.delete(`/live/shows/${showId}/photos/${photoId}`);

/** The server's order: tickets first, then in the order they were added. */
export function sortPhotos(photos: LivePhoto[]): LivePhoto[] {
	return [...photos].sort(
		(a, b) => Number(b.isTicket) - Number(a.isTicket) || a.id - b.id,
	);
}

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

/** What picking a suggestion gives: the name, and the TMDB person it's
 *  linked to (null when none). */
export interface ArtistPick {
	name: string;
	tmdbPersonId: number | null;
}

/** TMDB people for a name, best first (those with films/series in the
 *  user's list on top), at most 8; [] under 2 characters. */
export const searchTmdbPeople = (q: string, signal?: AbortSignal) =>
	req.get<TmdbPersonCandidate[]>("/live/tmdb-people", {
		params: { q },
		signal,
	});

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
