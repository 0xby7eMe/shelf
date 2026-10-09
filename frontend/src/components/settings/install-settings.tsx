import { epic } from "../../../wailsjs/go/models"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { inputClass, Label, Panel, selectContentClass, triggerClass } from "@/components/settings/ui"

const AUTO = "auto" // Select items can't have an empty value

// Where Epic and GOG games are installed, and the Proton build they run with.
// Both stores share these, so either tab can change them.
export function InstallSettings({
	settings,
	builds,
	onChange,
}: {
	settings: epic.Settings
	builds: epic.ProtonBuild[]
	onChange: (next: epic.Settings) => void
}) {
	return (
		<Panel>
			<label className="block space-y-2">
				<Label>Install folder</Label>
				<Input
					defaultValue={settings.installDir}
					onBlur={(e) => {
						const v = e.target.value.trim()
						if (v && v !== settings.installDir) onChange(new epic.Settings({ ...settings, installDir: v }))
					}}
					spellCheck={false}
					className={inputClass}
				/>
			</label>

			<div className="space-y-2">
				<Label>Proton version</Label>
				<Select
					value={settings.protonPath || AUTO}
					onValueChange={(v) => onChange(new epic.Settings({ ...settings, protonPath: !v || v === AUTO ? "" : v }))}
				>
					<SelectTrigger className={triggerClass}>
						<SelectValue />
					</SelectTrigger>
					<SelectContent className={selectContentClass}>
						<SelectItem value={AUTO}>{builds.length ? `Automatic (${builds[0].name})` : "None found"}</SelectItem>
						{builds.map((b) => (
							<SelectItem key={b.path} value={b.path}>
								{b.name}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
				{builds.length === 0 && (
					<p className="text-xs text-white/40">Install Proton through Steam or ProtonUp-Qt to run Epic and GOG games.</p>
				)}
			</div>
		</Panel>
	)
}
