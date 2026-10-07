import { useCallback, useEffect, useState } from "react"
import { ExternalLink, LogOut, RefreshCw } from "lucide-react"

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
	GetProtonBuilds,
	SetEpicSettings,
} from "../../../wailsjs/go/main/App"
import { epic } from "../../../wailsjs/go/models"
import { Input } from "@/components/ui/input"
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select"
import { ToggleRow } from "@/components/toggle-row"
import {
	IconButton,
	inputClass,
	Label,
	Panel,
	PillButton,
	SectionHeading,
	selectContentClass,
	triggerClass,
} from "@/components/settings/ui"
import { toast } from "@/lib/toast"
import { cn } from "@/lib/utils"

interface Props {
	account: epic.Account | null
	onAccountChange: () => void
}

const AUTO = "auto" // Select items can't have an empty value

export function EpicSection({ account, onAccountChange }: Props) {
	const [code, setCode] = useState("")
	const [busy, setBusy] = useState(false)
	const [error, setError] = useState("")
	const [settings, setSettings] = useState<epic.Settings | null>(null)
	const [builds, setBuilds] = useState<epic.ProtonBuild[]>([])
	const [importable, setImportable] = useState<epic.Importable[] | null>(null)

	const loggedIn = !!account?.loggedIn

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
			<SectionHeading title="Epic Games" hint="Your Epic library, installed through legendary and run with Proton." />

			{account && !account.legendaryFound && (
				<Panel>
					<p className="text-sm text-white/70">
						Shelf uses <span className="text-white">legendary</span> to talk to Epic. Install it first
						(Arch: <code className="text-white/90">pacman -S legendary</code>), then reopen this page.
					</p>
				</Panel>
			)}

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
							<Panel>
								<label className="block space-y-2">
									<Label>Install folder</Label>
									<Input
										defaultValue={settings.installDir}
										onBlur={(e) => {
											const v = e.target.value.trim()
											if (v && v !== settings.installDir) save(new epic.Settings({ ...settings, installDir: v }))
										}}
										spellCheck={false}
										className={inputClass}
									/>
								</label>

								<div className="space-y-2">
									<Label>Proton version</Label>
									<Select
										value={settings.protonPath || AUTO}
										onValueChange={(v) =>
											save(new epic.Settings({ ...settings, protonPath: !v || v === AUTO ? "" : v }))
										}
									>
										<SelectTrigger className={triggerClass}>
											<SelectValue />
										</SelectTrigger>
										<SelectContent className={selectContentClass}>
											<SelectItem value={AUTO}>
												{builds.length ? `Automatic (${builds[0].name})` : "None found"}
											</SelectItem>
											{builds.map((b) => (
												<SelectItem key={b.path} value={b.path}>
													{b.name}
												</SelectItem>
											))}
										</SelectContent>
									</Select>
									{builds.length === 0 && (
										<p className="text-xs text-white/40">
											Install Proton through Steam or ProtonUp-Qt to run Epic games.
										</p>
									)}
								</div>
							</Panel>

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
								<ToggleRow
									label="Sync cloud saves"
									hint="Download newer saves before a game starts and upload after it closes"
									checked={settings.cloudSaves}
									onChange={(v) => save(new epic.Settings({ ...settings, cloudSaves: v }))}
								/>
								<PillButton disabled={busy} onClick={checkUpdates}>
									<RefreshCw className={cn("size-3.5", busy && "animate-spin")} />
									Check for updates now
								</PillButton>
							</Panel>
						</>
					)}

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
