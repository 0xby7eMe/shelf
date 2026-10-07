import { useCallback, useEffect, useMemo, useRef, useState } from "react"

import { EpicQueue, EpicRepair, GetEpicAccount, Launch, UbisoftInstalling } from "../../wailsjs/go/main/App"
import { epic, library } from "../../wailsjs/go/models"
import { EventsOn } from "../../wailsjs/runtime/runtime"
import { setLogOpen } from "@/lib/logs"
import { getPrefs } from "@/lib/prefs"
import { formatBytes } from "@/lib/format"
import { toast } from "@/lib/toast"

// Payloads of backend events that aren't part of the generated bindings.
interface SaveEvent {
	appName: string
	phase: "before" | "after" | "manual"
	state: "downloaded" | "uploaded" | "failed" | "unchanged"
	error?: string
}

interface UpdatesEvent {
	titles: string[]
	auto: boolean
}

const verbs: Record<string, { doing: string; failed: string }> = {
	install: { doing: "Installing", failed: "install" },
	update: { doing: "Updating", failed: "update" },
	repair: { doing: "Repairing", failed: "repair" },
	verify: { doing: "Verifying", failed: "verify" },
	import: { doing: "Importing", failed: "import" },
}

// Label for a running or queued job, e.g. "Updating".
export function jobLabel(job: epic.Progress): string {
	return job.state === "queued" ? "Queued" : (verbs[job.kind]?.doing ?? "Working")
}

