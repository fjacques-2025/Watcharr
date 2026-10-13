import { req } from "@/lib/util/api";
import { ReqerError } from "@/lib/util/fetch";
import type { LiveShow, LiveShowRequest } from "@/types";

export const listShows = () => req.get<LiveShow[]>("/live/shows");

export const getShow = (id: number) => req.get<LiveShow>(`/live/shows/${id}`);

export const createShow = (body: LiveShowRequest) =>
	req.post<LiveShow>("/live/shows", body);

export const updateShow = (id: number, body: LiveShowRequest) =>
	req.put<LiveShow>(`/live/shows/${id}`, body);

export const deleteShow = (id: number) => req.delete(`/live/shows/${id}`);

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
	};
}

/** The server's own `{ error }` message when there is one: it's the authority
 *  on validation, and says what's actually wrong. */
export function errorText(err: unknown): string {
	if (ReqerError.withBody(err) && err.hasErrorInBody()) return err.body.error;
	if (err instanceof Error && err.message) return err.message;
	return "Something went wrong.";
}
