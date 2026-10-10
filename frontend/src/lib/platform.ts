import { useSyncExternalStore } from "react"

import { GetPlatform } from "../../wailsjs/go/main/App"
import { main } from "../../wailsjs/go/models"

// Which parts of Shelf work on this system. Until the backend has answered,
// everything counts as available, as it is on Linux.
const everything = new main.Platform({
	os: "linux",
	windowsGames: true,
	proton: true,
	hardwareMonitor: true,
	desktop: true,
	cloudSaves: true,
})

let current = everything
const listeners = new Set<() => void>()

GetPlatform()
	.then((p) => {
		current = p
		listeners.forEach((l) => l())
	})
	.catch(() => {})

export function platformNow(): main.Platform {
	return current
}

export function usePlatform(): main.Platform {
	return useSyncExternalStore(
		(cb) => {
			listeners.add(cb)
			return () => listeners.delete(cb)
		},
		() => current
	)
}
