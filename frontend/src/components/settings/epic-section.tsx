import { useCallback, useEffect, useState } from "react"
import { Download, ExternalLink, LogOut, RefreshCw } from "lucide-react"

import {
	EpicCheckUpdates,
	EpicFindImportable,
	EpicImport,
	EpicImportAll,
	EpicLogin,
	EpicLogout,
	EpicOpenLogin,
	EpicSync,
	GetEpicSettings,
	GetLegendaryInstall,
	GetProtonBuilds,
	InstallLegendary,
	SetEpicSettings,
} from "../../../wailsjs/go/main/App"
import { epic } from "../../../wailsjs/go/models"
import { EventsOn } from "../../../wailsjs/runtime/runtime"
import { Input } from "@/components/ui/input"
import { BattlEyePanel } from "@/components/settings/battleye-panel"
import { InstallSettings } from "@/components/settings/install-settings"
import { ToggleRow } from "@/components/toggle-row"
import {
	IconButton,
	inputClass,
	Label,
	Panel,
	PillButton,
	SectionHeading,
} from "@/components/settings/ui"
import { usePlatform } from "@/lib/platform"
import { toast } from "@/lib/toast"
import { cn } from "@/lib/utils"

interface Props {
	account: epic.Account | null
	onAccountChange: () => void
}

export function EpicSection({ account, onAccountChange }: Props) {
	const [code, setCode] = useState("")
	const [busy, setBusy] = useState(false)
	const [error, setError] = useState("")
	const [settings, setSettings] = useState<epic.Settings | null>(null)
	const [builds, setBuilds] = useState<epic.ProtonBuild[]>([])
	const [importable, setImportable] = useState<epic.Importable[] | null>(null)

	const loggedIn = !!account?.loggedIn
	const platform = usePlatform()

	useEffect(() => {
		GetEpicSettings().then(setSettings).catch(() => {})
		GetProtonBuilds().then((b) => setBuilds(b ?? [])).catch(() => {})
	}, [])

	const scan = useCallback(
		() =>
			EpicFindImportable()
				.then((list) => setImportable(list ?? []))
				.catch((e) => setError(String(e))),
		[]
	)
	useEffect(() => {
		if (loggedIn) scan()
	}, [loggedIn, scan])

	async function run(task: () => Promise<unknown>) {
		setBusy(true)
		setError("")
		try {
			await task()
			onAccountChange()
		} catch (e) {
			setError(String(e))
		} finally {
			setBusy(false)
		}
	}

	function save(next: epic.Settings) {
		setSettings(next)
		SetEpicSettings(next).catch((e) => setError(String(e)))
	}

	async function checkUpdates() {
		setBusy(true)
		try {
			const found = (await EpicCheckUpdates()) ?? []
			if (found.length === 0) toast.success("All your Epic games are up to date")
			else
				toast.info(found.length === 1 ? "1 game update available" : `${found.length} game updates available`, {
					description: found.map((u) => u.title).join(", "),
				})
		} catch (e) {
			setError(String(e))
		} finally {
			setBusy(false)
		}
	}

	async function importOne(g: epic.Importable) {
		try {
			await EpicImport(g.appName, g.path)
			setImportable((list) => (list ?? []).filter((x) => x.appName !== g.appName))
		} catch (e) {
			setError(String(e))
		}
	}

	async function importAll() {
		try {
			await EpicImportAll()
			setImportable([])
		} catch (e) {
			setError(String(e))
			scan()
		}
	}

	return (
		<div className="space-y-6">
			<SectionHeading
				title="Epic Games"
				hint={
					platform.proton
						? "Your Epic library, installed through legendary and run with Proton."
						: "Your Epic library, installed and started through legendary."
				}
			/>

			{account && !account.legendaryFound && <LegendarySetup onInstalled={onAccountChange} />}

			{account?.legendaryFound && !loggedIn && (
				<Panel>
					<div>
						<p className="text-sm font-medium">Connect your account</p>
						<p className="mt-1 text-sm text-white/55">
							Sign in on Epic's site, then paste the code it shows you.
						</p>
					</div>
					<PillButton onClick={() => run(EpicOpenLogin)}>
						<ExternalLink className="size-3.5" />
						Open Epic login
					</PillButton>
					<Input
						value={code}
						onChange={(e) => setCode(e.target.value)}
						placeholder="Authorization code"
						spellCheck={false}
						className={inputClass}
					/>
					<PillButton
						variant="solid"
						className="w-full"
						disabled={busy || !code.trim()}
						onClick={() => run(() => EpicLogin(code).then(() => setCode("")))}
					>
						{busy ? "Connecting…" : "Connect"}
					</PillButton>
				</Panel>
			)}

			{loggedIn && (
				<>
					<Panel className="flex items-center justify-between gap-3 space-y-0">
						<div className="min-w-0">
							<Label>Signed in</Label>
							<p className="truncate text-sm font-medium">{account?.name || "Epic account"}</p>
						</div>
						<div className="flex gap-2">
							<IconButton label="Refresh library from Epic" disabled={busy} onClick={() => run(EpicSync)}>
								<RefreshCw className={cn("size-3.5", busy && "animate-spin")} />
							</IconButton>
							<IconButton label="Sign out" disabled={busy} onClick={() => run(EpicLogout)}>
								<LogOut className="size-3.5" />
							</IconButton>
						</div>
					</Panel>

					{settings && (
						<>
							<InstallSettings settings={settings} builds={builds} onChange={save} />

							<Panel>
								<ToggleRow
									label="Check for updates automatically"
									hint="At startup and every few hours"
									checked={settings.autoCheckUpdates}
									onChange={(v) =>
										save(new epic.Settings({ ...settings, autoCheckUpdates: v, autoUpdate: v && settings.autoUpdate }))
									}
								/>
								<ToggleRow
									label="Install updates automatically"
									hint="Games you're playing are skipped. Individual games can opt out."
									checked={settings.autoUpdate}
									disabled={!settings.autoCheckUpdates}
									onChange={(v) => save(new epic.Settings({ ...settings, autoUpdate: v }))}
								/>
								{platform.cloudSaves && (
									<ToggleRow
										label="Sync cloud saves"
										hint="Download newer saves before a game starts and upload after it closes"
										checked={settings.cloudSaves}
										onChange={(v) => save(new epic.Settings({ ...settings, cloudSaves: v }))}
									/>
								)}
								<PillButton disabled={busy} onClick={checkUpdates}>
									<RefreshCw className={cn("size-3.5", busy && "animate-spin")} />
									Check for updates now
								</PillButton>
							</Panel>
						</>
					)}

					{platform.proton && <BattlEyePanel />}

					<Panel>
						<div className="flex items-center justify-between">
							<Label>Already installed</Label>
							<button onClick={scan} className="text-xs text-white/50 transition hover:text-white">
								Scan again
							</button>
						</div>
						{importable === null ? (
							<p className="text-xs text-white/40">Looking for installed games…</p>
						) : importable.length === 0 ? (
							<p className="text-xs text-white/40">
								No other installs found. Shelf checks Heroic, legendary and the Epic launcher's folders.
							</p>
						) : (
							<>
								<ul className="space-y-2">
									{importable.map((g) => (
										<li
											key={g.appName}
											className="flex items-center justify-between gap-3 rounded-xl bg-white/5 px-4 py-3 ring-1 ring-white/[0.06]"
										>
											<div className="min-w-0">
												<p className="truncate text-sm">{g.title}</p>
												<p className="truncate text-[11px] text-white/35">
													{g.source} · {g.path}
												</p>
											</div>
											<PillButton className="h-8 px-3.5" onClick={() => importOne(g)}>
												Import
											</PillButton>
										</li>
									))}
								</ul>
								{importable.length > 1 && (
									<PillButton variant="solid" className="w-full" onClick={importAll}>
										Import all {importable.length}
									</PillButton>
								)}
							</>
						)}
					</Panel>
				</>
			)}

			{error && <p className="text-sm break-words text-destructive">{error}</p>}
		</div>
	)
}

