import { useRef, useState } from "react"
import { Check, Plus, X } from "lucide-react"

import { library } from "../../wailsjs/go/models"
import {
	createCollection,
	setGameCollections,
	setGameTags,
	tagCounts,
	useOrganizer,
} from "@/lib/organizer"
import { cn } from "@/lib/utils"

const chip = "inline-flex h-7 items-center gap-1.5 rounded-full px-3 text-xs ring-1 transition"

/** Collections and tags of one game, edited in place. */
export function GameOrganizer({ game }: { game: library.Game }) {
	const org = useOrganizer()
	const [newName, setNewName] = useState<string | null>(null)
	const [tagText, setTagText] = useState("")
	// Enter and the blur that follows it must not create the collection twice.
	const creating = useRef(false)

	const mine = org.collections.filter((c) => c.games.includes(game.id)).map((c) => c.id)
	const tags = org.tags[game.id] ?? []
	const suggestions = tagCounts(org)
		.map((t) => t.tag)
		.filter((t) => !tags.includes(t))
		.slice(0, 6)

	function toggle(id: string) {
		setGameCollections(game.id, mine.includes(id) ? mine.filter((m) => m !== id) : [...mine, id])
	}

	async function addCollection() {
		const name = (newName ?? "").trim()
		if (creating.current) return
		if (!name) {
			setNewName(null)
			return
		}
		creating.current = true
		const id = await createCollection(name)
		creating.current = false
		if (id) {
			setNewName(null)
			setGameCollections(game.id, [...mine, id])
		}
	}

	function addTags(text: string) {
		const added = text.split(",").map((t) => t.trim()).filter(Boolean)
		if (added.length) setGameTags(game.id, [...tags, ...added])
		setTagText("")
	}

	return (
		<div className="space-y-5">
			<div>
				<p className="mb-2 text-[11px] tracking-wide text-white/45 uppercase">Collections</p>
				<div className="flex flex-wrap gap-1.5">
					{org.collections.map((c) => {
						const on = mine.includes(c.id)
						return (
							<button
								key={c.id}
								onClick={() => toggle(c.id)}
								aria-pressed={on}
								className={cn(
									chip,
									on ? "bg-white/15 text-white ring-white/25" : "bg-white/5 text-white/70 ring-white/10 hover:bg-white/10 hover:text-white"
								)}
							>
								{on && <Check className="size-3" />}
								{c.name}
							</button>
						)
					})}
					{newName === null ? (
						<button
							onClick={() => setNewName("")}
							className={cn(chip, "bg-transparent text-white/55 ring-white/10 hover:bg-white/5 hover:text-white")}
						>
							<Plus className="size-3" />
							New collection
						</button>
					) : (
						<input
							autoFocus
							value={newName}
							maxLength={40}
							placeholder="Name"
							onChange={(e) => setNewName(e.target.value)}
							onKeyDown={(e) => {
								if (e.key === "Enter") addCollection()
								if (e.key === "Escape") {
									e.stopPropagation()
									setNewName(null)
								}
							}}
							onBlur={addCollection}
							className="h-7 w-36 rounded-full bg-white/10 px-3 text-xs ring-1 ring-white/20 outline-none placeholder:text-white/35"
						/>
					)}
				</div>
			</div>

			<div>
				<p className="mb-2 text-[11px] tracking-wide text-white/45 uppercase">Tags</p>
				<div className="flex flex-wrap items-center gap-1.5">
					{tags.map((t) => (
						<span key={t} className={cn(chip, "bg-white/10 pr-1.5 text-white ring-white/15")}>
							{t}
							<button
								onClick={() => setGameTags(game.id, tags.filter((x) => x !== t))}
								aria-label={`Remove tag ${t}`}
								className="grid size-4 place-items-center rounded-full text-white/55 hover:bg-white/15 hover:text-white"
							>
								<X className="size-3" />
							</button>
						</span>
					))}
					<input
						value={tagText}
						maxLength={24}
						placeholder={tags.length ? "Add tag" : "Add a tag, e.g. co-op"}
						onChange={(e) => {
							const v = e.target.value
							if (v.endsWith(",")) addTags(v)
							else setTagText(v)
						}}
						onKeyDown={(e) => {
							if (e.key === "Enter") addTags(tagText)
							if (e.key === "Backspace" && !tagText && tags.length) setGameTags(game.id, tags.slice(0, -1))
						}}
						onBlur={() => tagText.trim() && addTags(tagText)}
						className="h-7 min-w-28 flex-1 rounded-full bg-transparent px-2 text-xs outline-none placeholder:text-white/35"
					/>
				</div>
				{suggestions.length > 0 && (
					<div className="mt-2 flex flex-wrap gap-1.5">
						{suggestions.map((t) => (
							<button
								key={t}
								onClick={() => setGameTags(game.id, [...tags, t])}
								className={cn(chip, "h-6 bg-transparent px-2.5 text-white/45 ring-white/10 hover:bg-white/5 hover:text-white")}
							>
								+ {t}
							</button>
						))}
					</div>
				)}
			</div>
		</div>
	)
}
