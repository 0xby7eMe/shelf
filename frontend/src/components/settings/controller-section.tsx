import { Gamepad2 } from "lucide-react"

import { PadGlyph, type PadAction } from "@/components/pad-glyph"
import { padKind, padLabel, usePadName, type PadKind } from "@/lib/gamepad"
import { setPrefs, usePrefs } from "@/lib/prefs"
import { sfx, useSoundSource } from "@/lib/sfx"
import { ToggleRow } from "@/components/toggle-row"
import { Label, Panel, PillButton, SectionHeading } from "@/components/settings/ui"

// What each button does. Movement has no single button, so it is text.
const PAD_ACTIONS: [PadAction | null, string, string][] = [
	[null, "D-pad / Left stick", "Move"],
	["confirm", "", "Select"],
	["back", "", "Back"],
	["favorite", "", "Favorite"],
	["random", "", "Random game"],
	["tabs", "", "Switch tab"],
	["menu", "", "Settings"],
	["search", "", "Search"],
	[null, "Right stick", "Scroll"],
]

const KIND_NAME: Record<PadKind, string> = { xbox: "Xbox", playstation: "PlayStation", nintendo: "Nintendo" }

export function ControllerSection() {
	const prefs = usePrefs()
	const pad = usePadName()
	const soundSource = useSoundSource()

	function test() {
		// A few steps, then select and back.
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
						{pad
							? `Connected · shown with ${KIND_NAME[padKind(pad)]} buttons`
							: "Connect one and press any button. The window needs to be focused."}
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
					hint={
						soundSource === "steam"
							? "Steam's own Deck sounds, played from your Steam install"
							: soundSource === "loading"
								? "Looking for Steam's sounds…"
								: "Built-in sounds. Steam's Deck sounds weren't found."
					}
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
					{PAD_ACTIONS.map(([action, text, label]) => (
						<div key={label} className="flex items-center justify-between gap-3">
							<dt className="text-white/55">{label}</dt>
							<dd className="flex min-h-5 items-center">
								{action ? (
									<PadGlyph action={action} />
								) : (
									<span className="rounded-md bg-white/5 px-2 py-0.5 text-xs text-white/80 ring-1 ring-white/10">
										{text}
									</span>
								)}
							</dd>
						</div>
					))}
				</dl>
			</Panel>
		</div>
	)
}
