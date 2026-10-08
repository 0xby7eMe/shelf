import { useEffect, useState } from "react"

import {
	CheckForUpdates,
	GetUpdateStatus,
	InstallUpdate,
	RestartApp,
	SetAutoUpdateCheck,
	SkipUpdate,
} from "../../../wailsjs/go/main/App"
import { update } from "../../../wailsjs/go/models"
import { BrowserOpenURL, EventsOn } from "../../../wailsjs/runtime/runtime"
import { ToggleRow } from "@/components/toggle-row"
import { Bar, Panel, PillButton, SectionHeading } from "@/components/settings/ui"
import { formatLastPlayed } from "@/lib/format"
import { toast } from "@/lib/toast"

export function AboutSection() {
	const [status, setStatus] = useState<update.Status | null>(null)
	const [checking, setChecking] = useState(false)
	const [installing, setInstalling] = useState(false)
	const [progress, setProgress] = useState<{ done: number; total: number } | null>(null)
	const [installed, setInstalled] = useState("")
	const [error, setError] = useState("")

	useEffect(() => {
		GetUpdateStatus().then(setStatus).catch(() => {})
		const offProgress = EventsOn("update:progress", setProgress)
		const offAvailable = EventsOn("update:available", setStatus)
		return () => {
			offProgress()
			offAvailable()
		}
	}, [])

	async function check() {
		setChecking(true)
		setError("")
		try {
			setStatus(await CheckForUpdates())
		} catch (e) {
			setError(String(e))
		} finally {
			setChecking(false)
		}
	}

	async function install() {
		setInstalling(true)
		setError("")
		setProgress(null)
		try {
			setInstalled(await InstallUpdate())
		} catch (e) {
			setError(String(e))
		} finally {
			setInstalling(false)
		}
	}

	async function skip() {
		if (!status) return
		try {
			setStatus(await SkipUpdate(status.latest ?? ""))
		} catch (e) {
			toast.error("Couldn't save that", { description: String(e) })
		}
	}

	async function setAuto(on: boolean) {
		try {
			setStatus(await SetAutoUpdateCheck(on))
		} catch (e) {
			toast.error("Couldn't save that", { description: String(e) })
		}
	}

	const pct = progress && progress.total > 0 ? Math.min(100, Math.round((progress.done / progress.total) * 100)) : null

	return (
		<div className="space-y-6">
			<SectionHeading title="About" hint="Which version of Shelf this is, and whether there is a newer one." />

			<Panel>
				<div className="flex items-center justify-between gap-4">
					<div>
						<p className="text-sm font-medium">Shelf {status?.supported ? status.current : "(development build)"}</p>
						<p className="mt-1 text-xs text-white/45">
							{!status
								? ""
								: !status.supported
									? "Development builds don't check for updates."
									: installed
										? `Updated to ${installed}. Restart to use it.`
										: status.available && !status.skipped
											? `${status.latest} is available.`
											: status.available
												? `${status.latest} is available, and you chose to skip it.`
												: status.checkedAt
													? `You're up to date. Last checked ${formatLastPlayed(status.checkedAt)}.`
													: "Not checked yet."}
						</p>
					</div>
					{status?.supported && !installed && (
						<PillButton onClick={check} disabled={checking || installing}>
							{checking ? "Checking…" : "Check now"}
						</PillButton>
					)}
				</div>

				{status?.supported && status.available && !installed && (
					<div className="space-y-4">
						{status.notes && (
							<pre className="max-h-48 overflow-y-auto rounded-xl bg-black/30 p-3 font-sans text-xs leading-relaxed whitespace-pre-wrap text-white/60">
								{status.notes}
							</pre>
						)}

						{installing && (
							<div className="space-y-2">
								<Bar accent value={pct ?? 8} />
								<p className="text-xs text-white/45">{pct === null ? "Starting…" : `Downloading ${pct}%`}</p>
							</div>
						)}

						<div className="flex flex-wrap items-center gap-2">
							{status.canInstall ? (
								<PillButton variant="solid" onClick={install} disabled={installing}>
									{installing ? "Updating…" : "Update now"}
								</PillButton>
							) : (
								<PillButton variant="solid" onClick={() => status.url && BrowserOpenURL(status.url)}>
									Open release page
								</PillButton>
							)}
							{status.canInstall && status.url && (
								<PillButton onClick={() => BrowserOpenURL(status.url!)}>Release page</PillButton>
							)}
							{!status.skipped && (
								<PillButton onClick={skip} disabled={installing}>
									Skip this version
								</PillButton>
							)}
						</div>
						{!status.canInstall && status.installNote && (
							<p className="text-xs leading-relaxed text-white/45">{status.installNote}</p>
						)}
					</div>
				)}

				{installed && (
					<PillButton variant="solid" onClick={() => RestartApp().catch((e) => setError(String(e)))}>
						Restart now
					</PillButton>
				)}

				{error && <p className="text-sm text-red-300/80">{error}</p>}
			</Panel>

			{status?.supported && (
				<Panel>
					<ToggleRow
						label="Check for updates automatically"
						hint="Shelf asks GitHub when it starts and once a day. Nothing else is sent."
						checked={status.autoCheck}
						onChange={setAuto}
					/>
				</Panel>
			)}
		</div>
	)
}