// Epic account state plus the jobs running right now, keyed by app name.
// Events from the backend turn into toasts here.
export function useEpic(games: library.Game[] | null, onReviewImport: () => void) {
	const [account, setAccount] = useState<epic.Account | null>(null)
	const [queue, setQueue] = useState<epic.QueueState>(() => new epic.QueueState({ jobs: [], paused: false }))
	const [ubisoft, setUbisoft] = useState<epic.UbisoftInstalling[]>([])
	const ubisoftRef = useRef<epic.UbisoftInstalling[]>([])
	const gamesRef = useRef(games)
	gamesRef.current = games
	const reviewRef = useRef(onReviewImport)
	reviewRef.current = onReviewImport

	const reloadAccount = useCallback(
		() =>
			GetEpicAccount()
				.then(setAccount)
				.catch(() => {}),
		[]
	)

	useEffect(() => {
		reloadAccount()
		EpicQueue()
			.then((q) => setQueue(new epic.QueueState({ jobs: q.jobs ?? [], paused: q.paused })))
			.catch(() => {})

		// Structure changes (added, started, finished, reordered) arrive as a whole list.
		const offQueue = EventsOn("epic:queue", (q: epic.QueueState) =>
			setQueue(new epic.QueueState({ jobs: q.jobs ?? [], paused: q.paused }))
		)

		const titleOf = (appName: string) =>
			gamesRef.current?.find((g) => g.id === `epic:${appName}`)?.name ?? appName
		const play = (appName: string) => ({
			label: "Play",
			onClick: () => Launch(`epic:${appName}`).catch((e) => toast.error(String(e))),
		})

		const offInstall = EventsOn("epic:install", (p: epic.Progress) => {
			// Progress ticks only update a job that is already listed.
			setQueue((prev) => {
				if (p.state !== "installing" && p.state !== "queued") return prev
				if (!prev.jobs.some((j) => j.appName === p.appName)) return prev
				return new epic.QueueState({
					jobs: prev.jobs.map((j) => (j.appName === p.appName ? { ...p, position: j.position } : j)),
					paused: prev.paused,
				})
			})

			const title = titleOf(p.appName)
			if (p.state === "done") {
				switch (p.kind) {
					case "install":
						toast.success(`${title} is installed`, { description: "Ready to play.", action: play(p.appName) })
						break
					case "update":
						toast.success(`${title} is up to date`, { action: play(p.appName) })
						break
					case "repair":
						toast.success(`${title} repaired`, { description: "Damaged files were downloaded again." })
						break
					case "verify":
						toast.success(`${title} verified`, { description: "All files are intact." })
						break
					case "import":
						toast.success(`${title} imported`, { description: "Using the existing files.", action: play(p.appName) })
						break
				}
			} else if (p.state === "failed") {
				if (p.kind === "verify" && p.damaged) {
					toast.error(`${title} has damaged files`, {
						description: p.error,
						action: {
							label: "Repair",
							onClick: () => EpicRepair(p.appName).catch((e) => toast.error(String(e))),
						},
					})
				} else {
					toast.error(`Couldn't ${verbs[p.kind]?.failed ?? "finish"} ${title}`, {
						description: p.error || "Unknown error",
					})
				}
			} else if (p.state === "cancelled") {
				toast.info(`${verbs[p.kind]?.doing ?? "Job"} ${title} cancelled`)
			}
		})

		const offLaunch = EventsOn("epic:launch-error", (e: { message: string }) => {
			toast.error("Game failed to start", { description: e.message })
			if (getPrefs().logWindow) setLogOpen(true)
		})

		const offSaves = EventsOn("epic:saves", (e: SaveEvent) => {
			const title = titleOf(e.appName)
			if (e.state === "failed") {
				toast.error(`Cloud save sync failed for ${title}`, { description: e.error })
			} else if (e.state === "downloaded") {
				toast.info("Cloud save downloaded", { description: title, duration: 3500 })
			} else if (e.state === "uploaded") {
				toast.success("Saves uploaded to the cloud", { description: title, duration: 4000 })
			} else if (e.phase === "manual") {
				toast.info("Saves are already in sync", { description: title, duration: 3500 })
			}
		})

		const offUpdates = EventsOn("epic:updates", (e: UpdatesEvent) => {
			const n = e.titles.length
			toast.info(n === 1 ? "1 game update available" : `${n} game updates available`, {
				description: `${e.titles.slice(0, 3).join(", ")}${n > 3 ? ` and ${n - 3} more` : ""}${e.auto ? ". Updating now." : ""}`,
			})
		})

		const offImport = EventsOn("epic:importable", (n: number) =>
			toast.info(n === 1 ? "Found 1 installed Epic game" : `Found ${n} installed Epic games`, {
				description: "Use them without downloading again.",
				action: { label: "Review", onClick: () => reviewRef.current() },
				duration: 12000,
			})
		)

		// Ubisoft Connect does its own downloads. All Shelf can see is that one is
		// going and how much has been written, so it shows as an install of unknown size.
		const ubisoftNow = (list: epic.UbisoftInstalling[]) => {
			const gone = ubisoftRef.current.filter((p) => !list.some((n) => n.key === p.key))
			ubisoftRef.current = list
			setUbisoft(list)
			for (const g of gone) {
				// Finished or interrupted? The library says which, once it has caught up.
				setTimeout(() => {
					const game = gamesRef.current?.find((x) => x.externalId === g.key && x.source === "ubisoft")
					if (!game) return
					if (game.installed) {
						toast.success(`${game.name} is installed`, {
							description: "Ready to play.",
							action: { label: "Play", onClick: () => Launch(game.id).catch((e) => toast.error(String(e))) },
						})
					} else {
						toast.info(`Download of ${game.name} stopped`, {
							description: "Ubisoft Connect closed before it finished. Press Install to continue.",
						})
					}
				}, 2500)
			}
		}
		UbisoftInstalling().then((l) => ubisoftNow(l ?? [])).catch(() => {})
		const offUbisoft = EventsOn("ubisoft:installing", (l: epic.UbisoftInstalling[]) => ubisoftNow(l ?? []))

		const offLibrary = EventsOn("library:changed", reloadAccount)
		return () => {
			offQueue()
			offInstall()
			offLaunch()
			offSaves()
			offUpdates()
			offImport()
			offLibrary()
			offUbisoft()
		}
	}, [reloadAccount])

	const installs = useMemo(() => {
		const out = Object.fromEntries(queue.jobs.map((j) => [j.appName, j])) as Record<string, epic.Progress>
		for (const u of ubisoft) {
			out[u.key] = new epic.Progress({
				appName: u.key,
				kind: "install",
				state: "installing",
				percent: 0,
				indeterminate: true,
				speed: u.bytes > 0 ? `${formatBytes(u.bytes)} downloaded` : "",
			})
		}
		return out
	}, [queue, ubisoft])

	return { account, installs, queue, reloadAccount }
}
