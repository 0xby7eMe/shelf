import { useCallback, useEffect, useState } from "react"
import { Check, Circle, Download, ExternalLink, Power, RefreshCw, Trash2 } from "lucide-react"

import {
	GetEpicSettings,
	GetProtonInstall,
	GetUbisoftSetup,
	InstallProtonGE,
	GetUbisoftStatus,
	SetEpicSettings,
	UbisoftCloseConnect,
	UbisoftOpenConnect,
	UbisoftReset,
	UbisoftSetup,
	UbisoftSync,
} from "../../../wailsjs/go/main/App"
import { epic } from "../../../wailsjs/go/models"
import { EventsOn } from "../../../wailsjs/runtime/runtime"
import { ToggleRow } from "@/components/toggle-row"
import { BattlEyePanel } from "@/components/settings/battleye-panel"
import { IconButton, Label, Panel, PillButton, SectionHeading } from "@/components/settings/ui"
import { confirm } from "@/lib/confirm"
import { toast } from "@/lib/toast"
import { cn } from "@/lib/utils"

export function UbisoftSection() {
	const [status, setStatus] = useState<epic.UbisoftStatus | null>(null)
	const [setup, setSetup] = useState<epic.UbisoftSetupState | null>(null)
	const [proton, setProton] = useState<epic.ProtonInstallState | null>(null)
	const [settings, setSettings] = useState<epic.Settings | null>(null)
	const [busy, setBusy] = useState(false)
	const [error, setError] = useState("")

	const ready = !!status?.connectInstalled
	const signedIn = !!status?.signedIn
	const installing = setup?.state === "running"
	const fetchingProton = proton?.state === "running"

	const reload = useCallback(() => {
		GetUbisoftStatus().then(setStatus).catch(() => {})
	}, [])

	useEffect(() => {
		reload()
		GetUbisoftSetup().then(setSetup).catch(() => {})
		GetProtonInstall().then(setProton).catch(() => {})
		GetEpicSettings().then(setSettings).catch(() => {})
		const offLibrary = EventsOn("library:changed", reload)
		const offSetup = EventsOn("ubisoft:setup", (s: epic.UbisoftSetupState) => {
			setSetup(s)
			if (s.state === "done") reload()
		})
		const offProton = EventsOn("proton:install", (p: epic.ProtonInstallState) => {
			setProton(p)
			if (p.state === "done") {
				reload()
				toast.success(`${p.name} is installed`, { description: "Shelf uses it from now on." })
			}
		})
		// Whether Connect is open changes without any event, so look again now and then.
		const t = setInterval(() => GetUbisoftStatus().then(setStatus).catch(() => {}), 3000)
		return () => {
			offLibrary()
			offSetup()
			offProton()
			clearInterval(t)
		}
	}, [reload])

	async function run(task: () => Promise<unknown>) {
		setBusy(true)
		setError("")
		try {
			await task()
			reload()
		} catch (e) {
			setError(String(e))
		} finally {
			setBusy(false)
		}
	}

	function act(task: () => Promise<unknown>, what: string) {
		task().catch((e) => toast.error(`Couldn't ${what}`, { description: String(e) }))
	}

	function setSoftware(on: boolean) {
		if (!settings) return
		const next = new epic.Settings({ ...settings, ubisoftSoftwareRendering: on })
		setSettings(next)
		SetEpicSettings(next).catch((e) => toast.error("Couldn't save", { description: String(e) }))
	}

	async function reset() {
		const ok = await confirm({
			title: "Reset Ubisoft Connect?",
			description:
				"This deletes Ubisoft Connect's Proton prefix: its login and every game installed inside it. Your Ubisoft account and library in Shelf are not affected.",
			confirmLabel: "Reset",
			destructive: true,
		})
		if (ok)
			UbisoftReset()
				.then(() => {
					toast.success("Ubisoft Connect was reset")
					reload()
				})
				.catch((e) => toast.error("Couldn't reset", { description: String(e) }))
	}

	return (
		<div className="space-y-6">
			<SectionHeading
				title="Ubisoft"
				hint="Your Ubisoft library, read from Ubisoft Connect. Games are installed and run through it with Proton."
			/>

			<Panel>
				<Step done={ready} title="Set up Ubisoft Connect">
					{status?.proton
						? `Shelf downloads Ubisoft's installer and runs it quietly with ${status.proton}.`
						: "No Proton found. Install GE-Proton (ProtonUp-Qt) or Proton through Steam."}
				</Step>
				{installing && (
					<div className="space-y-1.5">
						<div className="h-1 overflow-hidden rounded-full bg-white/10">
							<div
								className="h-full rounded-full bg-white/70 transition-[width] duration-300"
								style={{ width: `${setup?.percent ?? 0}%` }}
							/>
						</div>
						<p className="text-xs text-white/45">{setup?.message}…</p>
					</div>
				)}
				{setup?.state === "failed" && <p className="text-sm break-words text-destructive">{setup.error}</p>}
				<div className="flex flex-wrap gap-2">
					<PillButton
						variant={ready ? "soft" : "solid"}
						disabled={installing || !status?.proton}
						onClick={() => act(UbisoftSetup, "set up Ubisoft Connect")}
					>
						<RefreshCw className={cn("size-3.5", installing && "animate-spin")} />
						{ready ? "Reinstall" : "Set up Ubisoft Connect"}
					</PillButton>
					{status && !status.protonIsGE && (
						<PillButton
							variant={status.proton ? "soft" : "solid"}
							disabled={fetchingProton}
							onClick={() => act(InstallProtonGE, "install GE-Proton")}
						>
							<Download className={cn("size-3.5", fetchingProton && "animate-pulse")} />
							{fetchingProton ? "Installing GE-Proton…" : "Install GE-Proton"}
						</PillButton>
					)}
				</div>
				{status && !status.protonIsGE && !fetchingProton && (
					<p className="text-xs text-white/40">
						Ubisoft Connect behaves best with GE-Proton: black or frozen windows and slow game starts are the usual
						symptoms of other builds. It is downloaded from GitHub and checked before it is installed.
					</p>
				)}
				{fetchingProton && (
					<div className="space-y-1.5">
						<div className="h-1 overflow-hidden rounded-full bg-white/10">
							<div
								className="h-full rounded-full bg-white/70 transition-[width] duration-300"
								style={{ width: `${proton?.percent ?? 0}%` }}
							/>
						</div>
						<p className="text-xs text-white/45">{proton?.message}…</p>
					</div>
				)}
				{proton?.state === "failed" && <p className="text-sm break-words text-destructive">{proton.error}</p>}
				{proton?.state === "done" && ready && (
					<p className="text-xs text-white/40">
						If Connect's window misbehaves after switching, press Reset below so it starts with a fresh prefix.
					</p>
				)}
			</Panel>

			<Panel>
				<Step done={signedIn} title="Connect your account">
					{signedIn
						? `Signed in to Ubisoft Connect. ${status?.games ?? 0} games found in your library.`
						: "Ubisoft only lets its own launcher sign in, so you do that in Connect's window. Open it, sign in (two-step verification works as usual), then come back. Your games appear here on their own."}
				</Step>
				<div className="flex flex-wrap gap-2">
					<PillButton
						variant={ready && !signedIn ? "solid" : "soft"}
						disabled={!ready}
						onClick={() => act(UbisoftOpenConnect, "open Ubisoft Connect")}
					>
						<ExternalLink className="size-3.5" />
						Open Ubisoft Connect
					</PillButton>
					{status?.running && (
						<PillButton onClick={() => act(UbisoftCloseConnect, "close Ubisoft Connect")}>
							<Power className="size-3.5" />
							Close it
						</PillButton>
					)}
					<IconButton label="Read the library from Connect again" disabled={busy || !ready} onClick={() => run(UbisoftSync)}>
						<RefreshCw className={cn("size-3.5", busy && "animate-spin")} />
					</IconButton>
					<PillButton variant="danger" disabled={installing || !status} onClick={reset}>
						<Trash2 className="size-3.5" />
						Reset
					</PillButton>
				</div>
			</Panel>

			<BattlEyePanel />

			<Panel>
				<ToggleRow
					label="Fix a black Ubisoft Connect window"
					hint="Draws Connect's own window with software rendering while you sign in and install games. Many setups show it black otherwise. Games never use this: Play restarts Connect normally."
					checked={settings?.ubisoftSoftwareRendering ?? true}
					disabled={!settings}
					onChange={setSoftware}
				/>
				<p className="text-xs text-white/40">
					Takes effect the next time Connect is opened. Close it first if it is already open.
				</p>
			</Panel>

			<Panel>
				<Label>Playing</Label>
				<p className="text-sm text-white/55">
					Press Install on a game in Shelf and Ubisoft Connect downloads it. When it's done, Play starts it
					through Connect. Connect stays open in the background after a game closes; that's normal, and the
					newest GE-Proton works best with it.
				</p>
			</Panel>

			{error && <p className="text-sm break-words text-destructive">{error}</p>}
		</div>
	)
}

function Step({ done, title, children }: { done: boolean; title: string; children: React.ReactNode }) {
	return (
		<div className="flex items-start gap-3">
			<span
				className={cn(
					"mt-0.5 grid size-7 shrink-0 place-items-center rounded-full ring-1",
					done ? "bg-emerald-400/15 text-emerald-300 ring-emerald-400/20" : "bg-white/5 text-white/40 ring-white/10"
				)}
			>
				{done ? <Check className="size-3.5" /> : <Circle className="size-3" />}
			</span>
			<div className="min-w-0">
				<p className="text-sm font-medium">{title}</p>
				<p className="mt-1 text-sm text-white/55">{children}</p>
			</div>
		</div>
	)
}
