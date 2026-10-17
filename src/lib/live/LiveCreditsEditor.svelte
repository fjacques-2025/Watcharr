<script lang="ts" module>
	import type { LiveCredit, LiveCreditRequest } from "@/types";

	/** One person and every role they hold in the show. `key` only keeps the
	 *  rows stable in #each. On stage one person is often writer, director and
	 *  performer at once: one line with three roles, not three lines. */
	export interface CreditRow {
		key: number;
		name: string;
		tmdbPersonId: number | null;
		roles: string[];
		/** The character played; only used (and sent) while "Performer" is
		 *  ticked, but kept when it's unticked so re-ticking restores it. */
		character: string;
	}

	const PERFORMER = "Performer";

	export const defaultRoles = [
		"Performer",
		"Writer",
		"Director",
		"Choreographer",
		"Company",
		"Music",
		"Translation",
	];

	let nextKey = 0;

	export function newCreditRow(c?: Partial<Omit<CreditRow, "key">>): CreditRow {
		return {
			key: nextKey++,
			name: c?.name ?? "",
			tmdbPersonId: c?.tmdbPersonId ?? null,
			roles: c?.roles ? [...c.roles] : [],
			character: c?.character ?? "",
		};
	}

	/** The server stores one credit per (person, role): fold them back into one
	 *  row per person, in order of first appearance. */
	export function creditRowsFromCredits(credits: LiveCredit[]): CreditRow[] {
		const rows: CreditRow[] = [];
		const byPerson: Record<number, CreditRow> = {};
		for (const c of credits) {
			let row = byPerson[c.personId];
			if (!row) {
				row = newCreditRow({ name: c.name, tmdbPersonId: c.tmdbPersonId });
				byPerson[c.personId] = row;
				rows.push(row);
			}
			if (!row.roles.includes(c.role)) row.roles.push(c.role);
			if (c.role === PERFORMER && c.character) row.character = c.character;
		}
		return rows;
	}

	/** Default roles in their usual order, then custom ones as added: the same
	 *  person reads "Writer · Director · Performer" whichever was ticked first. */
	function orderedRoles(roles: string[]): string[] {
		const known = defaultRoles.filter((r) => roles.includes(r));
		const custom = roles.filter((r) => !defaultRoles.includes(r));
		return [...known, ...custom];
	}

	/** Rows → request: one credit per role, rows in order. A row without a name
	 *  is an unused line and is left out. */
	export function creditRowsToRequest(rows: CreditRow[]): LiveCreditRequest[] {
		return rows
			.filter((r) => r.name.trim())
			.flatMap((r) =>
				orderedRoles(r.roles)
					.map((role) => role.trim())
					.filter(Boolean)
					.map((role) => {
						const credit: LiveCreditRequest = {
							name: r.name.trim(),
							role,
							tmdbPersonId: r.tmdbPersonId,
						};
						// The character belongs to the performing, not to the
						// person's other roles.
						const character = r.character.trim();
						if (role === PERFORMER && character) credit.character = character;
						return credit;
					}),
			);
	}

	/** Names compared like the server does: case, accents and extra spaces
	 *  ignored. */
	function foldName(s: string): string {
		return s
			.normalize("NFD")
			.replace(/\p{M}/gu, "")
			.replace(/\s+/g, " ")
			.trim()
			.toLocaleLowerCase();
	}

	/** Adds picked people to the rows: someone already there only gains the
	 *  roles they lacked (their TMDB link and character are kept), anyone else
	 *  is appended. Nothing is removed. Returns a new array and leaves the
	 *  given rows untouched (a changed row is a copy with the same key). */
	export function mergeIntoRows(
		rows: CreditRow[],
		picked: { name: string; roles: string[] }[],
	): CreditRow[] {
		const out = [...rows];
		for (const p of picked) {
			const name = p.name.replace(/\s+/g, " ").trim();
			if (!name) continue;
			const roles = p.roles.map((r) => r.trim()).filter(Boolean);
			const key = foldName(name);
			const i = out.findIndex((r) => foldName(r.name) === key);
			if (i < 0) {
				out.push(newCreditRow({ name, roles: dedupeRoles([], roles) }));
				continue;
			}
			const row = out[i];
			const merged = dedupeRoles(row.roles, roles);
			if (merged.length !== row.roles.length)
				out[i] = { ...row, roles: merged };
		}
		return out;
	}

	/** `base` followed by the roles of `extra` it lacks (case ignored). */
	function dedupeRoles(base: string[], extra: string[]): string[] {
		const out = [...base];
		for (const role of extra) {
			if (!out.some((x) => x.toLowerCase() === role.toLowerCase()))
				out.push(role);
		}
		return out;
	}

	/** A message when a named row has no role; undefined when all is well. */
	export function validateCreditRows(rows: CreditRow[]): string | undefined {
		if (rows.some((r) => r.name.trim() && !r.roles.some((x) => x.trim())))
			return "Each artist needs at least one role (Performer, Director…).";
	}
