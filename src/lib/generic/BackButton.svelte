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

<button class="back-button" onclick={back} title="Go back">
	<Icon i="arrow" wh={16} /> Back
</button>

<style lang="scss">
	.back-button {
		display: flex;
		align-items: center;
		gap: 6px;
		width: max-content;
		margin-bottom: 14px;
		padding: 8px 14px;
		border-radius: 8px;
		font-weight: bold;
		fill: currentColor;
	}
</style>
