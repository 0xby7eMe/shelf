import { useEffect, useRef, useSyncExternalStore } from "react"

import { setHover } from "@/lib/hover"
import { usePrefs } from "@/lib/prefs"
import { sfx } from "@/lib/sfx"

// Gamepad support: the standard-mapping buttons drive a spatial focus
// navigation over whatever is on screen. Dialogs, sheets and open selects
// trap it, so the controller never wanders behind an overlay.

export type Dir = "up" | "down" | "left" | "right"

export interface PadHandlers {
	/** B pressed with nothing open to close. */
	onBack?: () => void
	/** LB / RB. */
	onTab?: (dir: -1 | 1) => void
	/** Start. */
	onMenu?: () => void
	/** Y. */
	onRandom?: () => void
	/** Back/Select button. */
	onSearch?: () => void
}

const BTN = { A: 0, B: 1, X: 2, Y: 3, LB: 4, RB: 5, SELECT: 8, START: 9, UP: 12, DOWN: 13, LEFT: 14, RIGHT: 15 }
const DEADZONE = 0.55
const FIRST_REPEAT = 400
const REPEAT = 115

const FOCUSABLE = [
	"button",
	"a[href]",
	"input",
	"textarea",
	"select",
	'[role="switch"]',
	'[role="option"]',
	'[role="combobox"]',
	'[tabindex]:not([tabindex="-1"])',
].join(",")

const LAYERS = '[role="dialog"], [role="listbox"], [role="menu"]'

// --- input mode: lets the UI show focus rings and button hints only for a pad ---

export type InputMode = "mouse" | "pad"
let mode: InputMode = "mouse"
const modeListeners = new Set<() => void>()

function setMode(next: InputMode) {
	if (mode === next) return
	mode = next
	document.documentElement.dataset.input = next
	modeListeners.forEach((l) => l())
}

export function useInputMode(): InputMode {
	return useSyncExternalStore(
		(cb) => {
			modeListeners.add(cb)
			return () => modeListeners.delete(cb)
		},
		() => mode
	)
}

// --- connected pads ---

export function activePad(): Gamepad | null {
	const pads = navigator.getGamepads?.() ?? []
	for (const p of pads) if (p?.connected) return p
	return null
}

export function usePadName(): string | null {
	return useSyncExternalStore(
		(cb) => {
			window.addEventListener("gamepadconnected", cb)
			window.addEventListener("gamepaddisconnected", cb)
			return () => {
				window.removeEventListener("gamepadconnected", cb)
				window.removeEventListener("gamepaddisconnected", cb)
			}
		},
		() => activePad()?.id ?? null
	)
}

// --- which family of controller it is, so hints show the buttons it really has ---

export type PadKind = "xbox" | "playstation" | "nintendo"

/**
 * Tells the controller family from the id the browser reports, which carries
 * either "Vendor: 054c Product: 0ce6" or a leading "054c-0ce6-". The vendor id
 * is the reliable part; names vary by driver. Anything unknown is treated as
 * an Xbox-style pad, which is what the standard mapping is modelled on.
 */
export function padKind(id: string | null | undefined): PadKind {
	if (!id) return "xbox"
	const lower = id.toLowerCase()
	const vendor = /vendor:\s*([0-9a-f]{4})/.exec(lower)?.[1] ?? /^([0-9a-f]{4})-[0-9a-f]{4}-/.exec(lower)?.[1]
	if (vendor === "054c" || /playstation|dualshock|dualsense|sony|\bps[2345]\b/.test(lower)) return "playstation"
	if (vendor === "057e" || /nintendo|switch|joy-?con|pro controller/.test(lower)) return "nintendo"
	return "xbox"
}

/** The family of the pad in use. */
export function usePadKind(): PadKind {
	return padKind(usePadName())
}

/** Strips vendor/product ids from a pad id like "Xbox Controller (STANDARD GAMEPAD Vendor: 045e Product: 0b12)". */
export function padLabel(id: string): string {
	return id.replace(/\s*\(.*\)\s*$/, "").replace(/^[0-9a-f]{4}-[0-9a-f]{4}-/i, "") || id
}

