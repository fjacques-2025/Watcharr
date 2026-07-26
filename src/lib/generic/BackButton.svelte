<script lang="ts">
	import { afterNavigate, goto } from "$app/navigation";
	import Icon from "@/lib/Icon.svelte";

	// Whether we reached this page via in-app navigation, so history.back()
	// stays inside the app. Stays false on a fresh load / deep link (e.g. a
	// bookmarked detail page, or the PWA opened directly here) → go home.
	let cameFromApp = $state(false);
	afterNavigate((nav) => {
		if (nav.from) cameFromApp = true;
	});

	function back() {
		if (cameFromApp) {
			history.back();
		} else {
			goto("/");
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
