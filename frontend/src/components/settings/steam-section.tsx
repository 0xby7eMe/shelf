import { useState } from "react"
import { ExternalLink } from "lucide-react"

import { DisconnectSteamAPI, SetSteamAPIKey } from "../../../wailsjs/go/main/App"
import { main } from "../../../wailsjs/go/models"
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime"
import { Input } from "@/components/ui/input"
import { Label, Panel, PillButton, inputClass } from "@/components/settings/ui"
import { confirm } from "@/lib/confirm"
import { formatDuration } from "@/lib/format"
import { toast } from "@/lib/toast"

const KEY_URL = "https://steamcommunity.com/dev/apikey"

interface Props {
	status: main.SteamAPIStatus | null
	onChange: (status: main.SteamAPIStatus) => void
}

export function SteamSection({ status, onChange }: Props) {
	const [key, setKey] = useState("")
	const [busy, setBusy] = useState(false)
	const [error, setError] = useState("")
	const [changing, setChanging] = useState(false)

	const configured = !!status?.configured
	const overview = status?.overview

	async function save(e: React.FormEvent) {
		e.preventDefault()
		if (!key.trim()) return
		setBusy(true)
		setError("")
		try {
			onChange(await SetSteamAPIKey(key))
			setKey("")
			setChanging(false)
			toast.success("Steam connected", { description: "Games you own but haven't installed now show in your library." })
		} catch (e) {
			setError(String(e))
		} finally {
			setBusy(false)
		}
	}

	async function disconnect() {
		const ok = await confirm({
			title: "Disconnect Steam?",
			description: "The key is forgotten. Games that aren't installed leave your library, and the friends list stops until you add a key again.",
			confirmLabel: "Disconnect",
			destructive: true,
		})
		if (!ok) return
		setBusy(true)
		try {
			onChange(await DisconnectSteamAPI())
		} catch (e) {
			setError(String(e))
		} finally {
			setBusy(false)
		}
	}

	return (
		<div className="space-y-6">
			<Panel>
				<div>
					<p className="text-sm font-medium">Steam Web API</p>
					<p className="mt-1 text-sm leading-relaxed text-white/45">
						Shelf reads installed games from Steam's own files. A free Web API key adds the rest: every game
						you own, installed or not, with correct play time and last played, plus your account details and
						your friends list.
					</p>
				</div>

				{overview && (
					<div className="flex items-center gap-4">
						{overview.avatar ? (
							<img src={overview.avatar} alt="" className="size-14 rounded-2xl object-cover ring-1 ring-white/10" />
						) : (
							<div className="size-14 rounded-2xl bg-white/10" />
						)}
						<div className="min-w-0 flex-1">
							<p className="truncate text-base font-semibold">{overview.name}</p>
							<p className="text-xs text-white/45">{overview.level > 0 ? `Level ${overview.level}` : "Connected"}</p>
						</div>
						{overview.profileUrl && (
							<button
								onClick={() => BrowserOpenURL(overview.profileUrl)}
								aria-label="Open your Steam profile"
								className="rounded-full p-2 text-white/40 transition hover:bg-white/5 hover:text-white"
							>
								<ExternalLink className="size-4" />
							</button>
						)}
					</div>
				)}

				{overview && !overview.gamesPrivate && (
					<div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
						<Stat label="Owned" value={overview.owned.toLocaleString()} />
						<Stat label="Played" value={overview.played.toLocaleString()} />
						<Stat label="Total" value={formatDuration(overview.totalMinutes)} />
						<Stat label="Last 2 weeks" value={formatDuration(overview.twoWeekMinutes)} />
					</div>
				)}

				{overview?.gamesPrivate && (
					<p className="text-sm leading-relaxed text-amber-200/80">
						Steam hides your game details, so only installed games show. Set <em>Game details</em> to Public in
						Steam, Settings, Privacy, and press Refresh below.
					</p>
				)}

				{configured && status?.message && <p className="text-sm text-red-300/80">{status.message}</p>}

				{(!configured || changing) && (
					<form onSubmit={save} className="space-y-3">
						<Label>{configured ? "New Web API key" : "Web API key"}</Label>
						<div className="flex flex-wrap gap-2">
							<Input
								type="password"
								autoComplete="off"
								spellCheck={false}
								value={key}
								onChange={(e) => setKey(e.target.value)}
								placeholder="Paste your key"
								className={`${inputClass} min-w-0 flex-1`}
							/>
							<PillButton type="submit" variant="solid" disabled={busy || !key.trim()}>
								{busy ? "Checking…" : "Connect"}
							</PillButton>
						</div>
						{error && <p className="text-sm text-red-300/80">{error}</p>}
						<p className="text-xs leading-relaxed text-white/40">
							Get one at{" "}
							<button type="button" onClick={() => BrowserOpenURL(KEY_URL)} className="underline underline-offset-2 hover:text-white/70">
								steamcommunity.com/dev/apikey
							</button>
							. Any domain name works when it asks, for example <code>localhost</code>. The key stays on this
							computer, in a file only you can read, and is only ever sent to Steam.
						</p>
					</form>
				)}

				{configured && (
					<div className="flex flex-wrap gap-2">
						{!changing && (
							<PillButton onClick={() => setChanging(true)} disabled={busy}>
								Change key
							</PillButton>
						)}
						{changing && (
							<PillButton onClick={() => setChanging(false)} disabled={busy}>
								Cancel
							</PillButton>
						)}
						<PillButton variant="danger" onClick={disconnect} disabled={busy}>
							Disconnect
						</PillButton>
					</div>
				)}
			</Panel>
		</div>
	)
}

function Stat({ label, value }: { label: string; value: string }) {
	return (
		<div className="rounded-xl bg-white/[0.04] px-4 py-3 ring-1 ring-white/[0.06]">
			<p className="text-lg font-semibold tracking-tight">{value}</p>
			<p className="text-[11px] tracking-wide text-white/45 uppercase">{label}</p>
		</div>
	)
}
