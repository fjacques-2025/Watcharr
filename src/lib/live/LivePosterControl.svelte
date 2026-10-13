<script lang="ts">
	// The show's poster, with ways to provide one: some pages declare none,
	// and on a phone copying an image's address is next to impossible.
	import type { LiveShow } from "@/types";
	import { notify } from "@/lib/util/notify";
	import LivePoster from "./LivePoster.svelte";
	import { errorText, removePoster, uploadPoster } from "./api";
	import { resizeImage } from "./resizeImage";

	interface Props {
		showId: number;
		poster: LiveShow["poster"];
		/** The server's updated show; the caller takes the poster fields. */
		onChanged: (show: LiveShow) => void;
	}

	let { showId, poster, onChanged }: Props = $props();

	let busy = $state(false);
	let input: HTMLInputElement | undefined = $state();

	async function picked(ev: Event) {
		const el = ev.currentTarget as HTMLInputElement;
		const file = el.files?.[0];
		// Reset so picking the same file again still fires.
		el.value = "";
		if (!file) return;
		busy = true;
		try {
			// Same treatment as photos: the server refuses over 10 megapixels,
			// can't read HEIC and ignores EXIF orientation.
			onChanged(await uploadPoster(showId, await resizeImage(file)));
		} catch (err) {
			console.error("live: poster upload failed", err);
			notify({ type: "error", text: errorText(err) });
		} finally {
			busy = false;
		}
	}

	async function remove() {
		busy = true;
		try {
			onChanged(await removePoster(showId));
		} catch (err) {
			console.error("live: poster removal failed", err);
			notify({ type: "error", text: errorText(err) });
		} finally {
			busy = false;
		}
	}
</script>

<div class="poster-control">
	{#if poster}
		<LivePoster {poster} />
		<div class="actions">
			<button
				type="button"
				class="plain"
				onclick={() => input?.click()}
				disabled={busy}
			>
				{busy ? "…" : "Change"}
			</button>
			<button type="button" class="plain" onclick={remove} disabled={busy}>
				Remove
			</button>
		</div>
	{:else}
		<button
			type="button"
			class="placeholder"
			onclick={() => input?.click()}
			disabled={busy}
		>
			{busy ? "Uploading…" : "Add a poster"}
		</button>
	{/if}
	<input
		bind:this={input}
		type="file"
		accept="image/*"
		hidden
		onchange={picked}
	/>
</div>

<style lang="scss">
	.poster-control {
		display: flex;
		flex-flow: column;
		gap: 6px;
	}

	.placeholder {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 100%;
		aspect-ratio: 2 / 3;
		padding: 8px;
		border: 2px dashed $text-color-accent;
		border-radius: 8px;
		background-color: transparent;
		font-size: 13px;
		text-align: center;
		color: $text-color-accent;

		@media (hover: hover) {
			&:not(:disabled):hover {
				border-color: $text-color;
				color: $text-color;
			}
		}
	}

	.actions {
		display: flex;
		justify-content: center;
		gap: 10px;

		button {
			padding: 2px 4px;
			font-size: 12px;
			color: $text-color-accent;
			text-decoration: underline;

			@media (hover: hover) {
				&:not(:disabled):hover {
					color: $text-color;
				}
			}
		}
	}
</style>
