import { getPrefs, subscribePrefs, type PosterSize } from "@/lib/prefs"

export const ACCENTS: { name: string; color: string }[] = [
	{ name: "White", color: "#ffffff" },
	{ name: "Sky", color: "#7dd3fc" },
	{ name: "Blue", color: "#93a8ff" },
	{ name: "Violet", color: "#c4a7ff" },
	{ name: "Pink", color: "#f9a8d4" },
	{ name: "Rose", color: "#fda4af" },
	{ name: "Amber", color: "#fcd34d" },
	{ name: "Lime", color: "#bef264" },
	{ name: "Mint", color: "#6ee7b7" },
]

export const POSTER_MIN: Record<PosterSize, number> = { small: 135, medium: 160, large: 200 }

// Black or white, whichever reads better on the colour, by its perceived brightness.
export function readableOn(hex: string): string {
	const n = parseInt(hex.slice(1), 16)
	const r = (n >> 16) & 255
	const g = (n >> 8) & 255
	const b = n & 255
	return (r * 299 + g * 587 + b * 114) / 1000 > 150 ? "#000000" : "#ffffff"
}

function apply() {
	const p = getPrefs()
	const root = document.documentElement
	root.style.setProperty("--shelf-accent", p.accent)
	root.style.setProperty("--shelf-accent-fg", readableOn(p.accent))
	root.style.setProperty("--poster-min", `${POSTER_MIN[p.posterSize]}px`)
	// Tailwind sizes things in rem, so this scales text and spacing together.
	root.style.fontSize = `${(p.uiScale / 100) * 16}px`
	root.dataset.reduceMotion = String(p.reduceMotion)
}

// Applies the appearance settings now and whenever they change.
export function startAppearance() {
	apply()
	return subscribePrefs(apply)
}
