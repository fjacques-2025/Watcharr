import type { Handle } from "@sveltejs/kit";

// Drop the (potentially large) `Link: rel=modulepreload` response header.
// On the heaviest routes (e.g. /tv/[id], /movie/[id]) this header can exceed a
// reverse proxy's header buffer (Synology DSM / nginx default ~4KB), which
// makes nginx reply "502 upstream sent too big header". The modulepreload hints
// are still emitted inside the HTML <head>, so preloading keeps working.
export const handle: Handle = async ({ event, resolve }) => {
	const response = await resolve(event);
	response.headers.delete("link");
	return response;
};