// legendary isn't installed: Shelf can fetch its standalone build, which needs
// no Python, from legendary's own GitHub releases.
function LegendarySetup({ onInstalled }: { onInstalled: () => void }) {
	const [state, setState] = useState<epic.ProtonInstallState | null>(null)
	const running = state?.state === "running"

	useEffect(() => {
		GetLegendaryInstall().then(setState).catch(() => {})
		return EventsOn("legendary:install", (s: epic.ProtonInstallState) => {
			setState(s)
			if (s.state === "done") onInstalled()
		})
	}, [onInstalled])

	return (
		<Panel>
			<div>
				<p className="text-sm font-medium">Install legendary</p>
				<p className="mt-1 text-sm text-white/55">
					Shelf uses <span className="text-white">legendary</span> to talk to Epic. Shelf can download its standalone
					build from legendary's GitHub releases. It runs without Python, and its checksum is checked before use.
				</p>
			</div>
			{running ? (
				<div className="space-y-2">
					<div className="h-1.5 overflow-hidden rounded-full bg-white/10">
						<div className="h-full rounded-full bg-white/70 transition-[width]" style={{ width: `${state.percent}%` }} />
					</div>
					<p className="text-xs text-white/50">{state.message}</p>
				</div>
			) : (
				<PillButton
					variant="solid"
					className="w-full"
					onClick={() =>
						InstallLegendary().catch((e) => setState(new epic.ProtonInstallState({ state: "failed", error: String(e) })))
					}
				>
					<Download className="size-3.5" />
					Install legendary
				</PillButton>
			)}
			{state?.state === "failed" && <p className="text-sm break-words text-destructive">{state.error}</p>}
			<p className="text-xs text-white/40">
				Or install it yourself: download the build for your system from{" "}
				<span className="text-white/60">github.com/legendary-gl/legendary/releases</span>, make it executable with{" "}
				<code className="text-white/60">chmod +x</code> and put it in your PATH.
			</p>
		</Panel>
	)
}
