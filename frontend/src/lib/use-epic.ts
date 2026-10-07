import { useCallback, useEffect, useRef, useState } from "react"

import { EpicInstallStates, GetEpicAccount, Launch } from "../../wailsjs/go/main/App"
import { epic, library } from "../../wailsjs/go/models"
import { EventsOn } from "../../wailsjs/runtime/runtime"
import { toast } from "@/lib/toast"

// Epic account state plus the installs running right now, keyed by app name.
export function useEpic(games: library.Game[] | null) {
	const [account, setAccount] = useState<epic.Account | null>(null)
	const [installs, setInstalls] = useState<Record<string, epic.Progress>>({})
	const gamesRef = useRef(games)
	gamesRef.current = games

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
			const title =
				gamesRef.current?.find((g) => g.id === `epic:${p.appName}`)?.name ?? p.appName
			if (p.state === "done") {
				toast.success(`${title} is installed`, {
					description: "Ready to play.",
					action: {
						label: "Play",
						onClick: () => Launch(`epic:${p.appName}`).catch((e) => toast.error(String(e))),
					},
				})
			} else if (p.state === "failed") {
				toast.error(`Couldn't install ${title}`, { description: p.error || "Unknown error" })
			} else if (p.state === "cancelled") {
				toast.info(`Install of ${title} cancelled`)
			}
		})
		const offLaunch = EventsOn("epic:launch-error", (e: { message: string }) =>
			toast.error("Game failed to start", { description: e.message })
		)
		const offLibrary = EventsOn("library:changed", reloadAccount)
		return () => {
			offInstall()
			offLaunch()
			offLibrary()
		}
	}, [reloadAccount])

	return { account, installs, reloadAccount }
}
