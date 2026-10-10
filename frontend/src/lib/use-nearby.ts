import { useCallback, useEffect, useRef, useState } from "react"

import { GetNearby, SetNearbySettings } from "../../wailsjs/go/main/App"
import { nearby } from "../../wailsjs/go/models"
import { EventsOn } from "../../wailsjs/runtime/runtime"

// The other Shelfs on the network. Asking is local, so it follows every
// "nearby:changed" event instead of polling.
export function useNearby() {
	const [snapshot, setSnapshot] = useState<nearby.Snapshot | null>(null)
	const [error, setError] = useState("")
	const alive = useRef(true)

	useEffect(() => {
		alive.current = true
		return () => {
			alive.current = false
		}
	}, [])

	const run = useCallback(async (call: () => Promise<nearby.Snapshot>) => {
		try {
			const next = await call()
			if (alive.current) {
				setSnapshot(next)
				setError("")
			}
		} catch (e) {
			if (alive.current) setError(String(e))
		}
	}, [])

	const reload = useCallback(() => run(GetNearby), [run])

	useEffect(() => {
		reload()
		return EventsOn("nearby:changed", reload)
	}, [reload])

	return {
		snapshot,
		error,
		save: (s: nearby.Settings) => run(() => SetNearbySettings(s)),
	}
}
