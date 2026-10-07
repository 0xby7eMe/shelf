import { useCallback, useEffect, useState } from "react"

import { GetBattlEyeInstall, GetBattlEyeRuntime, InstallBattlEyeRuntime } from "../../wailsjs/go/main/App"
import { epic } from "../../wailsjs/go/models"
import { EventsOn } from "../../wailsjs/runtime/runtime"
import { toast } from "@/lib/toast"

// Games that use BattlEye need Proton's BattlEye runtime to start. Steam
// carries it as a tool; otherwise Shelf downloads the same files. This is the
// state of it, and the action that installs it.
export function useBattlEyeRuntime() {
	const [runtime, setRuntime] = useState<epic.BattlEyeRuntime | null>(null)
	const [install, setInstall] = useState<epic.ProtonInstallState | null>(null)

	const reload = useCallback(() => GetBattlEyeRuntime().then(setRuntime).catch(() => {}), [])

	useEffect(() => {
		reload()
		GetBattlEyeInstall().then(setInstall).catch(() => {})
		const offInstall = EventsOn("battleye:install", (s: epic.ProtonInstallState) => {
			setInstall(s)
			if (s.state === "done") {
				reload()
				toast.success("BattlEye runtime installed", { description: "BattlEye games can start now." })
			}
		})
		const offLibrary = EventsOn("library:changed", reload)
		return () => {
			offInstall()
			offLibrary()
		}
	}, [reload])

	const installing = install?.state === "running"
	const start = useCallback(
		() => InstallBattlEyeRuntime().catch((e) => toast.error("Couldn't install the BattlEye runtime", { description: String(e) })),
		[]
	)

	return { runtime, install, installing, start }
}
