import { useEffect, useState } from "react"

import { GetStats } from "../../wailsjs/go/main/App"
import { library } from "../../wailsjs/go/models"
import { EventsOn } from "../../wailsjs/runtime/runtime"

export function useStats(weeks = 26) {
		const [stats, setStats] = useState<library.Stats | null>(null)

		useEffect(() => {
				let alive = true
				const load = () =>
				GetStats(weeks)
						.then((s) => alive && setStats(s))
						.catch(() => {})

				load()
				const off = EventsOn("history:changed", load)
				window.addEventListener("focus", load)

				return () => {
						alive = false
						off()
						window.removeEventListener("focus", load)
				}
		}, [weeks])

		return stats
}