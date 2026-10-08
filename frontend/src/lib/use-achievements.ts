import { useCallback, useEffect, useState } from "react"

import { GetAchievementOverview, ScanAchievements } from "../../wailsjs/go/main/App"
import { achievements } from "../../wailsjs/go/models"
import { EventsOn } from "../../wailsjs/runtime/runtime"

// The achievement overview. Opening it starts a scan of the played games, which
// skips what was looked at recently, and results arrive as events.
export function useAchievements(active: boolean) {
	const [overview, setOverview] = useState<achievements.Overview | null>(null)

	useEffect(() => {
		if (!active) return
		let alive = true
		const load = () =>
			GetAchievementOverview()
				.then((o) => alive && setOverview(o))
				.catch(() => {})

		ScanAchievements(false)
			.then((o) => alive && setOverview(o))
			.catch(() => {})
		const off = EventsOn("achievements:changed", load)
		return () => {
			alive = false
			off()
		}
	}, [active])

	const rescan = useCallback(() => {
		ScanAchievements(true)
			.then(setOverview)
			.catch(() => {})
	}, [])

	return { overview, rescan }
}
