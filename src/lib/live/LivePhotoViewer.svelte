<script lang="ts">
	import { onDestroy, onMount } from "svelte";
	import Icon from "@/lib/Icon.svelte";
	import { baseURL } from "@/lib/util/api";
	import { notify } from "@/lib/util/notify";
	import type { LivePhoto } from "@/types";
	import { deletePhoto, errorText, updatePhoto } from "./api";

	interface Props {
		showId: number;
		/** In display order. */
		photos: LivePhoto[];
		currentId: number;
		onNavigate: (photoId: number) => void;
		onClose: () => void;
		onUpdated: (p: LivePhoto) => void;
		onDeleted: (photoId: number) => void;
		/** Makes this photo the show's poster; omitted → no button. */
		onUseAsPoster?: (photoId: number) => Promise<void>;
	}

	let {
		showId,
		photos,
		currentId,
		onNavigate,
		onClose,
		onUpdated,
		onDeleted,
		onUseAsPoster,
	}: Props = $props();

	const SWIPE_PX = 50;
	const uid = $props.id();
	const titleId = `${uid}-title`;
	const captionId = `${uid}-caption`;

	let dialogEl: HTMLDivElement | undefined = $state();
	let closeBtn: HTMLButtonElement | undefined = $state();
	let draft = $state("");
	let busy = $state(false);
	let confirmingDelete = $state(false);

	let index = $derived(photos.findIndex((p) => p.id === currentId));
	let current = $derived(index >= 0 ? photos[index] : undefined);
	let src = $derived(current ? `${baseURL}/${current.image.path}` : "");
	let captionChanged = $derived(
		!!current && draft.trim() !== current.caption.trim(),
	);

	// The caption field follows the photo shown (not its later saves, which
	// set it themselves).
	let draftFor = -1;
	$effect(() => {
		const p = current;
		if (!p || p.id === draftFor) return;
		draftFor = p.id;
		draft = p.caption;
		confirmingDelete = false;
	});

	// The photo is gone (deleted elsewhere in the meantime): nothing to show.
	$effect(() => {
		if (index < 0) onClose();
	});

	let returnFocus: HTMLElement | null = null;
	let prevOverflow = "";

	onMount(() => {
		returnFocus =
			document.activeElement instanceof HTMLElement
				? document.activeElement
				: null;
		prevOverflow = document.documentElement.style.overflow;
		document.documentElement.style.overflow = "hidden";
		closeBtn?.focus();
	});

	onDestroy(() => {
		document.documentElement.style.overflow = prevOverflow;
		if (returnFocus?.isConnected) returnFocus.focus();
	});

	function go(step: number) {
		if (photos.length < 2 || index < 0) return;
		const i = (index + step + photos.length) % photos.length;
		onNavigate(photos[i].id);
	}

	async function save(
		p: LivePhoto,
		body: { isTicket: boolean; caption: string },
	) {
		if (busy) return;
		busy = true;
		try {
			const saved = await updatePhoto(showId, p.id, body);
			onUpdated(saved);
			if (saved.id === current?.id) draft = saved.caption;
		} catch (err) {
			console.error("live: photo update failed", err);
			notify({ type: "error", text: `Failed to save: ${errorText(err)}` });
		} finally {
			busy = false;
		}
	}

	function saveCaption() {
		if (!current || !captionChanged) return;
		save(current, { isTicket: current.isTicket, caption: draft.trim() });
	}

	function toggleTicket() {
		if (!current) return;
		// The saved caption: an unsaved edit stays in the field.
		save(current, { isTicket: !current.isTicket, caption: current.caption });
	}

	async function remove() {
		const p = current;
		if (!p || busy) return;
		busy = true;
		// Where to land afterwards: the next one, or the previous at the end.
		const next =
			photos.length > 1
				? photos[index + 1 < photos.length ? index + 1 : index - 1].id
				: undefined;
		try {
			await deletePhoto(showId, p.id);
			if (next !== undefined) onNavigate(next);
			onDeleted(p.id);
			if (next === undefined) onClose();
		} catch (err) {
			console.error("live: photo delete failed", err);
			notify({ type: "error", text: `Failed to delete: ${errorText(err)}` });
		} finally {
			busy = false;
			confirmingDelete = false;
		}
	}

	function typing(t: EventTarget | null) {
		return (
			t instanceof HTMLInputElement ||
			t instanceof HTMLTextAreaElement ||
			(t instanceof HTMLElement && t.isContentEditable)
		);
	}

	function onkeydown(ev: KeyboardEvent) {
		if (ev.isComposing) return;
		switch (ev.key) {
			case "Escape":
				ev.preventDefault();
				if (confirmingDelete) confirmingDelete = false;
				else onClose();
				break;
			case "ArrowLeft":
				if (typing(ev.target)) return;
				ev.preventDefault();
				go(-1);
				break;
			case "ArrowRight":
				if (typing(ev.target)) return;
				ev.preventDefault();
				go(1);
				break;
			case "Tab":
				trapFocus(ev);
				break;
		}
	}

	// Keep Tab inside the dialog.
	function trapFocus(ev: KeyboardEvent) {
		if (!dialogEl) return;
		const els = Array.from(
			dialogEl.querySelectorAll<HTMLElement>(
				"button:not(:disabled), input:not(:disabled), [tabindex]:not([tabindex='-1'])",
			),
		);
		if (!els.length) return;
		const first = els[0];
		const last = els[els.length - 1];
		if (ev.shiftKey && document.activeElement === first) {
			ev.preventDefault();
			last.focus();
		} else if (!ev.shiftKey && document.activeElement === last) {
			ev.preventDefault();
			first.focus();
		} else if (!dialogEl.contains(document.activeElement)) {
			ev.preventDefault();
			first.focus();
		}
	}

	let touchX = 0;
	let touchY = 0;
	function ontouchstart(ev: TouchEvent) {
		if (ev.touches.length !== 1) return;
		touchX = ev.touches[0].clientX;
		touchY = ev.touches[0].clientY;
	}
	function ontouchend(ev: TouchEvent) {
		const t = ev.changedTouches[0];
		if (!t) return;
		const dx = t.clientX - touchX;
		const dy = t.clientY - touchY;
		if (Math.abs(dx) >= SWIPE_PX && Math.abs(dx) > Math.abs(dy) * 1.5)
			go(dx < 0 ? 1 : -1);
	}
