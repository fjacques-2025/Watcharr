<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { onMount } from "svelte";
	import Icon from "@/lib/Icon.svelte";
	import { adjacentItem, itemHref } from "@/lib/util/listNav.svelte";

	// Previous/next navigation across the current watched-list order, for a
	// detail page. Renders edge arrows and supports a horizontal swipe on touch.
	interface Props {
		type: "tv" | "movie";
	}
	let { type }: Props = $props();

	let id = $derived(Number(page.params.id));
	let adj = $derived(adjacentItem(type, id));

	function go(dir: "prev" | "next") {
		const it = dir === "prev" ? adj.prev : adj.next;
		// Replace history instead of pushing, so browsing prev/next doesn't
		// stack entries — the Back button then returns straight to the list.
		if (it) goto(itemHref(it), { replaceState: true });
	}

	// Don't hijack swipes that start inside a horizontally-scrollable area
	// (e.g. the cast / similar rows), so those keep scrolling normally.
	function inHorizontalScroller(el: HTMLElement | null): boolean {
		while (el && el !== document.body) {
			if (
				el.scrollWidth > el.clientWidth + 4 &&
				getComputedStyle(el).overflowX !== "visible"
			) {
				return true;
			}
			el = el.parentElement;
		}
		return false;
	}

	onMount(() => {
		let x0: number | null = null;
		let y0 = 0;
		const onStart = (e: TouchEvent) => {
			if (e.touches.length !== 1 || inHorizontalScroller(e.target as HTMLElement)) {
				x0 = null;
				return;
			}
			x0 = e.touches[0].clientX;
			y0 = e.touches[0].clientY;
		};
		const onEnd = (e: TouchEvent) => {
			if (x0 === null) return;
			const dx = e.changedTouches[0].clientX - x0;
			const dy = e.changedTouches[0].clientY - y0;
			x0 = null;
			// Mostly-horizontal swipe past the threshold.
			if (Math.abs(dx) > 80 && Math.abs(dx) > Math.abs(dy) * 1.5) {
				if (dx < 0) go("next");
				else go("prev");
			}
		};
		window.addEventListener("touchstart", onStart, { passive: true });
		window.addEventListener("touchend", onEnd, { passive: true });
		return () => {
			window.removeEventListener("touchstart", onStart);
			window.removeEventListener("touchend", onEnd);
		};
	});
</script>

{#if adj.prev}
	<button class="item-nav prev" onclick={() => go("prev")} title="Previous" aria-label="Previous">
		<Icon i="chevron" facing="left" wh={26} />
	</button>
{/if}
{#if adj.next}
	<button class="item-nav next" onclick={() => go("next")} title="Next" aria-label="Next">
		<Icon i="chevron" facing="right" wh={26} />
	</button>
{/if}

<style lang="scss">
	.item-nav {
		position: fixed;
		top: 50%;
		transform: translateY(-50%);
		z-index: 55;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 42px;
		height: 60px;
		border: none;
		border-radius: 10px;
		background: rgba(0, 0, 0, 0.45);
		fill: white;
		opacity: 0.55;
		cursor: pointer;
		transition: opacity 150ms ease;

		&:hover {
			opacity: 1;
		}

		&.prev {
			left: 8px;
		}
		&.next {
			right: 8px;
		}
	}
</style>
