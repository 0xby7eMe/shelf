import { useCallback, useEffect, useRef, useState } from "react"

import { DisconnectFriends, GetFriends, SetFriendsConfig } from "../../wailsjs/go/main/App"
import { friends } from "../../wailsjs/go/models"

const REFRESH_MS = 60_000

// The friends list, refreshed for as long as `active` is true. Asking costs
// network requests, so nothing is polled while the dialog is closed.
export function useFriends(active: boolean) {
	const [snapshot, setSnapshot] = useState<friends.Snapshot | null>(null)
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState("")
	const alive = useRef(true)

	useEffect(() => {
		alive.current = true
		return () => {
			alive.current = false
		}
	}, [])

	const run = useCallback(async (call: () => Promise<friends.Snapshot>) => {
		setLoading(true)
		try {
			const next = await call()
			if (alive.current) {
				setSnapshot(next)
				setError("")
			}
		} catch (e) {
			if (alive.current) setError(String(e))
		} finally {
			if (alive.current) setLoading(false)
		}
	}, [])

	const refresh = useCallback(() => run(() => GetFriends(true)), [run])

	useEffect(() => {
		if (!active) return
		run(() => GetFriends(false))
		const timer = setInterval(() => run(() => GetFriends(false)), REFRESH_MS)
		return () => clearInterval(timer)
	}, [active, run])

	return {
		snapshot,
		loading,
		error,
		refresh,
		save: (source: string, values: Record<string, string>) => run(() => SetFriendsConfig(source, values)),
		disconnect: (source: string) => run(() => DisconnectFriends(source)),
	}
}
