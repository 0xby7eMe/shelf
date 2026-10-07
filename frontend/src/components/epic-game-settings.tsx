import { useEffect, useState } from "react"

import {
	GetEpicGameSettings,
	GetEpicTools,
	GetProtonBuilds,
	SetEpicGameSettings,
} from "../../wailsjs/go/main/App"
import { epic, library } from "../../wailsjs/go/models"
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
import { ToggleRow } from "@/components/toggle-row"
import { toast } from "@/lib/toast"
import { cn } from "@/lib/utils"

interface Props {
	game: library.Game | null
	onClose: () => void
}

const GLOBAL = "global" // Select items can't have an empty value

const inputClass =
	"h-10 rounded-full border-0 bg-white/5 px-4 text-sm shadow-none ring-1 ring-white/5"
const triggerClass =
	"h-10 w-full rounded-full border-0 bg-white/5 px-4 text-sm shadow-none ring-1 ring-white/5"
const selectContentClass = "border-white/10 bg-popover/80 backdrop-blur-xl"

export function EpicGameSettings({ game, onClose }: Props) {
	return (
		<Dialog open={game !== null} onOpenChange={(open) => !open && onClose()}>
			<DialogContent
				style={{
					backdropFilter: "blur(30px)",
					WebkitBackdropFilter: "blur(30px)",
					backgroundColor: "rgba(10, 10, 10, 0.6)",
				}}
				className="max-h-[88vh] gap-5 overflow-y-auto border-white/10 p-6 sm:max-w-lg"
			>
				{game && <Form key={game.id} game={game} onClose={onClose} />}
			</DialogContent>
		</Dialog>
	)
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
	return (
		<label className="block space-y-2">
			<span className="text-[11px] tracking-wide text-white/45 uppercase">{label}</span>
			{children}
			{hint && <span className="block text-xs text-white/40">{hint}</span>}
		</label>
	)
}

function ChoiceSelect({
	value,
	onChange,
	on,
	off,
}: {
	value: string
	onChange: (v: string) => void
	on: string
	off: string
}) {
	return (
		<Select value={value} onValueChange={(v) => v && onChange(v)}>
			<SelectTrigger className={triggerClass}>
				<SelectValue />
			</SelectTrigger>
			<SelectContent className={selectContentClass}>
				<SelectItem value="default">Follow global setting</SelectItem>
				<SelectItem value="on">{on}</SelectItem>
				<SelectItem value="off">{off}</SelectItem>
			</SelectContent>
		</Select>
	)
}

