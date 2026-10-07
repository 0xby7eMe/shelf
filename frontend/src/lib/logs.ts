import { useSyncExternalStore } from "react"

import { ClearLogs, GetLogs } from "../../wailsjs/go/main/App"
import { applog } from "../../wailsjs/go/models"
import { EventsOn } from "../../wailsjs/runtime/runtime"

// The log window's data: a mirror of the backend's rolling log, fed by events.

const MAX_LINES = 3000

let lines: applog.Line[] = []
let open = false
const listeners = new Set<() => void>()
const emit = () => listeners.forEach((l) => l())

function subscribe(cb: () => void) {
	listeners.add(cb)
	return () => listeners.delete(cb)
}

/** Starts mirroring the backend log. Returns a function that stops it. */
export function startLogs(): () => void {
	let live = true
	GetLogs()
		.then((initial) => {
			if (!live) return
			// Events may have arrived while loading; the snapshot already holds those lines.
			const known = new Set(lines.map((l) => `${l.time}|${l.source}|${l.text}`))
			lines = [...(initial ?? []).filter((l) => !known.has(`${l.time}|${l.source}|${l.text}`)), ...lines].slice(-MAX_LINES)
			emit()
		})
		.catch(() => {})

	const off = EventsOn("log:lines", (batch: applog.Line[]) => {
		lines = [...lines, ...batch].slice(-MAX_LINES)
		emit()
	})
	return () => {
		live = false
		off()
	}
}

export function useLogLines(): applog.Line[] {
	return useSyncExternalStore(subscribe, () => lines)
}

export function useLogOpen(): boolean {
	return useSyncExternalStore(subscribe, () => open)
}

export function setLogOpen(next: boolean) {
	open = next
	emit()
}

export function clearLogs() {
	lines = []
	emit()
	ClearLogs().catch(() => {})
}
