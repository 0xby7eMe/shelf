import { useSyncExternalStore } from "react"

export type PosterSize = "small" | "medium" | "large"

// App preferences that only matter to this front end. They live in localStorage,
// which can be unavailable, so every access is guarded and defaults always work.
export interface Prefs {
	gamepad: boolean
	sounds: boolean
	volume: number // 0-100
	logWindow: boolean // show the log window button
	logHeight: number // px
	hardwareMonitor: boolean // show the hardware monitor button

	accent: string // "#rrggbb", the colour of buttons, switches and progress bars
	posterSize: PosterSize
	heroBanner: boolean // the big banner for the game played last
	shelfRecent: boolean // the "Continue playing" shelf
	shelfUnplayed: boolean // the "Never played" shelf
	uiScale: number // percent, 85-130
	reduceMotion: boolean
}

const KEY = "shelf:prefs"

function osReducesMotion(): boolean {
	try {
		return window.matchMedia("(prefers-reduced-motion: reduce)").matches
	} catch {
		return false
	}
}

export const DEFAULT_ACCENT = "#ffffff"
export const UI_SCALES = [90, 100, 110, 125]

const defaults: Prefs = {
	gamepad: true,
	sounds: true,
	volume: 80,
	logWindow: false,
	logHeight: 280,
	hardwareMonitor: false,

	accent: DEFAULT_ACCENT,
	posterSize: "medium",
	heroBanner: true,
	shelfRecent: true,
	shelfUnplayed: true,
	uiScale: 100,
	reduceMotion: osReducesMotion(),
}

const clamp = (n: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, n))

// Each saved value is checked on its way in, so a hand-edited or older file
// can never put the interface into a state it can't draw.
const validators: { [K in keyof Prefs]: (v: unknown) => Prefs[K] | undefined } = {
	gamepad: bool,
	sounds: bool,
	volume: (v) => (typeof v === "number" ? clamp(v, 0, 100) : undefined),
	logWindow: bool,
	logHeight: (v) => (typeof v === "number" ? clamp(v, 120, 900) : undefined),
	hardwareMonitor: bool,

	accent: (v) => (typeof v === "string" && /^#[0-9a-f]{6}$/i.test(v) ? v.toLowerCase() : undefined),
	posterSize: (v) => (v === "small" || v === "medium" || v === "large" ? v : undefined),
	heroBanner: bool,
	shelfRecent: bool,
	shelfUnplayed: bool,
	uiScale: (v) => (typeof v === "number" ? clamp(Math.round(v), 85, 130) : undefined),
	reduceMotion: bool,
}

function bool(v: unknown): boolean | undefined {
	return typeof v === "boolean" ? v : undefined
}

function load(): Prefs {
	const prefs: Prefs = { ...defaults }
	try {
		const raw = localStorage.getItem(KEY)
		if (raw) {
			const saved = JSON.parse(raw)
			for (const key of Object.keys(defaults) as (keyof Prefs)[]) {
				const value = validators[key](saved?.[key])
				if (value !== undefined) (prefs[key] as Prefs[typeof key]) = value
			}
		}
	} catch {
		// fall through to defaults
	}
	return prefs
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

// Puts the appearance settings back to how Shelf ships.
export function resetAppearance() {
	setPrefs({
		accent: defaults.accent,
		posterSize: defaults.posterSize,
		heroBanner: defaults.heroBanner,
		shelfRecent: defaults.shelfRecent,
		shelfUnplayed: defaults.shelfUnplayed,
		uiScale: defaults.uiScale,
		reduceMotion: osReducesMotion(),
	})
}

export function subscribePrefs(cb: () => void): () => void {
	listeners.add(cb)
	return () => listeners.delete(cb)
}

export function usePrefs(): Prefs {
	return useSyncExternalStore(subscribePrefs, () => current)
}
