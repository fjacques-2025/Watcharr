// `<input type="datetime-local">` speaks a zone-less "YYYY-MM-DDTHH:mm", while
// the server stores RFC3339. These convert between the two in the browser's own
// zone: a show booked for 20:00 must stay 20:00 on the wall clock, which a trip
// through toISOString() (UTC) would break.

const pad = (n: number) => String(n).padStart(2, "0");

/** RFC3339 → "YYYY-MM-DDTHH:mm" in local time, for a datetime-local input. */
export function toLocalInput(rfc3339: string): string {
	const d = new Date(rfc3339);
	if (isNaN(d.getTime())) return "";
	return (
		`${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` +
		`T${pad(d.getHours())}:${pad(d.getMinutes())}`
	);
}

/**
 * "YYYY-MM-DDTHH:mm" (local) → RFC3339 with the local offset, e.g.
 * "2026-11-03T20:00:00+01:00". Returns undefined if the value doesn't parse.
 */
export function fromLocalInput(value: string): string | undefined {
	const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(value);
	if (!m) return;
	const [y, mo, da, h, mi] = m.slice(1).map(Number);
	// Built from parts so it's read as local time; the offset is then taken
	// for that date, not today's, so a show past a DST change gets the right one.
	const d = new Date(y, mo - 1, da, h, mi);
	if (isNaN(d.getTime())) return;
	const off = -d.getTimezoneOffset();
	const sign = off >= 0 ? "+" : "-";
	const abs = Math.abs(off);
	return (
		`${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` +
		`T${pad(d.getHours())}:${pad(d.getMinutes())}:00` +
		`${sign}${pad(Math.floor(abs / 60))}:${pad(abs % 60)}`
	);
}

/** "Tue 3 Nov, 20:00" — the year only when it isn't this one. */
export function formatShowDate(rfc3339: string): string {
	const d = new Date(rfc3339);
	if (isNaN(d.getTime())) return rfc3339;
	return d.toLocaleString(undefined, {
		weekday: "short",
		day: "numeric",
		month: "short",
		year: d.getFullYear() === new Date().getFullYear() ? undefined : "numeric",
		hour: "2-digit",
		minute: "2-digit",
	});
}
