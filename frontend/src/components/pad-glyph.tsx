import { usePadKind, type PadKind } from "@/lib/gamepad"
import { cn } from "@/lib/utils"

// What a button does in Shelf, not where it sits. Each controller family
// draws the button that is in that place on its own pad.
export type PadAction =
	| "confirm" // bottom face button
	| "back" // right face button
	| "favorite" // left face button
	| "random" // top face button
	| "tabs" // both bumpers
	| "menu" // Start / Options / +
	| "search" // Select / Create / View / -

const FACE = {
	xbox: {
		confirm: { label: "A", color: "#6dbd45" },
		back: { label: "B", color: "#e2403b" },
		favorite: { label: "Y", color: "#f2c230" },
		random: { label: "X", color: "#3d8fe0" },
	},
	// The symbols keep PlayStation's colours; they are drawn, not typed.
	playstation: {
		confirm: { label: "cross", color: "#6aa6e8" },
		back: { label: "circle", color: "#e0535f" },
		favorite: { label: "triangle", color: "#5bc7a0" },
		random: { label: "square", color: "#d98cc6" },
	},
	// Nintendo's A and B are swapped against Xbox's: the bottom button is B.
	nintendo: {
		confirm: { label: "B", color: "#e8e8e8" },
		back: { label: "A", color: "#e8e8e8" },
		favorite: { label: "Y", color: "#e8e8e8" },
		random: { label: "X", color: "#e8e8e8" },
	},
} as const

const BUMPERS: Record<PadKind, [string, string]> = {
	xbox: ["LB", "RB"],
	playstation: ["L1", "R1"],
	nintendo: ["L", "R"],
}

const MENU: Record<PadKind, string> = { xbox: "menu", playstation: "Options", nintendo: "+" }
const SEARCH: Record<PadKind, string> = { xbox: "view", playstation: "Create", nintendo: "−" }

function Symbol({ name, color }: { name: string; color: string }) {
	const common = { fill: "none", stroke: color, strokeWidth: 2.2, strokeLinecap: "round", strokeLinejoin: "round" } as const
	return (
		<svg viewBox="0 0 24 24" className="size-3.5" aria-hidden>
			{name === "cross" && <path d="M7 7l10 10M17 7L7 17" {...common} />}
			{name === "circle" && <circle cx="12" cy="12" r="6" {...common} />}
			{name === "square" && <rect x="6.5" y="6.5" width="11" height="11" rx="1" {...common} />}
			{name === "triangle" && <path d="M12 5.5l7 12.5H5z" {...common} />}
		</svg>
	)
}

function Face({ kind, action }: { kind: PadKind; action: "confirm" | "back" | "favorite" | "random" }) {
	const { label, color } = FACE[kind][action]
	const symbol = kind === "playstation"
	return (
		<span
			className="grid size-5 shrink-0 place-items-center rounded-full bg-black/40 text-[10px] leading-none font-bold ring-1 ring-white/25"
			style={symbol ? undefined : { color }}
			title={label}
		>
			{symbol ? <Symbol name={label} color={color} /> : label}
		</span>
	)
}

function Pill({ children, className }: { children: React.ReactNode; className?: string }) {
	return (
		<span
			className={cn(
				"grid h-5 min-w-6 shrink-0 place-items-center rounded-md bg-white/15 px-1.5 text-[10px] leading-none font-semibold text-white ring-1 ring-white/10",
				className
			)}
		>
			{children}
		</span>
	)
}

// Xbox's menu button is three lines, its view button two overlapping windows.
function MenuIcon() {
	return (
		<svg viewBox="0 0 24 24" className="size-3.5" aria-hidden fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round">
			<path d="M5 7h14M5 12h14M5 17h14" />
		</svg>
	)
}

function ViewIcon() {
	return (
		<svg viewBox="0 0 24 24" className="size-3.5" aria-hidden fill="none" stroke="currentColor" strokeWidth="2" strokeLinejoin="round">
			<rect x="4" y="8" width="10" height="10" rx="1.5" />
			<path d="M9 8V6.5A1.5 1.5 0 0 1 10.5 5H18a1.5 1.5 0 0 1 1.5 1.5V14a1.5 1.5 0 0 1-1.5 1.5h-1.5" />
		</svg>
	)
}

/** The glyph for an action, drawn for the connected controller (or the given family). */
export function PadGlyph({ action, kind }: { action: PadAction; kind?: PadKind }) {
	const detected = usePadKind()
	const k = kind ?? detected

	switch (action) {
		case "confirm":
		case "back":
		case "favorite":
		case "random":
			return <Face kind={k} action={action} />
		case "tabs": {
			const [l, r] = BUMPERS[k]
			return (
				<span className="flex items-center gap-1">
					<Pill>{l}</Pill>
					<Pill>{r}</Pill>
				</span>
			)
		}
		case "menu":
			return <Pill>{k === "xbox" ? <MenuIcon /> : MENU[k]}</Pill>
		case "search":
			return <Pill>{k === "xbox" ? <ViewIcon /> : SEARCH[k]}</Pill>
	}
}
