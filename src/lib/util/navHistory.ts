// Tracks whether the user has navigated within the app since load, so the
// in-app Back button knows history.back() will stay inside the app and can
// fall back to home when there is no in-app history (fresh load / deep link).
let hasInAppHistory = false;

export function markInAppNavigation() {
	hasInAppHistory = true;
}

export function canGoBackInApp(): boolean {
	return hasInAppHistory;
}
