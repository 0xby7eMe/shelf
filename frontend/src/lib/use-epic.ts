import { useCallback, useEffect, useState } from "react"

import { EpicInstallStates, GetEpicAccount } from "../../wailsjs/go/main/App"
import { epic } from "../../wailsjs/go/models"
import { EventsOn } from "../../wailsjs/runtime/runtime"

// Epic account state plus the installs running right now, keyed by app name.
export function useEpic() {
	const [account, setAccount] = useState<epic.Account | null>(null)
	const [installs, setInstalls] = useState<Record<string, epic.Progress>>({})
	const [notice, setNotice] = useState("")

	const reloadAccount = useCallback(
		() =>
			GetEpicAccount()
				.then(setAccount)
				.catch(() => {}),
		[]
	)

	useEffect(() => {
		reloadAccount()
		EpicInstallStates()
			.then((list) => setInstalls(Object.fromEntries((list ?? []).map((p) => [p.appName, p]))))
			.catch(() => {})

		const offInstall = EventsOn("epic:install", (p: epic.Progress) => {
			setInstalls((prev) => {
				const next = { ...prev }
				if (p.state === "installing") next[p.appName] = p
				else delete next[p.appName]
				return next
			})
			if (p.state === "failed") setNotice(`Install failed: ${p.error || "unknown error"}`)
		})
		const offLaunch = EventsOn("epic:launch-error", (e: { message: string }) =>
			setNotice(e.message)
		)
		const offLibrary = EventsOn("library:changed", reloadAccount)
		return () => {
			offInstall()
			offLaunch()
			offLibrary()
		}
	}, [reloadAccount])

	return { account, installs, notice, setNotice, reloadAccount }
}
