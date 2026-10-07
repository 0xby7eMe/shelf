import { useState } from "react"
import { Popover } from "@base-ui/react/popover"
import { Check, MoreHorizontal, SlidersHorizontal } from "lucide-react"

import { cn } from "@/lib/utils"

export type SortKey = "name" | "playtime" | "recent"
export type SourceFilter = "all" | "steam" | "epic" | "ubisoft"

export const SORTS: { id: SortKey; label: string }[] = [
	{ id: "name", label: "Name" },
	{ id: "playtime", label: "Most played" },
	{ id: "recent", label: "Recently played" },
]

export const SOURCES: { id: SourceFilter; label: string }[] = [
	{ id: "all", label: "All stores" },
	{ id: "steam", label: "Steam" },
	{ id: "epic", label: "Epic Games" },
	{ id: "ubisoft", label: "Ubisoft" },
]

/** The look of a round button in the header. */
export const headerButton =
	"grid size-9 place-items-center rounded-full bg-white/5 text-white/70 ring-1 ring-white/5 backdrop-blur-md transition hover:bg-white/10 hover:text-white"

// Animated the way the project's other popups are (tw-animate-css, keyed on data-open).
const popup =
	"rounded-2xl border border-white/10 bg-popover/85 p-1.5 text-popover-foreground shadow-2xl outline-none backdrop-blur-xl " +
	"origin-(--transform-origin) duration-100 data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 " +
	"data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95"

function Menu({
	open,
	onOpenChange,
	trigger,
	label,
	triggerClass,
	width,
	children,
}: {
	open: boolean
	onOpenChange: (open: boolean) => void
	trigger: React.ReactNode
	label: string
	triggerClass: string
	width: string
	children: React.ReactNode
}) {
	return (
		<Popover.Root open={open} onOpenChange={onOpenChange}>
			<Popover.Trigger aria-label={label} title={label} className={triggerClass}>
				{trigger}
			</Popover.Trigger>
			<Popover.Portal>
				<Popover.Positioner side="bottom" align="start" sideOffset={8} className="z-50">
					<Popover.Popup aria-label={label} className={cn(popup, width)}>
						{children}
					</Popover.Popup>
				</Popover.Positioner>
			</Popover.Portal>
		</Popover.Root>
	)
}

function Row({
	onClick,
	checked,
	icon,
	hint,
	children,
}: {
	onClick: () => void
	checked?: boolean
	icon?: React.ReactNode
	hint?: string
	children: React.ReactNode
}) {
	return (
		<button
			onClick={onClick}
			className="flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-xs text-white/80 outline-none transition hover:bg-white/10 hover:text-white focus-visible:bg-white/10"
		>
			{icon && <span className="grid size-4 shrink-0 place-items-center text-white/55">{icon}</span>}
			<span className="min-w-0 flex-1 truncate">{children}</span>
			{hint && (
				<kbd className="rounded border border-white/10 px-1.5 text-[10px] text-white/40">{hint}</kbd>
			)}
			{checked !== undefined && (
				<Check className={cn("size-3.5 shrink-0", checked ? "text-white" : "text-transparent")} />
			)}
		</button>
	)
}

function Heading({ children }: { children: React.ReactNode }) {
	return <p className="px-3 pt-2 pb-1 text-[10px] tracking-[0.2em] text-white/35 uppercase">{children}</p>
}

/**
 * Which store to show and how to order the games, in one place instead of two
 * dropdowns. A dot on the button says something other than the default is set.
 */
export function FilterMenu({
	source,
	onSource,
	sort,
	onSort,
}: {
	source: SourceFilter
	onSource: (s: SourceFilter) => void
	sort: SortKey
	onSort: (s: SortKey) => void
}) {
	const [open, setOpen] = useState(false)
	const active = source !== "all" || sort !== "name"
	const summary = [SOURCES.find((s) => s.id === source)?.label, SORTS.find((s) => s.id === sort)?.label]
		.filter(Boolean)
		.join(", ")

	return (
		<Menu
			open={open}
			onOpenChange={setOpen}
			label={`Store and order: ${summary}`}
			width="w-56"
			triggerClass={cn(
				"relative flex h-9 items-center gap-2 rounded-full px-3.5 text-xs ring-1 backdrop-blur-md transition hover:bg-white/10 hover:text-white",
				active ? "bg-white/10 text-white ring-white/15" : "bg-white/5 text-white/70 ring-white/5"
			)}
			trigger={
				<>
					<SlidersHorizontal className="size-3.5" />
					<span className="hidden md:inline">Filters</span>
					{active && <span className="absolute top-1.5 right-1.5 size-1.5 rounded-full bg-emerald-400" />}
				</>
			}
		>
			<Heading>Store</Heading>
			{SOURCES.map((s) => (
				<Row key={s.id} checked={source === s.id} onClick={() => onSource(s.id)}>
					{s.label}
				</Row>
			))}
			<Heading>Sort by</Heading>
			{SORTS.map((s) => (
				<Row key={s.id} checked={sort === s.id} onClick={() => onSort(s.id)}>
					{s.label}
				</Row>
			))}
			{active && (
				<div className="mt-1.5 border-t border-white/10 pt-1.5">
					<Row
						onClick={() => {
							onSource("all")
							onSort("name")
						}}
					>
						Reset
					</Row>
				</div>
			)}
		</Menu>
	)
}

export interface MoreItem {
	id: string
	label: string
	icon: React.ReactNode
	hint?: string
	/** Shows a check mark, for things that are switched on. */
	checked?: boolean
	onSelect: () => void
}

/** The actions used now and then, kept out of the bar. */
export function MoreMenu({ items }: { items: MoreItem[] }) {
	const [open, setOpen] = useState(false)
	return (
		<Menu
			open={open}
			onOpenChange={setOpen}
			label="More"
			width="w-60"
			triggerClass={cn(headerButton, open && "bg-white/15 text-white")}
			trigger={<MoreHorizontal className="size-4" />}
		>
			{items.map((item) => (
				<Row
					key={item.id}
					icon={item.icon}
					hint={item.hint}
					checked={item.checked}
					onClick={() => {
						setOpen(false)
						item.onSelect()
					}}
				>
					{item.label}
				</Row>
			))}
		</Menu>
	)
}