</script>

<script lang="ts">
	import { tick } from "svelte";
	import Icon from "@/lib/Icon.svelte";
	import type { LiveCreditSuggestion } from "@/types";
	import { errorText, parseCredits } from "./api";
	import ArtistSuggest from "./ArtistSuggest.svelte";
	import CreditSuggestions from "./CreditSuggestions.svelte";

	interface Props {
		rows: CreditRow[];
	}

	let { rows = $bindable() }: Props = $props();

	const uid = $props.id();
	// Rows currently typing a role that isn't in the list ("Other…").
	let addingOther: Record<number, boolean> = $state({});
	let otherText: Record<number, string> = $state({});

	// "Paste a list": the text stays until the artists are added, so a
	// dismissed reading can be fixed and read again.
	let pasteOpen = $state(false);
	let pasteText = $state("");
	let reading = $state(false);
	let pasteError: string | undefined = $state();
	// null: not read yet (or the text changed since).
	let pasteResult: LiveCreditSuggestion[] | null = $state(null);

	const PASTE_MAX = 20000;
	const pasteId = `${uid}-paste`;

	const nameInputId = (key: number) => `${uid}-name-${key}`;
	const otherInputId = (key: number) => `${uid}-other-${key}`;

	async function add() {
		// "Performer" is by far the most common credit. Copying the line above
		// would also copy a writer-director's roles onto the next performer.
		const row = newCreditRow({ roles: [PERFORMER] });
		rows.push(row);
		await tick();
		document.getElementById(nameInputId(row.key))?.focus();
	}

	function toggleRole(row: CreditRow, role: string) {
		const i = row.roles.indexOf(role);
		if (i >= 0) row.roles.splice(i, 1);
		else row.roles.push(role);
	}

	async function startOther(row: CreditRow) {
		addingOther[row.key] = true;
		otherText[row.key] = "";
		await tick();
		document.getElementById(otherInputId(row.key))?.focus();
	}

	function commitOther(row: CreditRow) {
		const text = (otherText[row.key] ?? "").replace(/\s+/g, " ").trim();
		addingOther[row.key] = false;
		otherText[row.key] = "";
		if (!text) return;
		// Typed a role that's in the list: tick it rather than add a twin.
		const known = defaultRoles.find(
			(r) => r.toLowerCase() === text.toLowerCase(),
		);
		const role = known ?? text.slice(0, 100);
		if (!row.roles.some((r) => r.toLowerCase() === role.toLowerCase()))
			row.roles.push(role);
	}

	function otherKeydown(ev: KeyboardEvent, row: CreditRow) {
		if (ev.key === "Enter") {
			// Inside the show form: Enter must add the role, not submit.
			ev.preventDefault();
			commitOther(row);
		} else if (ev.key === "Escape") {
			addingOther[row.key] = false;
			otherText[row.key] = "";
		}
	}

	async function openPaste() {
		pasteOpen = true;
		await tick();
		document.getElementById(pasteId)?.focus();
	}

	function closePaste() {
		pasteOpen = false;
		pasteText = "";
		pasteError = undefined;
		pasteResult = null;
	}

	async function readPaste() {
		if (reading) return;
		const text = pasteText.trim();
		if (!text) {
			pasteError = "Paste the names first.";
			return;
		}
		reading = true;
		pasteError = undefined;
		pasteResult = null;
		try {
			const r = await parseCredits(text);
			pasteResult = r.credits ?? [];
		} catch (err) {
			console.error("LiveCreditsEditor: parse-credits failed", err);
			pasteError = errorText(err);
		} finally {
			reading = false;
		}
	}

	function pasteKeydown(ev: KeyboardEvent) {
		// Enter is a new line; Ctrl/Cmd+Enter reads. Never submits the form.
		if (ev.key === "Enter" && (ev.ctrlKey || ev.metaKey) && !ev.isComposing) {
			ev.preventDefault();
			readPaste();
		}
	}

	function remove(i: number) {
		rows.splice(i, 1);
	}

	function move(i: number, by: -1 | 1) {
		const j = i + by;
		if (j < 0 || j >= rows.length) return;
		const [r] = rows.splice(i, 1);
		rows.splice(j, 0, r);
	}