function Form({ game, onClose }: { game: library.Game; onClose: () => void }) {
	const [s, setS] = useState<epic.GameSettings | null>(null)
	const [builds, setBuilds] = useState<epic.ProtonBuild[]>([])
	const [tools, setTools] = useState<epic.Tools | null>(null)
	const [error, setError] = useState("")
	const [saving, setSaving] = useState(false)

	useEffect(() => {
		GetEpicGameSettings(game.externalId).then(setS).catch((e) => setError(String(e)))
		GetProtonBuilds().then((b) => setBuilds(b ?? [])).catch(() => {})
		GetEpicTools().then(setTools).catch(() => {})
	}, [game.externalId])

	if (!s) {
		return (
			<>
				<DialogTitle className="sr-only">{game.name} settings</DialogTitle>
				<DialogDescription className="sr-only">Loading settings</DialogDescription>
				{error && <p className="text-sm text-destructive">{error}</p>}
			</>
		)
	}

	const patch = (p: Partial<epic.GameSettings>) => setS(new epic.GameSettings({ ...s, ...p }))

	async function save() {
		if (!s) return
		setSaving(true)
		try {
			await SetEpicGameSettings(game.externalId, s)
			toast.success("Settings saved", { description: game.name })
			onClose()
		} catch (e) {
			setError(String(e))
		} finally {
			setSaving(false)
		}
	}

	return (
		<>
			<div>
				<DialogTitle className="text-[11px] font-medium tracking-[0.25em] text-muted-foreground uppercase">
					{game.name}
				</DialogTitle>
				<DialogDescription className="sr-only">Launch settings for {game.name}</DialogDescription>
			</div>

			<Field label="Proton version">
				<Select
					value={s.protonPath || GLOBAL}
					onValueChange={(v) => patch({ protonPath: !v || v === GLOBAL ? "" : v })}
				>
					<SelectTrigger className={triggerClass}>
						<SelectValue />
					</SelectTrigger>
					<SelectContent className={selectContentClass}>
						<SelectItem value={GLOBAL}>Follow global setting</SelectItem>
						{builds.map((b) => (
							<SelectItem key={b.path} value={b.path}>
								{b.name}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
			</Field>

			<Field label="Launch arguments" hint="Passed to the game, e.g. -windowed -nologo">
				<Input
					value={s.launchArgs}
					onChange={(e) => patch({ launchArgs: e.target.value })}
					spellCheck={false}
					className={inputClass}
				/>
			</Field>

			<Field label="Environment variables" hint="One KEY=value per line">
				<textarea
					value={s.env}
					onChange={(e) => patch({ env: e.target.value })}
					rows={3}
					spellCheck={false}
					placeholder={"DXVK_HUD=fps\nPROTON_ENABLE_NVAPI=1"}
					className="w-full resize-y rounded-2xl border-0 bg-white/5 px-4 py-3 font-mono text-xs ring-1 ring-white/5 outline-none placeholder:text-white/25 focus-visible:ring-white/20"
				/>
			</Field>

			<div className="space-y-4">
				<ToggleRow
					label="MangoHud overlay"
					hint={tools && !tools.mangoHud ? "mangohud isn't installed" : undefined}
					checked={s.mangoHud}
					disabled={tools ? !tools.mangoHud : false}
					onChange={(v) => patch({ mangoHud: v })}
				/>
				<ToggleRow
					label="GameMode"
					hint={tools && !tools.gameMode ? "gamemode isn't installed" : undefined}
					checked={s.gameMode}
					disabled={tools ? !tools.gameMode : false}
					onChange={(v) => patch({ gameMode: v })}
				/>
				<ToggleRow
					label="Offline mode"
					hint="Start without contacting Epic. Online features won't work."
					checked={s.offline}
					onChange={(v) => patch({ offline: v })}
				/>
				<ToggleRow
					label="Allow outdated launches"
					hint="Start the game even when an update is available."
					checked={s.skipUpdateCheck}
					onChange={(v) => patch({ skipUpdateCheck: v })}
				/>
			</div>

			<Field label="Automatic updates">
				<ChoiceSelect
					value={s.autoUpdate}
					onChange={(v) => patch({ autoUpdate: v })}
					on="Always update this game"
					off="Never update automatically"
				/>
			</Field>

			{game.cloudSaves && (
				<>
					<Field label="Cloud saves">
						<ChoiceSelect
							value={s.cloudSaves}
							onChange={(v) => patch({ cloudSaves: v })}
							on="Sync around every launch"
							off="Don't sync this game"
						/>
					</Field>
					<Field
						label="Save folder"
						hint="Leave empty to let Shelf find it inside the game's Proton prefix."
					>
						<Input
							value={s.savePath}
							onChange={(e) => patch({ savePath: e.target.value })}
							spellCheck={false}
							placeholder="Automatic"
							className={inputClass}
						/>
					</Field>
				</>
			)}

			{error && <p className="text-sm break-words text-destructive">{error}</p>}

			<button
				onClick={save}
				disabled={saving}
				className={cn(
					"h-11 w-full rounded-full bg-white text-sm font-medium text-black transition",
					"hover:shadow-[0_0_40px_rgba(255,255,255,0.3)] disabled:opacity-50"
				)}
			>
				{saving ? "Saving…" : "Save"}
			</button>
		</>
	)
}
