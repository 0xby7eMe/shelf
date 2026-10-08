import { getPrefs, subscribePrefs, type PosterSize } from "@/lib/prefs"

export const POSTER_MIN: Record<PosterSize, number> = { small: 135, medium: 160, large: 200 }

function apply() {
	const p = getPrefs()
	const root = document.documentElement
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
