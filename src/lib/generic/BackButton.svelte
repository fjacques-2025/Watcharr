<script lang="ts">
	import { goto } from "$app/navigation";
	import Icon from "@/lib/Icon.svelte";
	import { canGoBackInApp, lastInAppUrl } from "@/lib/util/navHistory";

	function back() {
		// Go back within the app when there's in-app history. Otherwise (fresh
		// load / deep link / PWA opened directly here) return to the page we came
		// from if this tab remembers one — landing on the home list would throw
		// away the listing you were browsing. Home only as a last resort.
		if (canGoBackInApp()) {
			history.back();
		} else {
			goto(lastInAppUrl() ?? "/");
		}
	}
</script>

<!-- Arrow only: the word "Back" restated what the arrow already says, and it
     cost width on every screen that carries this button. `aria-label` keeps the
     name for assistive tech, which the text was otherwise providing. -->
<button class="back-button" onclick={back} title="Go back" aria-label="Go back">
	<Icon i="arrow" wh={18} />
</button>

<style lang="scss">
	.back-button {
		display: flex;
		align-items: center;
		justify-content: center;
		width: max-content;
		margin-bottom: 14px;
		// Squarer than before now that it holds a single glyph, but still wide
		// enough to stay a comfortable target.
		padding: 8px 12px;
		border-radius: 8px;
		fill: currentColor;
	}
</style>
