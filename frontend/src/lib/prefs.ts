import { useSyncExternalStore } from "react"

// App preferences that only matter to this front end. They live in localStorage,
// which can be unavailable, so every access is guarded and defaults always work.
export interface Prefs {
	gamepad: boolean
	sounds: boolean
	volume: number // 0-100
}

const KEY = "shelf:prefs"
const defaults: Prefs = { gamepad: true, sounds: true, volume: 60 }

function load(): Prefs {
	try {
		const raw = localStorage.getItem(KEY)
		if (raw) {
			const saved = JSON.parse(raw)
			return {
				gamepad: typeof saved.gamepad === "boolean" ? saved.gamepad : defaults.gamepad,
				sounds: typeof saved.sounds === "boolean" ? saved.sounds : defaults.sounds,
				volume:
					typeof saved.volume === "number"
						? Math.min(100, Math.max(0, saved.volume))
						: defaults.volume,
			}
		}
	} catch {
		// fall through to defaults
	}
	return { ...defaults }
}

let current = load()
const listeners = new Set<() => void>()

export function getPrefs(): Prefs {
	return current
}

export function setPrefs(patch: Partial<Prefs>) {
	current = { ...current, ...patch }
	try {
		localStorage.setItem(KEY, JSON.stringify(current))
	} catch {
		// not persisted this time
	}
	listeners.forEach((l) => l())
}

export function usePrefs(): Prefs {
	return useSyncExternalStore(
		(cb) => {
			listeners.add(cb)
			return () => listeners.delete(cb)
		},
		() => current
	)
}
