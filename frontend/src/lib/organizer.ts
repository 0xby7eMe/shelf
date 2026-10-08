import { useSyncExternalStore } from "react"

import {
	CreateCollection,
	DeleteCollection,
	DeleteTag,
	GetOrganization,
	RenameCollection,
	SetGameCollections,
	SetGameTags,
} from "../../wailsjs/go/main/App"
import { library } from "../../wailsjs/go/models"
import { toast } from "@/lib/toast"

// Collections and tags, kept by the backend. One copy is shared by the header
// menu, the game sheet and the settings page.

/** What the library is narrowed down to, besides the filter tabs. */
export type Group = { kind: "collection"; id: string } | { kind: "tag"; tag: string } | null

const EMPTY = library.Organization.createFrom({ collections: [], tags: {} })

let state: library.Organization = EMPTY
const listeners = new Set<() => void>()

function set(next: library.Organization) {
	state = library.Organization.createFrom({ collections: next.collections ?? [], tags: next.tags ?? {} })
	listeners.forEach((l) => l())
}

GetOrganization()
	.then(set)
	.catch(() => {})

export function useOrganizer(): library.Organization {
	return useSyncExternalStore(
		(cb) => {
			listeners.add(cb)
			return () => listeners.delete(cb)
		},
		() => state
	)
}

// Runs a change and shows the backend's refusal, such as a duplicate name, as a toast.
async function run(what: string, change: () => Promise<library.Organization>): Promise<library.Organization | null> {
	try {
		const next = await change()
		set(next)
		return state
	} catch (e) {
		toast.error(`Couldn't ${what}`, { description: String(e) })
		return null
	}
}

/** Makes a collection and returns its id, or null if it was refused. */
export async function createCollection(name: string): Promise<string | null> {
	const next = await run("create the collection", () => CreateCollection(name))
	return next?.collections[next.collections.length - 1]?.id ?? null
}
export const renameCollection = (id: string, name: string) =>
	run("rename the collection", () => RenameCollection(id, name))
export const deleteCollection = (id: string) => run("delete the collection", () => DeleteCollection(id))
export const setGameCollections = (gameId: string, ids: string[]) =>
	run("change the collections", () => SetGameCollections(gameId, ids))
export const setGameTags = (gameId: string, tags: string[]) => run("change the tags", () => SetGameTags(gameId, tags))
export const deleteTag = (tag: string) => run("delete the tag", () => DeleteTag(tag))

/** The ids of the games in a group, or null for no group. */
export function groupMembers(org: library.Organization, group: Group): Set<string> | null {
	if (!group) return null
	if (group.kind === "collection") {
		const c = org.collections.find((c) => c.id === group.id)
		return new Set(c?.games ?? [])
	}
	const ids = Object.entries(org.tags)
		.filter(([, tags]) => tags.includes(group.tag))
		.map(([id]) => id)
	return new Set(ids)
}

/** Every tag in use and how many games carry it, most used first. */
export function tagCounts(org: library.Organization): { tag: string; count: number }[] {
	const counts = new Map<string, number>()
	for (const tags of Object.values(org.tags)) {
		for (const t of tags) counts.set(t, (counts.get(t) ?? 0) + 1)
	}
	return [...counts.entries()]
		.map(([tag, count]) => ({ tag, count }))
		.sort((a, b) => b.count - a.count || a.tag.localeCompare(b.tag))
}

export function groupLabel(org: library.Organization, group: Group): string {
	if (!group) return ""
	if (group.kind === "tag") return `#${group.tag}`
	return org.collections.find((c) => c.id === group.id)?.name ?? ""
}
