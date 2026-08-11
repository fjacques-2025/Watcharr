/**
 * Scroll back to a saved position once the (async-rendered) page is tall enough
 * to reach it.
 *
 * Scrolling straight away doesn't work: at restore time the list usually hasn't
 * laid out yet, so the browser clamps the scroll near the top. So poll on
 * animation frames until the document can actually accommodate `y`, then scroll
 * — and scroll once more on the next frame, to win over SvelteKit's own early
 * scroll restoration.
 *
 * Gives up after `maxFrames` (~1s at 60fps) so a page that never grows tall
 * enough doesn't leave a frame callback running forever.
 */
export function restoreScrollTo(y: number, maxFrames = 60) {
	if (y <= 0) return;
	let frames = 0;
	const step = () => {
		const maxScroll =
			document.documentElement.scrollHeight - window.innerHeight;
		if (maxScroll >= y || frames >= maxFrames) {
			window.scrollTo(0, y);
			requestAnimationFrame(() => window.scrollTo(0, y));
			return;
		}
		frames++;
		requestAnimationFrame(step);
	};
	requestAnimationFrame(step);
}
