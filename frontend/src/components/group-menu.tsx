import { useState } from "react"
import { Folder, Tag } from "lucide-react"

import { Menu, Row, Heading } from "@/components/header-menus"
import { groupLabel, tagCounts, useOrganizer, type Group } from "@/lib/organizer"
import { cn } from "@/lib/utils"

/** Narrows the library to one collection or tag. */
export function GroupMenu({ group, onGroup }: { group: Group; onGroup: (g: Group) => void }) {
	const [open, setOpen] = useState(false)
	const org = useOrganizer()
	const tags = tagCounts(org)
	const label = groupLabel(org, group)
	const empty = org.collections.length === 0 && tags.length === 0

	const isCollection = (id: string) => group?.kind === "collection" && group.id === id
	const isTag = (tag: string) => group?.kind === "tag" && group.tag === tag

	function pick(g: Group) {
		onGroup(g)
		setOpen(false)
	}

	return (
		<Menu
			open={open}
			onOpenChange={setOpen}
			label={group ? `Showing ${label}` : "Collections and tags"}
			width="w-60"
			triggerClass={cn(
				"flex h-9 max-w-44 items-center gap-2 rounded-full px-3.5 text-xs ring-1 backdrop-blur-md transition hover:bg-white/10 hover:text-white",
				group ? "bg-white/10 text-white ring-white/15" : "bg-white/5 text-white/70 ring-white/5"
			)}
			trigger={
				<>
					{group?.kind === "tag" ? <Tag className="size-3.5 shrink-0" /> : <Folder className="size-3.5 shrink-0" />}
					<span className="hidden truncate md:inline">{group ? label : "Collections"}</span>
				</>
			}
		>
			{empty ? (
				<p className="px-3 py-3 text-xs leading-relaxed text-white/45">
					No collections or tags yet. Open a game and add it to a collection or give it a tag.
				</p>
			) : (
				<div className="max-h-80 overflow-y-auto">
					{org.collections.length > 0 && <Heading>Collections</Heading>}
					{org.collections.map((c) => (
						<Row
							key={c.id}
							icon={<Folder className="size-3.5" />}
							hint={String(c.games.length)}
							checked={isCollection(c.id)}
							onClick={() => pick({ kind: "collection", id: c.id })}
						>
							{c.name}
						</Row>
					))}
					{tags.length > 0 && <Heading>Tags</Heading>}
					{tags.map(({ tag, count }) => (
						<Row
							key={tag}
							icon={<Tag className="size-3.5" />}
							hint={String(count)}
							checked={isTag(tag)}
							onClick={() => pick({ kind: "tag", tag })}
						>
							{tag}
						</Row>
					))}
				</div>
			)}
			{group && (
				<div className="mt-1.5 border-t border-white/10 pt-1.5">
					<Row onClick={() => pick(null)}>Show everything</Row>
				</div>
			)}
		</Menu>
	)
}
