import { useState } from "react"
import { Check, Folder, Pencil, Plus, Tag, Trash2, X } from "lucide-react"

import { library } from "../../../wailsjs/go/models"
import { Input } from "@/components/ui/input"
import { IconButton, Panel, PillButton, SectionHeading, inputClass } from "@/components/settings/ui"
import { confirm } from "@/lib/confirm"
import {
	createCollection,
	deleteCollection,
	deleteTag,
	renameCollection,
	tagCounts,
	useOrganizer,
} from "@/lib/organizer"

// Make, rename and delete collections, and clean up tags. Filing a game happens
// in its detail sheet; this is for looking after the groups themselves.
export function CollectionsSection({ games }: { games: library.Game[] }) {
	const org = useOrganizer()
	const [name, setName] = useState("")
	const [editing, setEditing] = useState<{ id: string; name: string } | null>(null)
	const tags = tagCounts(org)

	// Games that are no longer in the library still count in the file; show what's there now.
	const present = new Set(games.map((g) => g.id))

	async function add() {
		if (name.trim() && (await createCollection(name))) setName("")
	}

	async function saveRename() {
		if (!editing) return
		if (await renameCollection(editing.id, editing.name)) setEditing(null)
	}

	async function remove(id: string, label: string) {
		const ok = await confirm({
			title: `Delete "${label}"?`,
			description: "The collection goes away. The games in it stay in your library.",
			confirmLabel: "Delete",
			destructive: true,
		})
		if (ok) deleteCollection(id)
	}

	async function removeTag(tag: string, count: number) {
		const ok = await confirm({
			title: `Remove the tag "${tag}"?`,
			description: `It is taken off ${count === 1 ? "1 game" : `${count} games`}.`,
			confirmLabel: "Remove",
			destructive: true,
		})
		if (ok) deleteTag(tag)
	}

	return (
		<div className="space-y-6">
			<SectionHeading
				title="Collections"
				hint="Group your games your own way. Add a game to a collection or tag it from its detail sheet."
			/>

			<Panel>
				<form
					onSubmit={(e) => {
						e.preventDefault()
						add()
					}}
					className="flex gap-2"
				>
					<Input
						value={name}
						maxLength={40}
						onChange={(e) => setName(e.target.value)}
						placeholder="New collection, e.g. Co-op nights"
						className={inputClass}
					/>
					<PillButton type="submit" variant="solid" disabled={!name.trim()}>
						<Plus className="size-3.5" />
						Create
					</PillButton>
				</form>

				{org.collections.length === 0 ? (
					<p className="text-sm text-white/40">No collections yet.</p>
				) : (
					<ul className="space-y-1.5">
						{org.collections.map((c) => {
							const count = c.games.filter((id) => present.has(id)).length
							return (
								<li key={c.id} className="flex items-center gap-2 rounded-xl bg-white/[0.04] px-3 py-2 ring-1 ring-white/[0.06]">
									<Folder className="size-4 shrink-0 text-white/45" />
									{editing?.id === c.id ? (
										<>
											<Input
												autoFocus
												value={editing.name}
												maxLength={40}
												onChange={(e) => setEditing({ id: c.id, name: e.target.value })}
												onKeyDown={(e) => {
													if (e.key === "Enter") saveRename()
													if (e.key === "Escape") {
														e.stopPropagation()
														setEditing(null)
													}
												}}
												className="h-8 flex-1 rounded-full border-0 bg-white/10 px-3 text-sm shadow-none ring-1 ring-white/15"
											/>
											<IconButton label="Save name" onClick={saveRename}>
												<Check className="size-3.5" />
											</IconButton>
											<IconButton label="Cancel" onClick={() => setEditing(null)}>
												<X className="size-3.5" />
											</IconButton>
										</>
									) : (
										<>
											<span className="min-w-0 flex-1 truncate text-sm">{c.name}</span>
											<span className="text-xs tabular-nums text-white/40">
												{count} {count === 1 ? "game" : "games"}
											</span>
											<IconButton label={`Rename ${c.name}`} onClick={() => setEditing({ id: c.id, name: c.name })}>
												<Pencil className="size-3.5" />
											</IconButton>
											<IconButton label={`Delete ${c.name}`} onClick={() => remove(c.id, c.name)}>
												<Trash2 className="size-3.5" />
											</IconButton>
										</>
									)}
								</li>
							)
						})}
					</ul>
				)}
			</Panel>

			<Panel>
				<p className="text-[11px] tracking-wide text-white/45 uppercase">Tags</p>
				{tags.length === 0 ? (
					<p className="text-sm text-white/40">No tags yet.</p>
				) : (
					<div className="flex flex-wrap gap-2">
						{tags.map(({ tag, count }) => (
							<span
								key={tag}
								className="inline-flex h-8 items-center gap-2 rounded-full bg-white/5 pr-1.5 pl-3 text-xs text-white/80 ring-1 ring-white/10"
							>
								<Tag className="size-3 text-white/45" />
								{tag}
								<span className="tabular-nums text-white/40">{count}</span>
								<button
									onClick={() => removeTag(tag, count)}
									aria-label={`Remove tag ${tag} from every game`}
									title="Remove from every game"
									className="grid size-5 place-items-center rounded-full text-white/45 hover:bg-white/15 hover:text-white"
								>
									<X className="size-3" />
								</button>
							</span>
						))}
					</div>
				)}
			</Panel>
		</div>
	)
}
