import { useCallback, useEffect, useState } from "react"

import { GetDesktopStatus, SetDesktopSettings } from "../../../wailsjs/go/main/App"
import { desktop, main } from "../../../wailsjs/go/models"
import { Input } from "@/components/ui/input"
import { ToggleRow } from "@/components/toggle-row"
import { Panel, PillButton, SectionHeading, inputClass } from "@/components/settings/ui"
import { toast } from "@/lib/toast"
import { cn } from "@/lib/utils"

function Dot({ on, label }: { on: boolean; label: string }) {
	return (
		<span className="flex items-center gap-2 text-xs text-white/50">
			<span className={cn("size-1.5 rounded-full", on ? "bg-emerald-400" : "bg-white/25")} />
			{label}
		</span>
	)
}

// How Shelf fits into the rest of the desktop. Everything here is off until switched on.
export function DesktopSection() {
	const [status, setStatus] = useState<main.DesktopStatus | null>(null)
	const [clientId, setClientId] = useState("")

	const load = useCallback(() => GetDesktopStatus().then(setStatus).catch(() => {}), [])
	useEffect(() => {
		GetDesktopStatus()
			.then((s) => {
				setStatus(s)
				setClientId(s.settings.discordClientId)
			})
			.catch(() => {})
		// Whether Discord is reachable changes without us doing anything.
		const t = setInterval(load, 4000)
		return () => clearInterval(t)
	}, [load])

	async function save(patch: Partial<desktop.Settings>) {
		if (!status) return
		try {
			setStatus(await SetDesktopSettings(desktop.Settings.createFrom({ ...status.settings, ...patch })))
		} catch (e) {
			toast.error("Couldn't save", { description: String(e) })
			load()
		}
	}

	if (!status) return null
	const s = status.settings
	const idChanged = clientId.trim() !== s.discordClientId

	return (
		<div className="space-y-6">
			<SectionHeading title="Desktop" hint="Let Shelf show up where you already look: Discord, your application menu, links and the tray." />

			<Panel>
				<ToggleRow
					label="Discord rich presence"
					hint="Show the game you are playing, and for how long, on your Discord profile. Discord has to be running on this PC."
					checked={s.discordEnabled}
					disabled={!s.discordClientId && !idChanged}
					onChange={(v) => save({ discordEnabled: v, discordClientId: clientId.trim() })}
				/>
				<div className="space-y-2">
					<p className="text-[11px] tracking-wide text-white/45 uppercase">Discord application ID</p>
					<div className="flex gap-2">
						<Input
							value={clientId}
							inputMode="numeric"
							onChange={(e) => setClientId(e.target.value.replace(/\D/g, ""))}
							placeholder="1234567890123456789"
							className={inputClass}
						/>
						<PillButton disabled={!idChanged} onClick={() => save({ discordClientId: clientId.trim() })}>
							Save
						</PillButton>
					</div>
					<p className="text-xs leading-relaxed text-white/40">
						Discord shows the name of an application, so make one called "Shelf" at discord.com/developers/applications and paste its
						Application ID here.
					</p>
				</div>
				{s.discordEnabled && <Dot on={status.discordConnected} label={status.discordConnected ? "Connected to Discord" : "Waiting for Discord"} />}
			</Panel>

			<Panel>
				<ToggleRow
					label="Application menu entries"
					hint="Add every installed game to your application menu and launcher, so you can start it without opening Shelf. They are kept up to date as games come and go."
					checked={s.menuEntries}
					onChange={(v) => save({ menuEntries: v })}
				/>
				<ToggleRow
					label="Open shelf:// links"
					hint="Makes shelf://launch/steam:620 start a game from anywhere: a browser, a script or a stream deck. Use the game's id, like steam:620 or epic:Fortnite."
					checked={s.urlHandler}
					onChange={(v) => save({ urlHandler: v })}
				/>
			</Panel>

			<Panel>
				<ToggleRow
					label="Tray icon"
					hint="A tray icon with your recently played games. Needs a desktop that shows tray icons (on GNOME that takes an extension). Applies the next time Shelf starts."
					checked={s.tray}
					onChange={(v) => save({ tray: v })}
				/>
				<ToggleRow
					label="Keep running in the tray when the window is closed"
					hint="Closing the window hides it instead of quitting. Quit from the tray menu. Only switch this on if you can see the tray icon."
					checked={s.closeToTray}
					disabled={!s.tray}
					onChange={(v) => save({ closeToTray: v })}
				/>
				{s.tray && <Dot on={status.trayRunning} label={status.trayRunning ? "Tray icon is showing" : "Tray icon starts with Shelf"} />}
			</Panel>
		</div>
	)
}
