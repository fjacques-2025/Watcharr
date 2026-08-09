<script lang="ts">
	import DropDown from "./DropDown.svelte";
	import type { DropDownItem } from "@/types";
	import Error from "./Error.svelte";
	import { req } from "./util/api";

	interface LanguageOption {
		code: string;
		name: string;
	}

	interface Props {
		selectedLang?: string;
		disabled?: boolean;
		onChange: (lang: string) => void;
	}

	let {
		selectedLang = $bindable("en-US"),
		disabled = false,
		onChange,
	}: Props = $props();

	let mappedLangs: DropDownItem[] = $state([]);

	async function getLanguages() {
		const l = await req.get<LanguageOption[]>(`/content/languages`);
		mappedLangs = l
			.map((ll) => {
				return {
					id: ll.code,
					value: ll.name,
				} as DropDownItem;
			})
			.sort((a, b) => a.value.localeCompare(b.value));
	}
</script>

{#await getLanguages() then}
	<DropDown
		placeholder="Select a language"
		bind:active={selectedLang}
		options={mappedLangs}
		onChange={() => onChange(selectedLang)}
		isDropDownItem={true}
		{disabled}
	/>
{:catch err}
	<Error error={err} pretty="Failed to load languages!" />
{/await}