</script>

<fieldset class="credits-editor">
	<legend>Artists</legend>

	{#if rows.length}
		<ol>
			{#each rows as row, i (row.key)}
				<li>
					<div class="name">
						<ArtistSuggest
							id={nameInputId(row.key)}
							bind:value={row.name}
							placeholder="Name"
							ariaLabel="Name"
							onType={() => {
								// The link was to the person that was picked; a
								// name edited by hand may be someone else.
								row.tmdbPersonId = null;
							}}
							onPick={(p) => {
								row.name = p.name;
								row.tmdbPersonId = p.tmdbPersonId;
							}}
						/>
						{#if row.tmdbPersonId}
							<span class="tmdb-badge" title="Linked to a TMDB person">
								TMDB
								<button
									type="button"
									class="plain"
									aria-label="Unlink from TMDB"
									title="Unlink from TMDB"
									onclick={() => (row.tmdbPersonId = null)}
								>
									×
								</button>
							</span>
						{/if}
					</div>
					<div class="row-actions">
						<button
							type="button"
							class="plain"
							aria-label="Move up"
							title="Move up"
							disabled={i === 0}
							onclick={() => move(i, -1)}
						>
							<Icon i="chevron" wh={14} facing="up" />
						</button>
						<button
							type="button"
							class="plain"
							aria-label="Move down"
							title="Move down"
							disabled={i === rows.length - 1}
							onclick={() => move(i, 1)}
						>
							<Icon i="chevron" wh={14} facing="down" />
						</button>
						<button
							type="button"
							class="plain"
							aria-label="Remove"
							title="Remove"
							onclick={() => remove(i)}
						>
							<Icon i="close" wh={14} />
						</button>
					</div>
					<div class="roles" role="group" aria-label="Roles">
						{#each defaultRoles as r (r)}
							<button
								type="button"
								class="chip"
								aria-pressed={row.roles.includes(r)}
								onclick={() => toggleRole(row, r)}
							>
								{r}
							</button>
						{/each}
						{#each row.roles.filter((r) => !defaultRoles.includes(r)) as r (r)}
							<span class="chip custom">
								{r}
								<button
									type="button"
									class="plain"
									aria-label={`Remove the role ${r}`}
									title="Remove this role"
									onclick={() => toggleRole(row, r)}
								>
									×
								</button>
							</span>
						{/each}
						{#if addingOther[row.key]}
							<input
								id={otherInputId(row.key)}
								class="other-input"
								type="text"
								bind:value={otherText[row.key]}
								placeholder="Other role"
								aria-label="Other role"
								autocomplete="off"
								maxlength={100}
								onkeydown={(ev) => otherKeydown(ev, row)}
								onblur={() => commitOther(row)}
							/>
						{:else}
							<button
								type="button"
								class="chip other"
								onclick={() => startOther(row)}
							>
								Other…
							</button>
						{/if}
					</div>
					{#if row.roles.includes(PERFORMER)}
						<label class="character">
							<span aria-hidden="true">as</span>
							<input
								type="text"
								bind:value={row.character}
								placeholder="Character (optional)"
								aria-label="Character played"
								autocomplete="off"
								maxlength={200}
							/>
						</label>
					{/if}
				</li>
			{/each}
		</ol>
	{/if}

	<div class="add-row">
		<button type="button" class="add" onclick={add}>
			<Icon i="add" wh={14} /> Add an artist
		</button>
		{#if !pasteOpen}
			<button type="button" class="add" onclick={openPaste}>
				Paste a list
			</button>
		{/if}
	</div>

	{#if pasteOpen}
		<div class="paste">
			<label for={pasteId}>Paste a list of artists</label>
			<textarea
				id={pasteId}
				bind:value={pasteText}
				oninput={() => {
					// What was read no longer matches the text.
					pasteResult = null;
					pasteError = undefined;
				}}
				onkeydown={pasteKeydown}
				rows={5}
				maxlength={PASTE_MAX}
				placeholder="avec Jeanne Martin, Paul Durand&#10;mise en scène Marie Dupont"
			></textarea>
			{#if pasteError}
				<p class="error" role="alert">{pasteError}</p>
			{/if}
			{#if pasteResult && pasteResult.length}
				<CreditSuggestions
					suggestions={pasteResult}
					onAdd={(picked) => {
						rows = mergeIntoRows(rows, picked);
						closePaste();
					}}
					onDismiss={() => (pasteResult = null)}
				/>
			{:else}
				{#if pasteResult}
					<p class="help" role="status">No artists recognised in this text.</p>
				{/if}
				<div class="paste-actions">
					<button type="button" class="secondary" onclick={closePaste}>
						Cancel
					</button>
					<button
						type="button"
						onclick={readPaste}
						disabled={reading || !pasteText.trim()}
						aria-busy={reading}
					>
						{reading ? "Reading…" : "Read"}
					</button>
				</div>
			{/if}
		</div>
	{/if}
</fieldset>

<style lang="scss">
	.credits-editor {
		display: flex;
		flex-flow: column;
		gap: 8px;
		min-width: 0;
		margin: 0;
		padding: 0;
		border: 0;

		legend {
			margin-bottom: 5px;
			padding: 0;
			font-size: 14px;
			font-weight: bold;
		}
	}

	ol {
		display: flex;
		flex-flow: column;
		gap: 8px;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	// Name and its buttons on one line, the person's roles underneath.
	li {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: 6px;
		padding-bottom: 8px;
		border-bottom: 1px solid $bg-color-accent;

		.roles,
		.character {
			grid-column: 1 / -1;
		}
	}

	.character {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 13px;
		color: $text-color-accent;

		input {
			flex: 0 1 260px;
			min-width: 0;
			padding: 3px 10px;
			font-size: 13px;
			color: $text-color;
			background-color: transparent;
			border-color: $text-color-accent;
		}
	}

	.roles {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 5px;
	}

	.chip {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		width: auto;
		padding: 3px 10px;
		border: 1px solid $text-color-accent;
		border-radius: 12px;
		background-color: transparent;
		font-size: 13px;
		color: $text-color;

		// Hover only tints: a pressed role is filled, so the two never look
		// alike (on touch, hover sticks to the last tapped chip).
		@media (hover: hover) {
			&:hover {
				background-color: $bg-color-accent;
			}
		}

		&[aria-pressed="true"],
		&.custom {
			border-color: $text-color;
			background-color: $text-color;
			color: $bg-color;
		}

		&.custom {
			padding-right: 2px;

			button {
				padding: 0 6px;
				font-size: 14px;
				line-height: 1;
				color: $bg-color;
			}
		}

		&.other {
			border-style: dashed;
		}
	}

	.other-input {
		width: 160px;
		padding: 3px 10px;
		font-size: 13px;
		color: $text-color;
		background-color: transparent;
		border-color: $text-color;
	}

	.name {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
	}

	.tmdb-badge {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		flex: 0 0 auto;
		padding: 0 0 0 6px;
		border: 1px solid $bg-color-accent;
		border-radius: 8px;
		font-size: 11px;
		color: $text-color-accent;

		button {
			padding: 2px 6px;
			font-size: 14px;
			line-height: 1;
			color: $text-color-accent;

			@media (hover: hover) {
				&:hover {
					color: $text-color;
				}
			}
		}
	}

	.row-actions {
		display: flex;
		flex-flow: row;
		align-items: center;

		button {
			display: flex;
			align-items: center;
			justify-content: center;
			width: 30px;
			height: 30px;
			padding: 0;
			border-radius: 6px;
			fill: $text-color;

			&:disabled {
				opacity: 0.3;
				cursor: default;
			}

			@media (hover: hover) {
				&:not(:disabled):hover {
					background-color: $bg-color-accent;
				}
			}
		}
	}

	.add-row {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}

	.add {
		gap: 6px;
		width: auto;
		font-size: 14px;
	}

	.paste {
		display: flex;
		flex-flow: column;
		gap: 6px;
		padding: 10px;
		border: 1px solid $bg-color-accent;
		border-radius: 8px;

		label {
			font-size: 14px;
			font-weight: bold;
		}

		textarea {
			resize: vertical;
			font: inherit;
			font-size: 14px;
		}

		.help {
			font-size: 13px;
			color: $text-color-accent;
		}

		.error {
			padding: 8px 10px;
			border-radius: 8px;
			color: white;
			background-color: $error;
		}
	}

	.paste-actions {
		display: flex;
		flex-wrap: wrap;
		justify-content: flex-end;
		gap: 8px;

		button {
			width: auto;
			min-width: 100px;
		}
	}
</style>