</script>

<svelte:window {onkeydown} />

<!-- Clicking the dark area around the photo closes it (Escape does too). -->
<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div
	class="viewer"
	onclick={(e) => {
		if (e.target === e.currentTarget) onClose();
	}}
>
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-labelledby={titleId}
		bind:this={dialogEl}
	>
		<div class="bar">
			<span class="count" id={titleId}>
				{current?.isTicket ? "Ticket" : "Photo"}
				{index + 1} / {photos.length}
			</span>
			<button
				class="plain icon"
				bind:this={closeBtn}
				onclick={onClose}
				aria-label="Close"
			>
				<Icon i="close" wh={22} />
			</button>
		</div>

		<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
		<div
			class="stage"
			{ontouchstart}
			{ontouchend}
			onclick={(e) => {
				if (e.target === e.currentTarget) onClose();
			}}
		>
			{#if current}
				{#key current.id}
					<img
						{src}
						alt={current.caption || (current.isTicket ? "Ticket" : "Photo")}
					/>
				{/key}
			{/if}
			{#if photos.length > 1}
				<button
					class="plain nav prev"
					onclick={() => go(-1)}
					aria-label="Previous photo"
				>
					<Icon i="chevron" wh={26} />
				</button>
				<button
					class="plain nav next"
					onclick={() => go(1)}
					aria-label="Next photo"
				>
					<Icon i="chevron" facing="right" wh={26} />
				</button>
			{/if}
		</div>

		{#if current}
			<div class="panel">
				<form
					class="caption"
					onsubmit={(e) => {
						e.preventDefault();
						saveCaption();
					}}
				>
					<label class="sr-only" for={captionId}>Caption</label>
					<input
						id={captionId}
						type="text"
						bind:value={draft}
						placeholder="Add a caption"
						maxlength={300}
						autocomplete="off"
					/>
					{#if captionChanged}
						<button type="submit" disabled={busy}>Save</button>
					{/if}
				</form>
				<div class="actions">
					<button
						class="ticket"
						aria-pressed={current.isTicket}
						onclick={toggleTicket}
						disabled={busy}
					>
						{#if current.isTicket}<Icon i="check" wh={16} />{:else}<Icon
								i="ticket"
								wh={16}
							/>{/if}
						Ticket
					</button>
					{#if onUseAsPoster}
						<button
							class="secondary"
							onclick={async () => {
								busy = true;
								try {
									await onUseAsPoster(current.id);
								} finally {
									busy = false;
								}
							}}
							disabled={busy}
						>
							Use as poster
						</button>
					{/if}
					{#if confirmingDelete}
						<button class="danger" onclick={remove} disabled={busy}>
							Confirm delete
						</button>
						<button
							class="secondary"
							onclick={() => (confirmingDelete = false)}
							disabled={busy}
						>
							Cancel
						</button>
					{:else}
						<button
							class="danger"
							onclick={() => (confirmingDelete = true)}
							disabled={busy}
						>
							<Icon i="trash" wh={16} /> Delete
						</button>
					{/if}
				</div>
			</div>
		{/if}
	</div>
</div>

<style lang="scss">
	.viewer {
		position: fixed;
		inset: 0;
		z-index: 99998;
		display: flex;
		justify-content: center;
		background-color: rgba(0, 0, 0, 0.9);
	}

	.dialog {
		display: flex;
		flex-flow: column;
		width: 100%;
		max-width: 1000px;
		height: 100dvh;
	}

	.bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 8px 8px 8px 16px;
		color: white;

		.count {
			font-size: 14px;
			font-variant-numeric: tabular-nums;
		}
	}

	.icon,
	.nav {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 44px;
		height: 44px;
		padding: 0;
		border-radius: 50%;
		color: white;
		fill: white;

		@media (hover: hover) {
			&:hover {
				background-color: rgba(255, 255, 255, 0.15);
			}
		}

		&:focus-visible {
			outline: 2px solid white;
		}
	}

	.stage {
		position: relative;
		flex: 1 1 auto;
		min-height: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 0 8px;

		img {
			max-width: 100%;
			max-height: 100%;
			object-fit: contain;
			border-radius: 4px;
			user-select: none;
			-webkit-user-drag: none;
		}

		.nav {
			position: absolute;
			top: 50%;
			transform: translateY(-50%);
			background-color: rgba(0, 0, 0, 0.45);

			@media (hover: hover) {
				&:hover {
					background-color: rgba(0, 0, 0, 0.7);
				}
			}

			&.prev {
				left: 8px;
			}

			&.next {
				right: 8px;
			}
		}
	}

	.panel {
		display: flex;
		flex-flow: column;
		gap: 10px;
		padding: 12px 16px calc(12px + env(safe-area-inset-bottom)) 16px;
		background-color: $bg-color;
		color: $text-color;

		@media (min-width: 1000px) {
			border-radius: 8px 8px 0 0;
		}
	}

	.caption {
		display: flex;
		flex-flow: row;
		gap: 8px;

		input {
			flex: 1 1 auto;
			min-width: 0;
			color: $text-color;
			background-color: transparent;
			border-color: $text-color;
		}

		button {
			width: auto;
			flex: 0 0 auto;
		}
	}

	.actions {
		display: flex;
		flex-flow: row wrap;
		gap: 10px;

		button {
			gap: 6px;
			width: auto;
		}

		// On: an accent edge, distinct from the (inverted) hover look.
		.ticket[aria-pressed="true"] {
			border-color: $accent-color;
			box-shadow: inset 0 0 0 1px $accent-color;
		}
	}

	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		margin: -1px;
		padding: 0;
		overflow: hidden;
		clip: rect(0 0 0 0);
		white-space: nowrap;
		border: 0;
	}
</style>
