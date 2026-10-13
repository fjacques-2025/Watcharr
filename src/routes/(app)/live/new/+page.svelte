<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import BackButton from "@/lib/generic/BackButton.svelte";
	import LiveShowForm from "@/lib/live/LiveShowForm.svelte";
	import { createShow } from "@/lib/live/api";
	import type { LiveShowRequest } from "@/types";

	async function add(body: LiveShowRequest) {
		const s = await createShow(body);
		// Replace this entry: Back from the new show should return to the list,
		// not to an empty form that would add it a second time.
		goto(resolve(`/live/${s.id}`), { replaceState: true });
	}
</script>

<svelte:head>
	<title>Add a show</title>
</svelte:head>

<div class="content">
	<div class="inner">
		<div class="head">
			<BackButton />
			<h2>Add a show</h2>
		</div>
		<LiveShowForm submitLabel="Add" onSubmit={add} />
	</div>
</div>

<style lang="scss">
	.content {
		display: flex;
		width: 100%;
		justify-content: center;
		margin-top: -18px;

		.inner {
			width: 100%;
			max-width: 600px;
			padding: 0 15px 60px 15px;
		}
	}

	.head {
		display: flex;
		align-items: center;
		gap: 10px;
		margin-bottom: 16px;

		:global(button.back-button) {
			margin-bottom: 0;
			flex: 0 0 auto;
		}
	}
</style>
