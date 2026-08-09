<script lang="ts">
	import { goto } from "$app/navigation";
	import Icon from "@/lib/Icon.svelte";
	import { canGoBackInApp } from "@/lib/util/navHistory";

	function back() {
		// Go back within the app when there's in-app history; otherwise (fresh
		// load / deep link / PWA opened directly here) fall back to home.
		if (canGoBackInApp()) {
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
