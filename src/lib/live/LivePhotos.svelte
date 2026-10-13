<script lang="ts">
	import { onDestroy } from "svelte";
	import Icon from "@/lib/Icon.svelte";
	import type { LivePhoto } from "@/types";
	import LivePhotoViewer from "./LivePhotoViewer.svelte";
	import LivePoster from "./LivePoster.svelte";
	import { errorText, uploadPhoto } from "./api";
	import { resizeImage, UnreadableImageError } from "./resizeImage";

	interface Props {
		showId: number;
		/** In display order: tickets first. */
		photos: LivePhoto[];
		onAdded: (p: LivePhoto) => void;
		onUpdated: (p: LivePhoto) => void;
		onDeleted: (photoId: number) => void;
		onUseAsPoster?: (photoId: number) => Promise<void>;
	}

	let { showId, photos, onAdded, onUpdated, onDeleted, onUseAsPoster }: Props =
		$props();

	type Upload = {
		key: number;
		name: string;
		file: File;
		isTicket: boolean;
		status: "waiting" | "resizing" | "uploading" | "done" | "failed";
		error: string;
	};

	const statusLabels: Record<Upload["status"], string> = {
		waiting: "Waiting…",
		resizing: "Preparing…",
		uploading: "Uploading…",
		done: "Added",
		failed: "Failed",
	};

	let uploads: Upload[] = $state([]);
	let running = $state(false);
	let openId: number | undefined = $state();
	let photosInput: HTMLInputElement | undefined = $state();
	let ticketInput: HTMLInputElement | undefined = $state();

	let failedCount = $derived(
		uploads.filter((u) => u.status === "failed").length,
	);

	let nextKey = 0;
	let destroyed = false;

	onDestroy(() => {
		destroyed = true;
	});

	function queue(files: FileList | null | undefined, isTicket: boolean) {
		if (!files?.length) return;
		for (const file of Array.from(files)) {
			uploads.push({
				key: nextKey++,
				name: file.name || "Photo",
				file,
				isTicket,
				status: "waiting",
				error: "",
			});
		}
		run();
	}

	// One at a time, in the order chosen: a failure doesn't stop the others,
	// and each photo shows up in the grid as soon as it's stored.
	async function run() {
		if (running) return;
		running = true;
		try {
			for (;;) {
				if (destroyed) return;
				const u = uploads.find((x) => x.status === "waiting");
				if (!u) break;
				try {
					u.status = "resizing";
					const blob = await resizeImage(u.file);
					if (destroyed) return;
					u.status = "uploading";
					const p = await uploadPhoto(showId, blob, u.isTicket);
					if (destroyed) return;
					u.status = "done";
					onAdded(p);
				} catch (err) {
					console.error("live: photo upload failed", err);
					u.status = "failed";
					u.error =
						err instanceof UnreadableImageError ? err.message : errorText(err);
				}
			}
			// The ones that made it are in the grid now; failures stay listed
			// until dismissed.
			uploads = uploads.filter((x) => x.status !== "done");
		} finally {
			running = false;
		}
	}

	function dismissFailed() {
		uploads = uploads.filter((x) => x.status !== "failed");
	}

	function pickedFrom(ev: Event, isTicket: boolean) {
		const input = ev.currentTarget as HTMLInputElement;
		queue(input.files, isTicket);
		// Choosing the same file again must fire `change` again.
		input.value = "";
	}
</script>

<div class="live-photos">
	{#if photos.length}
		<ul class="grid">
			{#each photos as p, i (p.id)}
				<li>
					<button
						class="plain thumb"
						onclick={() => (openId = p.id)}
						aria-label={`${p.isTicket ? "Ticket" : "Photo"} ${i + 1} of ${photos.length}${p.caption ? `: ${p.caption}` : ""}`}
					>
						<LivePoster poster={p.image} ratio="1 / 1" />
						{#if p.isTicket}<span class="badge">Ticket</span>{/if}
					</button>
				</li>
			{/each}
		</ul>
	{/if}

	<div class="buttons">
		<button onclick={() => photosInput?.click()}>
			<Icon i="add" wh={16} /> Add photos
		</button>
		<button onclick={() => ticketInput?.click()}>
			<Icon i="ticket" wh={16} /> Add ticket
		</button>
		<input
			bind:this={photosInput}
			type="file"
			accept="image/*"
			multiple
			hidden
			onchange={(e) => pickedFrom(e, false)}
		/>
		<input
			bind:this={ticketInput}
			type="file"
			accept="image/*"
			hidden
			onchange={(e) => pickedFrom(e, true)}
		/>
	</div>

	{#if uploads.length}
		<ul class="uploads" aria-live="polite">
			{#each uploads as u (u.key)}
				<li data-status={u.status}>
					<span class="name">{u.isTicket ? "Ticket" : u.name}</span>
					<span class="status">
						{statusLabels[u.status]}{#if u.error}: {u.error}{/if}
					</span>
				</li>
			{/each}
		</ul>
		{#if failedCount && !running}
			<button class="secondary dismiss" onclick={dismissFailed}>Dismiss</button>
		{/if}
	{/if}
</div>

{#if openId !== undefined}
	<LivePhotoViewer
		{showId}
		{photos}
		currentId={openId}
		onNavigate={(id) => (openId = id)}
		onClose={() => (openId = undefined)}
		{onUpdated}
		{onDeleted}
		{onUseAsPoster}
	/>
{/if}

<style lang="scss">
	.live-photos {
		display: flex;
		flex-flow: column;
		gap: 12px;
	}

	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(96px, 1fr));
		gap: 8px;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.thumb {
		position: relative;
		display: block;
		width: 100%;
		padding: 0;
		border-radius: 6px;
		cursor: zoom-in;
		transition: opacity 150ms ease;

		@media (hover: hover) {
			&:hover {
				opacity: 0.85;
			}
		}

		&:focus-visible {
			outline: 2px solid $text-color;
			outline-offset: 2px;
		}

		.badge {
			position: absolute;
			left: 4px;
			bottom: 4px;
			padding: 1px 6px;
			border-radius: 8px;
			font-size: 11px;
			font-weight: bold;
			color: white;
			background-color: rgba(0, 0, 0, 0.65);
		}
	}

	.buttons {
		display: flex;
		flex-flow: row wrap;
		gap: 10px;

		button {
			gap: 6px;
			width: auto;
		}
	}

	.uploads {
		display: flex;
		flex-flow: column;
		gap: 4px;
		margin: 0;
		padding: 0;
		list-style: none;
		font-size: 14px;

		li {
			display: flex;
			flex-flow: row wrap;
			gap: 0 10px;
		}

		.name {
			overflow-wrap: anywhere;
		}

		.status {
			color: $text-color-accent;
		}

		[data-status="failed"] .status {
			color: $error;
		}
	}

	.dismiss {
		width: auto;
		align-self: flex-start;
	}
</style>
