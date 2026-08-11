// Tracks whether the user has navigated within the app since load, so the
// in-app Back button knows history.back() will stay inside the app, and what to
// fall back to when there is no in-app history (fresh load / deep link).
let hasInAppHistory = false;

// Where we came from, kept in sessionStorage so it survives a full page load —
// which is exactly the case where `hasInAppHistory` is false and the button
// would otherwise dump you on the home list, losing the page you were on.
const LAST_URL_KEY = "watcharr:last-in-app-url";

export function markInAppNavigation(from?: URL) {
	hasInAppHistory = true;
	if (!from) return;
	try {
		sessionStorage.setItem(LAST_URL_KEY, from.pathname + from.search);
	} catch {
		// Private mode / storage disabled: the fallback just stays "home".
	}
}

export function canGoBackInApp(): boolean {
	return hasInAppHistory;
}

/**
 * The last in-app page we navigated away from, if we know of one.
 * Only a same-app path is ever returned, so it is safe to `goto`.
 */
export function lastInAppUrl(): string | undefined {
	try {
		const v = sessionStorage.getItem(LAST_URL_KEY);
		return v?.startsWith("/") ? v : undefined;
	} catch {
		return undefined;
	}
}