// --- spatial navigation ---

function visible(el: HTMLElement): boolean {
	if (el.hasAttribute("disabled") || el.getAttribute("aria-disabled") === "true") return false
	if (el.hasAttribute("data-nav-skip") || el.closest("[data-nav-skip],[inert],[aria-hidden='true']")) return false
	const r = el.getBoundingClientRect()
	if (r.width < 2 || r.height < 2) return false
	return getComputedStyle(el).visibility !== "hidden"
}

function topLayer(): HTMLElement | null {
	const layers = [...document.querySelectorAll<HTMLElement>(LAYERS)].filter(visible)
	return layers.at(-1) ?? null
}

function candidates(): HTMLElement[] {
	const root: ParentNode = topLayer() ?? document
	return [...root.querySelectorAll<HTMLElement>(FOCUSABLE)].filter(visible)
}

function score(from: DOMRect, to: DOMRect, dir: Dir): number | null {
	const fx = from.left + from.width / 2
	const fy = from.top + from.height / 2
	const tx = to.left + to.width / 2
	const ty = to.top + to.height / 2
	const dx = tx - fx
	const dy = ty - fy

	let gap: number, along: number, across: number
	switch (dir) {
		case "right":
			if (dx <= 1) return null
			gap = to.left - from.right
			along = dx
			across = Math.abs(dy)
			break
		case "left":
			if (dx >= -1) return null
			gap = from.left - to.right
			along = -dx
			across = Math.abs(dy)
			break
		case "down":
			if (dy <= 1) return null
			gap = to.top - from.bottom
			along = dy
			across = Math.abs(dx)
			break
		case "up":
			if (dy >= -1) return null
			gap = from.top - to.bottom
			along = -dy
			across = Math.abs(dx)
			break
	}
	// Stay roughly in line: inside a 45 degree cone, or overlapping on the
	// other axis (so a wide search box is still reachable from a poster below it).
	const horizontal = dir === "left" || dir === "right"
	const overlaps = horizontal
		? from.top < to.bottom && to.top < from.bottom
		: from.left < to.right && to.left < from.right
	if (!overlaps && across > along * 1.2) return null

	// Prefer near items in line with the current one; being off-axis costs double.
	return Math.max(0, gap) + along * 0.05 + across * 2
}

function focusEl(el: HTMLElement) {
	el.focus({ preventScroll: true })
	// The focused card takes the hover look, so only one card is ever raised.
	setHover(el.closest(".group"))
	el.scrollIntoView({ block: "center", inline: "center", behavior: "smooth" })
}

function scrollParent(el: Element | null): HTMLElement | null {
	for (let n = el?.parentElement ?? null; n; n = n.parentElement) {
		const oy = getComputedStyle(n).overflowY
		if ((oy === "auto" || oy === "scroll") && n.scrollHeight > n.clientHeight) return n
	}
	return document.querySelector<HTMLElement>("[data-scroll-root]")
}

function move(dir: Dir): boolean {
	const list = candidates()
	if (list.length === 0) return false

	const cur = document.activeElement as HTMLElement | null
	const inList = cur && list.includes(cur)
	if (!inList) {
		// Nothing focused yet: start with the first item that is on screen.
		const onScreen = list.find((el) => {
			const r = el.getBoundingClientRect()
			return r.top >= 0 && r.bottom <= window.innerHeight
		})
		focusEl(onScreen ?? list[0])
		return true
	}

	const from = cur.getBoundingClientRect()
	let best: HTMLElement | null = null
	let bestScore = Infinity
	for (const el of list) {
		if (el === cur) continue
		const s = score(from, el.getBoundingClientRect(), dir)
		if (s !== null && s < bestScore) {
			best = el
			bestScore = s
		}
	}
	if (!best) return false
	focusEl(best)
	return true
}

function pressEscape() {
	const target = document.activeElement ?? document.body
	target.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }))
}

// --- the polling loop ---

