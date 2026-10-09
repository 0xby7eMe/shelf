import { FolderOpen, Terminal } from "lucide-react"

import { OpenLogsFolder } from "../../../wailsjs/go/main/App"
import { ToggleRow } from "@/components/toggle-row"
import { Panel, PillButton, SectionHeading } from "@/components/settings/ui"
import { setLogOpen, useLogOpen } from "@/lib/logs"
import { usePlatform } from "@/lib/platform"
import { setPrefs, usePrefs } from "@/lib/prefs"

export function AdvancedSection() {
	const prefs = usePrefs()
	const logOpen = useLogOpen()
	const platform = usePlatform()

	return (
		<div className="space-y-6">
			<SectionHeading title="Advanced" hint="Tools for finding out why something doesn't work." />

			<Panel>
				<ToggleRow
					label="Log window"
					hint="A console with live output from downloads, game launches and cloud saves. Adds a button to the header."
					checked={prefs.logWindow}
					onChange={(v) => {
						setPrefs({ logWindow: v })
						if (!v) setLogOpen(false)
					}}
				/>
				<div className="flex flex-wrap gap-2">
					<PillButton
						disabled={!prefs.logWindow}
						onClick={() => setLogOpen(!logOpen)}
					>
						<Terminal className="size-3.5" />
						{logOpen ? "Hide log window" : "Show log window"}
					</PillButton>
					<PillButton onClick={() => OpenLogsFolder().catch(() => {})}>
						<FolderOpen className="size-3.5" />
						Open log files
					</PillButton>
				</div>
				<p className="text-xs text-white/40">
					Each game's launch output is also saved as a file in the log folder, whether or not the window is on.
				</p>
			</Panel>

			{platform.hardwareMonitor && (
			<Panel>
				<ToggleRow
					label="Hardware monitor"
					hint="A task-manager-style page with live graphs for CPU, memory, disks, network and graphics cards. Adds a button to the header, and P opens it. Nothing is sampled while the page is closed."
					checked={prefs.hardwareMonitor}
					onChange={(v) => setPrefs({ hardwareMonitor: v })}
				/>
			</Panel>
			)}
		</div>
	)
}
