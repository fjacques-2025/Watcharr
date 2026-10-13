// Helpers for working with many show links at once (the bulk add page).

/** A full http(s) address. */
export function isHttpUrl(s: string): boolean {
	try {
		const u = new URL(s);
		return u.protocol === "http:" || u.protocol === "https:";
	} catch {
		return false;
	}
}

/**
 * The key two links to the same show page share: lowercase host without a
 * leading "www.", path without trailing slashes; scheme, port, query and
 * fragment ignored. "" when the link doesn't parse.
 */
export function linkKey(s: string): string {
	try {
		const u = new URL(s.trim());
		const host = u.hostname.toLowerCase().replace(/^www\./, "");
		const path = u.pathname.replace(/\/+$/, "");
		return host ? `${host}${path}` : "";
	} catch {
		return "";
	}
}

export interface PastedLinks {
	/** Unique http(s) links, in the order pasted. */
	links: string[];
	/** Non-blank lines that aren't http(s) links. */
	ignored: string[];
	/** How many lines repeated a link already listed. */
	duplicates: number;
}

/** One link per line: trimmed, blank lines skipped, repeats dropped. */
export function parsePastedLinks(text: string): PastedLinks {
	const out: PastedLinks = { links: [], ignored: [], duplicates: 0 };
	const seen = new Set<string>();
	for (const raw of text.split(/\r?\n/)) {
		const line = raw.trim();
		if (!line) continue;
		if (!isHttpUrl(line)) {
			out.ignored.push(line);
			continue;
		}
		const key = linkKey(line) || line;
		if (seen.has(key)) {
			out.duplicates++;
			continue;
		}
		seen.add(key);
		out.links.push(line);
	}
	return out;
}

/** Runs `fn` on each item with at most `limit` calls in flight. */
export async function eachLimited<T>(
	items: T[],
	limit: number,
	fn: (item: T) => Promise<void>,
): Promise<void> {
	let next = 0;
	const worker = async () => {
		while (next < items.length) {
			const item = items[next++];
			await fn(item);
		}
	};
	await Promise.all(
		Array.from({ length: Math.min(limit, items.length) }, worker),
	);
}
