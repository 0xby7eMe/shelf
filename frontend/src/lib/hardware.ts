import { useEffect, useState } from "react"

import { HardwareHistory, HardwareInfo, HardwareStart, HardwareStop } from "../../wailsjs/go/main/App"
import { sysmon } from "../../wailsjs/go/models"
import { EventsOn } from "../../wailsjs/runtime/runtime"

/** How many seconds the graphs show, like a task manager's sixty-second window. */
export const WINDOW = 60

// How often the view tells the backend it is still there. The backend stops by
// itself if it hears nothing for a while, so a closed window costs nothing.
const KEEPALIVE_MS = 5000

export interface Hardware {
	info: sysmon.Info | null
	/** Oldest first, at most WINDOW of them. */
	samples: sysmon.Sample[]
	/** Counts every sample received, so graphs can scroll their grid with time. */
	ticks: number
}

/**
 * Live hardware readings for as long as the component is mounted. Sampling
 * starts with the first call and ends when the view goes away.
 */
export function useHardware(): Hardware {
	const [info, setInfo] = useState<sysmon.Info | null>(null)
	const [samples, setSamples] = useState<sysmon.Sample[]>([])
	const [ticks, setTicks] = useState(0)

	useEffect(() => {
		let alive = true
		HardwareInfo()
			.then((i) => alive && setInfo(i))
			.catch(() => {})

		const off = EventsOn("hardware:sample", (s: sysmon.Sample) => {
			setSamples((prev) => [...prev.slice(-(WINDOW - 1)), s])
			setTicks((n) => n + 1)
		})

		HardwareStart()
		// The past the backend already has, so reopening doesn't start from nothing.
		HardwareHistory()
			.then((h) => {
				if (!alive || !h?.length) return
				setSamples((prev) => {
					const seen = new Set(prev.map((p) => p.time))
					return [...h.filter((x) => !seen.has(x.time)), ...prev].slice(-WINDOW)
				})
				setTicks((n) => n + h.length)
			})
			.catch(() => {})
		const keepalive = setInterval(() => HardwareStart(), KEEPALIVE_MS)

		return () => {
			alive = false
			off()
			clearInterval(keepalive)
			HardwareStop()
		}
	}, [])

	return { info, samples, ticks }
}

/** The values of one reading across the samples, for a graph. */
export function series(samples: sysmon.Sample[], pick: (s: sysmon.Sample) => number | null | undefined): (number | null)[] {
	return samples.map((s) => pick(s) ?? null)
}

/** Rounds a rate graph's ceiling up to a tidy number, so its scale doesn't jump on every spike. */
export function niceMax(peak: number, floor: number): number {
	const v = Math.max(peak, floor)
	const pow = 10 ** Math.floor(Math.log10(v))
	for (const m of [1, 2, 5, 10]) if (v <= m * pow) return m * pow
	return 10 * pow
}

/** Colours for each kind of hardware, so a tile and its graph agree. */
export const COLORS = {
	cpu: "#5aa9ff",
	memory: "#b68cff",
	disk: "#6fd08c",
	disk2: "#4fb3a9",
	net: "#ffb454",
	net2: "#ff8a5c",
	gpu: "#ff7a8a",
} as const
