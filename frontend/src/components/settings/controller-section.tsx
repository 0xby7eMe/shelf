import { Gamepad2 } from "lucide-react"

import { padLabel, usePadName } from "@/lib/gamepad"
import { setPrefs, usePrefs } from "@/lib/prefs"
import { sfx } from "@/lib/sfx"
import { ToggleRow } from "@/components/toggle-row"
import { Label, Panel, PillButton, SectionHeading } from "@/components/settings/ui"

export const PAD_HINTS: [string, string][] = [
	["D-pad / Left stick", "Move"],
	["A", "Select"],
	["B", "Back"],
	["X", "Favorite"],
	["Y", "Random game"],
	["LB / RB", "Switch tab"],
	["Start", "Settings"],
	["Select", "Search"],
	["Right stick", "Scroll"],
]

export function ControllerSection() {
	const prefs = usePrefs()
	const pad = usePadName()

	function test() {
		// A few steps along the scale, then select and back.
		sfx.move("right")
		setTimeout(() => sfx.move("right"), 140)
		setTimeout(() => sfx.move("right"), 280)
		setTimeout(sfx.confirm, 560)
		setTimeout(sfx.back, 960)
	}

	return (
		<div className="space-y-6">
			<SectionHeading title="Controller" hint="Browse and launch games with a gamepad." />

			<Panel className="flex items-center gap-4 space-y-0">
				<span className="grid size-10 shrink-0 place-items-center rounded-full bg-white/5 ring-1 ring-white/10">
					<Gamepad2 className="size-4 text-white/70" />
				</span>
				<div className="min-w-0">
					<p className="truncate text-sm font-medium">{pad ? padLabel(pad) : "No controller found"}</p>
					<p className="text-xs text-white/45">
						{pad ? "Connected" : "Connect one and press any button. The window needs to be focused."}
					</p>
				</div>
			</Panel>

			<Panel>
				<ToggleRow
					label="Gamepad navigation"
					hint="Move around and select with a controller"
					checked={prefs.gamepad}
					onChange={(v) => setPrefs({ gamepad: v })}
				/>
				<ToggleRow
					label="Interface sounds"
					hint="Soft clicks while you navigate"
					checked={prefs.sounds}
					onChange={(v) => setPrefs({ sounds: v })}
				/>
				<div className="space-y-2">
					<div className="flex items-center justify-between">
						<Label>Volume</Label>
						<span className="text-xs text-white/45 tabular-nums">{prefs.volume}%</span>
					</div>
					<input
						type="range"
						min={0}
						max={100}
						step={5}
						value={prefs.volume}
						disabled={!prefs.sounds}
						onChange={(e) => setPrefs({ volume: Number(e.target.value) })}
						onPointerUp={() => sfx.move()}
						aria-label="Volume"
						className="h-1.5 w-full cursor-pointer appearance-none rounded-full bg-white/10 accent-white disabled:opacity-40"
					/>
				</div>
				<PillButton onClick={test} disabled={!prefs.sounds}>
					Play test sounds
				</PillButton>
			</Panel>

			<Panel>
				<Label>Buttons</Label>
				<dl className="grid grid-cols-2 gap-x-6 gap-y-2.5 text-sm">
					{PAD_HINTS.map(([button, action]) => (
						<div key={button} className="flex items-center justify-between gap-3">
							<dt className="text-white/55">{action}</dt>
							<dd className="rounded-md bg-white/5 px-2 py-0.5 text-xs text-white/80 ring-1 ring-white/10">
								{button}
							</dd>
						</div>
					))}
				</dl>
			</Panel>
		</div>
	)
}
