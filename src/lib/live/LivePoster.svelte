<script lang="ts">
	import { decode } from "blurhash";
	import { baseURL } from "@/lib/util/api";
	import type { LiveShowPoster } from "@/types";

	interface Props {
		/** The stored poster; a neutral placeholder is drawn without one. */
		poster?: LiveShowPoster;
		/** CSS aspect-ratio of the box: a poster by default, "1 / 1" for
		 *  photo thumbnails (cropped to fill). */
		ratio?: string;
		/** Show a landscape image whole, the box taking its own shape,
		 *  instead of cropping it; off for photo thumbnails. */
		whole?: boolean;
	}

	let { poster, ratio = "2 / 3", whole = true }: Props = $props();

	// Small on purpose: the blurhash is stretched to the box by CSS.
	const BH_W = 20;
	const BH_H = 30;

	// Tracked per path, so a poster replaced by an edit starts over with its
	// own blurhash instead of inheriting the previous image's state.
	let loadedPath = $state("");
	let failedPath = $state("");
	let landscapePath = $state("");
	let landscapeRatio = $state("");
	let bhCanvas: HTMLCanvasElement | undefined = $state();

	let src = $derived(poster?.path ? `${baseURL}/${poster.path}` : "");
	let loaded = $derived(!!poster?.path && loadedPath === poster.path);
	let failed = $derived(!!poster?.path && failedPath === poster.path);
	// A landscape poster cropped to 2:3 loses most of itself (a wide
	// production photo used as og:image), so the box takes the image's own
	// shape instead: no bands around it.
	let landscape = $derived(whole && !!poster?.path && landscapePath === poster.path);

	function onLoad(e: Event) {
		const img = e.currentTarget as HTMLImageElement;
		const path = poster?.path ?? "";
		if (img.naturalWidth > img.naturalHeight * 1.1) {
			landscapeRatio = `${img.naturalWidth} / ${img.naturalHeight}`;
			landscapePath = path;
		}
		loadedPath = path;
	}

	$effect(() => {
		const bh = poster?.blurHash;
		if (!bh || !bhCanvas) return;
		try {
			const pixels = decode(bh, BH_W, BH_H);
			const ctx = bhCanvas.getContext("2d");
			if (!ctx) return;
			const imageData = ctx.createImageData(BH_W, BH_H);
			imageData.data.set(pixels);
			ctx.putImageData(imageData, 0, 0);
		} catch (err) {
			// A bad hash only costs the placeholder; the image still loads.
			console.warn("LivePoster: blurhash decode failed", err);
		}
	});
</script>

<div class="live-poster" class:landscape style:aspect-ratio={landscape ? landscapeRatio : ratio}>
	{#if src && !failed}
		{#if !loaded && poster?.blurHash}
			<canvas width={BH_W} height={BH_H} bind:this={bhCanvas}></canvas>
		{/if}
		<img
			{src}
			alt=""
			loading="lazy"
			class:loaded
			onload={onLoad}
			onerror={() => (failedPath = poster?.path ?? "")}
		/>
	{/if}
</div>

<style lang="scss">
	.live-poster {
		position: relative;
		width: 100%;
		overflow: hidden;
		border-radius: 6px;
		background-color: $bg-color-accent;

		&.landscape {
			background-color: transparent;
		}

		canvas,
		img {
			position: absolute;
			inset: 0;
			width: 100%;
			height: 100%;
		}

		img {
			object-fit: cover;
			opacity: 0;
			transition: opacity 150ms ease;

			&.loaded {
				opacity: 1;
			}

		}
	}
</style>
