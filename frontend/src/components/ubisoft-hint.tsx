import { useCallback, useEffect, useState } from "react"
import { ExternalLink, RefreshCw, Settings } from "lucide-react"

import { GetUbisoftStatus, UbisoftOpenConnect, UbisoftSync } from "../../wailsjs/go/main/App"
import { epic } from "../../wailsjs/go/models"
import { EventsOn } from "../../wailsjs/runtime/runtime"
import { toast } from "@/lib/toast"

// Shown in place of the library when Ubisoft has no games to list, saying why
// and what to do. Ubisoft's library comes from Connect, so it stays empty
// until Connect is set up, signed in and has loaded the games.
export function UbisoftHint({ onSettings }: { onSettings: () => void }) {
	const [status, setStatus] = useState<epic.UbisoftStatus | null>(null)

	const reload = useCallback(() => GetUbisoftStatus().then(setStatus).catch(() => {}), [])

	useEffect(() => {
		reload()
		const off = EventsOn("library:changed", reload)
		// Connect opening or closing doesn't announce itself.
		const t = setInterval(reload, 3000)
		return () => {
			off()
			clearInterval(t)
		}
	}, [reload])

	function act(task: () => Promise<unknown>, what: string) {
		task().catch((e) => toast.error(`Couldn't ${what}`, { description: String(e) }))
	}

	if (!status) return null

	let title: string
	let body: string
	let actions: React.ReactNode

	const pill =
		"inline-flex h-10 items-center justify-center gap-2 rounded-full px-5 text-xs font-medium ring-1 transition"
	const solid = `${pill} bg-solid text-solid-foreground ring-transparent accent-glow`
	const soft = `${pill} bg-white/5 text-white ring-white/10 hover:bg-white/10`

	if (!status.connectInstalled) {
		title = "Ubisoft Connect isn't set up"
		body = "Your Ubisoft games come from Ubisoft Connect, which Shelf runs in its own Proton prefix. Set it up first."
		actions = (
			<button className={solid} onClick={onSettings}>
				<Settings className="size-3.5" />
				Set up Ubisoft Connect
			</button>
		)
	} else if (!status.signedIn) {
		title = status.running ? "Sign in to Ubisoft Connect" : "Sign in to see your games"
		body = status.running
			? "Ubisoft Connect is open. Sign in there, two-step verification included. Your games appear here on their own once it has loaded them."
			: "Ubisoft only lets its own launcher sign in. Open Ubisoft Connect, sign in, and your games will appear here."
		actions = (
			<>
				{!status.running && (
					<button className={solid} onClick={() => act(UbisoftOpenConnect, "open Ubisoft Connect")}>
						<ExternalLink className="size-3.5" />
						Open Ubisoft Connect
					</button>
				)}
				<button className={soft} onClick={onSettings}>
					<Settings className="size-3.5" />
					Settings
				</button>
			</>
		)
	} else {
		title = "Ubisoft Connect hasn't listed any games"
		body = status.running
			? "You're signed in, but Connect hasn't recorded your library yet. It can take a minute after signing in; this page updates by itself."
			: "You're signed in, but Connect hasn't recorded a library for this account. Open it for a minute so it can load your games, then refresh."
		actions = (
			<>
				<button className={solid} onClick={() => act(UbisoftSync, "read the library")}>
					<RefreshCw className="size-3.5" />
					Refresh
				</button>
				{!status.running && (
					<button className={soft} onClick={() => act(UbisoftOpenConnect, "open Ubisoft Connect")}>
						<ExternalLink className="size-3.5" />
						Open Ubisoft Connect
					</button>
				)}
			</>
		)
	}

	return (
		<div className="mx-auto max-w-md space-y-4 pt-20 text-center">
			<h3 className="text-base font-semibold tracking-tight">{title}</h3>
			<p className="text-sm text-muted-foreground">{body}</p>
			<div className="flex flex-wrap justify-center gap-2 pt-1">{actions}</div>
		</div>
	)
}