export function useGamepad(handlers: PadHandlers) {
	const { gamepad } = usePrefs()
	const ref = useRef(handlers)
	ref.current = handlers

	useEffect(() => {
		if (!gamepad) return

		let raf = 0
		const prev: boolean[] = []
		const held: Partial<Record<Dir, { since: number; last: number }>> = {}

		const onMouse = () => setMode("mouse")
		window.addEventListener("pointermove", onMouse, { passive: true })

		const press = (i: number) => {
			const h = ref.current
			switch (i) {
				case BTN.A: {
					const el = document.activeElement as HTMLElement | null
					if (el && el !== document.body) {
						sfx.confirm()
						el.click()
					} else if (move("down")) sfx.move()
					break
				}
				case BTN.B:
					if (topLayer()) {
						sfx.back()
						pressEscape()
					} else if (h.onBack) {
						sfx.back()
						h.onBack()
					}
					break
				case BTN.X: {
					const heart = (document.activeElement as HTMLElement | null)
						?.closest(".group")
						?.querySelector<HTMLElement>("[data-fav]")
					if (heart) {
						sfx.confirm()
						heart.click()
					}
					break
				}
				case BTN.Y:
					if (h.onRandom) {
						sfx.confirm()
						h.onRandom()
					}
					break
				case BTN.LB:
				case BTN.RB:
					if (h.onTab) {
						sfx.tab()
						h.onTab(i === BTN.LB ? -1 : 1)
					}
					break
				case BTN.START:
					if (h.onMenu) {
						sfx.confirm()
						h.onMenu()
					}
					break
				case BTN.SELECT:
					if (h.onSearch) {
						sfx.confirm()
						h.onSearch()
					}
					break
			}
		}

		const step = (dir: Dir) => {
			if (move(dir)) sfx.move(dir)
			else sfx.blocked()
		}

		const tick = (now: number) => {
			raf = requestAnimationFrame(tick)
			const pad = activePad()
			if (!pad) return

			let used = false
			pad.buttons.forEach((b, i) => {
				if (b.pressed && !prev[i]) {
					used = true
					if (i < BTN.UP || i > BTN.RIGHT) press(i)
				}
				prev[i] = b.pressed
			})

			// D-pad and left stick share one direction with key-style repeat.
			const ax = pad.axes[0] ?? 0
			const ay = pad.axes[1] ?? 0
			const want: Partial<Record<Dir, boolean>> = {
				up: !!pad.buttons[BTN.UP]?.pressed || ay < -DEADZONE,
				down: !!pad.buttons[BTN.DOWN]?.pressed || ay > DEADZONE,
				left: !!pad.buttons[BTN.LEFT]?.pressed || ax < -DEADZONE,
				right: !!pad.buttons[BTN.RIGHT]?.pressed || ax > DEADZONE,
			}
			// Diagonals would fire two moves at once; keep the stronger stick axis.
			if (Math.abs(ax) > DEADZONE && Math.abs(ay) > DEADZONE && !pad.buttons[BTN.UP]?.pressed && !pad.buttons[BTN.DOWN]?.pressed) {
				if (Math.abs(ax) > Math.abs(ay)) want.up = want.down = false
				else want.left = want.right = false
			}
			for (const dir of ["up", "down", "left", "right"] as Dir[]) {
				const h = held[dir]
				if (!want[dir]) {
					delete held[dir]
					continue
				}
				used = true
				if (!h) {
					held[dir] = { since: now, last: now }
					step(dir)
				} else if (now - h.since > FIRST_REPEAT && now - h.last > REPEAT) {
					h.last = now
					step(dir)
				}
			}

			// Right stick scrolls whatever the focus sits in.
			const sy = pad.axes[3] ?? 0
			if (Math.abs(sy) > 0.2) {
				used = true
				scrollParent(document.activeElement)?.scrollBy({ top: sy * 22 })
			}

			if (used) setMode("pad")
		}
		raf = requestAnimationFrame(tick)

		return () => {
			cancelAnimationFrame(raf)
			window.removeEventListener("pointermove", onMouse)
			setMode("mouse")
		}
	}, [gamepad])
}
