import { useEffect, useState } from "react"
import { ExternalLink, LogOut, RefreshCw } from "lucide-react"

import {
	EpicLogin,
	EpicLogout,
	EpicOpenLogin,
	EpicSync,
	GetEpicSettings,
	GetProtonBuilds,
	SetEpicSettings,
} from "../../wailsjs/go/main/App"
import { epic } from "../../wailsjs/go/models"
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select"
import { cn } from "@/lib/utils"

interface Props {
	open: boolean
	account: epic.Account | null
	onOpenChange: (open: boolean) => void
	onAccountChange: () => void
}

// Select items can't have an empty value, so "automatic" gets a sentinel.
const AUTO = "auto"

const pill =
	"inline-flex h-10 items-center justify-center gap-2 rounded-full px-5 text-xs ring-1 ring-white/10 transition disabled:pointer-events-none disabled:opacity-40"

export function EpicDialog({ open, account, onOpenChange, onAccountChange }: Props) {
	const [code, setCode] = useState("")
	const [busy, setBusy] = useState(false)
	const [error, setError] = useState("")
	const [settings, setSettings] = useState<epic.Settings | null>(null)
	const [builds, setBuilds] = useState<epic.ProtonBuild[]>([])

	useEffect(() => {
		if (!open) return
		setError("")
		GetEpicSettings().then(setSettings).catch(() => {})
		GetProtonBuilds().then((b) => setBuilds(b ?? [])).catch(() => {})
	}, [open])

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

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent
				style={{
					backdropFilter: "blur(30px)",
					WebkitBackdropFilter: "blur(30px)",
					backgroundColor: "rgba(10, 10, 10, 0.6)",
				}}
				className="gap-6 border-white/10 p-6 sm:max-w-lg"
			>
				<div>
					<DialogTitle className="text-[11px] font-medium tracking-[0.25em] text-muted-foreground uppercase">
						Epic Games
					</DialogTitle>
					<DialogDescription className="sr-only">
						Connect your Epic Games account and choose how games run
					</DialogDescription>
				</div>

				{account && !account.legendaryFound && (
					<p className="rounded-xl bg-white/5 p-4 text-sm text-white/70 ring-1 ring-white/[0.06]">
						Shelf uses <span className="text-white">legendary</span> to talk to Epic. Install it
						first (Arch: <code className="text-white/90">pacman -S legendary</code>), then reopen
						this dialog.
					</p>
				)}

				{account?.legendaryFound && !account.loggedIn && (
					<div className="space-y-3">
						<p className="text-sm text-white/60">
							Sign in on Epic's site, then paste the code it shows you below.
						</p>
						<button
							onClick={() => run(EpicOpenLogin)}
							className={cn(pill, "bg-white/5 text-white/80 hover:bg-white/10 hover:text-white")}
						>
							<ExternalLink className="size-3.5" />
							Open Epic login
						</button>
						<Input
							value={code}
							onChange={(e) => setCode(e.target.value)}
							placeholder="Authorization code"
							spellCheck={false}
							className="h-10 rounded-full border-0 bg-white/5 px-4 text-sm shadow-none ring-1 ring-white/5"
						/>
						<button
							disabled={busy || !code.trim()}
							onClick={() => run(() => EpicLogin(code).then(() => setCode("")))}
							className={cn(pill, "w-full bg-white text-black hover:shadow-[0_0_40px_rgba(255,255,255,0.3)]")}
						>
							{busy ? "Connecting…" : "Connect"}
						</button>
					</div>
				)}

				{account?.loggedIn && (
					<>
						<div className="flex items-center justify-between gap-3 rounded-xl bg-white/5 px-4 py-3 ring-1 ring-white/[0.06]">
							<div className="min-w-0">
								<p className="text-[11px] tracking-wide text-white/45 uppercase">Signed in</p>
								<p className="truncate text-sm font-medium">{account.name || "Epic account"}</p>
							</div>
							<div className="flex gap-2">
								<button
									disabled={busy}
									onClick={() => run(EpicSync)}
									title="Refresh library from Epic"
									aria-label="Refresh library from Epic"
									className="grid size-9 place-items-center rounded-full bg-white/5 text-white/70 ring-1 ring-white/10 transition hover:bg-white/10 hover:text-white disabled:opacity-40"
								>
									<RefreshCw className={cn("size-3.5", busy && "animate-spin")} />
								</button>
								<button
									disabled={busy}
									onClick={() => run(EpicLogout)}
									title="Sign out"
									aria-label="Sign out"
									className="grid size-9 place-items-center rounded-full bg-white/5 text-white/70 ring-1 ring-white/10 transition hover:bg-white/10 hover:text-white disabled:opacity-40"
								>
									<LogOut className="size-3.5" />
								</button>
							</div>
						</div>

						{settings && (
							<div className="space-y-4">
								<label className="block space-y-2">
									<span className="text-[11px] tracking-wide text-white/45 uppercase">
										Install folder
									</span>
									<Input
										defaultValue={settings.installDir}
										onBlur={(e) => {
											const v = e.target.value.trim()
											if (v && v !== settings.installDir)
												save(new epic.Settings({ ...settings, installDir: v }))
										}}
										spellCheck={false}
										className="h-10 rounded-full border-0 bg-white/5 px-4 text-sm shadow-none ring-1 ring-white/5"
									/>
								</label>

								<label className="block space-y-2">
									<span className="text-[11px] tracking-wide text-white/45 uppercase">
										Proton version
									</span>
									<Select
										value={settings.protonPath || AUTO}
										onValueChange={(v) =>
											save(new epic.Settings({ ...settings, protonPath: v === AUTO ? "" : v }))
										}
									>
										<SelectTrigger className="h-10 w-full rounded-full border-0 bg-white/5 px-4 text-sm shadow-none ring-1 ring-white/5">
											<SelectValue />
										</SelectTrigger>
										<SelectContent className="border-white/10 bg-popover/80 backdrop-blur-xl">
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
										<span className="block text-xs text-white/40">
											Install Proton through Steam or ProtonUp-Qt to run Epic games.
										</span>
									)}
								</label>
							</div>
						)}
					</>
				)}

				{error && <p className="text-sm break-words text-destructive">{error}</p>}
			</DialogContent>
		</Dialog>
	)
}
