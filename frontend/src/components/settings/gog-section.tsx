import { useEffect, useState } from "react"
import { ExternalLink, LogOut, RefreshCw } from "lucide-react"

import {
	GetEpicSettings,
	GetProtonBuilds,
	GogLogin,
	GogLogout,
	GogOpenLogin,
	GogSync,
	SetEpicSettings,
} from "../../../wailsjs/go/main/App"
import { epic } from "../../../wailsjs/go/models"
import { Input } from "@/components/ui/input"
import { IconButton, inputClass, Label, Panel, PillButton, SectionHeading } from "@/components/settings/ui"
import { InstallSettings } from "@/components/settings/install-settings"
import { toast } from "@/lib/toast"
import { cn } from "@/lib/utils"

interface Props {
	account: epic.GogAccount | null
	onAccountChange: () => void
}

export function GogSection({ account, onAccountChange }: Props) {
	const [code, setCode] = useState("")
	const [busy, setBusy] = useState(false)
	const [error, setError] = useState("")
	const [settings, setSettings] = useState<epic.Settings | null>(null)
	const [builds, setBuilds] = useState<epic.ProtonBuild[]>([])
	const loggedIn = !!account?.loggedIn

	useEffect(() => {
		GetEpicSettings().then(setSettings).catch(() => {})
		GetProtonBuilds().then((b) => setBuilds(b ?? [])).catch(() => {})
	}, [])

	function save(next: epic.Settings) {
		setSettings(next)
		SetEpicSettings(next).catch((e) => setError(String(e)))
	}

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

	return (
		<div className="space-y-6">
			<SectionHeading title="GOG" hint="Your GOG library, installed from GOG's own installers and run with Proton." />

			{account && !loggedIn && (
				<Panel>
					<div>
						<p className="text-sm font-medium">Connect your account</p>
						<p className="mt-1 text-sm text-white/55">
							Sign in on GOG's site. It then shows an empty page: copy that page's whole address and paste it here.
						</p>
					</div>
					<PillButton onClick={() => run(GogOpenLogin)}>
						<ExternalLink className="size-3.5" />
						Open GOG login
					</PillButton>
					<Input
						value={code}
						onChange={(e) => setCode(e.target.value)}
						placeholder="https://embed.gog.com/on_login_success?…&code=…"
						spellCheck={false}
						className={inputClass}
					/>
					<PillButton
						variant="solid"
						className="w-full"
						disabled={busy || !code.trim()}
						onClick={() =>
							run(async () => {
								await GogLogin(code)
								setCode("")
								toast.success("Connected to GOG", { description: "Your games are loading into the library." })
							})
						}
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
							<p className="truncate text-sm font-medium">{account?.name || "GOG account"}</p>
						</div>
						<div className="flex gap-2">
							<IconButton label="Refresh library from GOG" disabled={busy} onClick={() => run(GogSync)}>
								<RefreshCw className={cn("size-3.5", busy && "animate-spin")} />
							</IconButton>
							<IconButton label="Sign out" disabled={busy} onClick={() => run(GogLogout)}>
								<LogOut className="size-3.5" />
							</IconButton>
						</div>
					</Panel>

					{settings && <InstallSettings settings={settings} builds={builds} onChange={save} />}
					<p className="px-1 text-xs text-white/40">
						Shared with Epic games. Each game gets a Proton prefix of its own.
					</p>
				</>
			)}

			{error && <p className="text-sm break-words text-destructive">{error}</p>}
		</div>
	)
}
