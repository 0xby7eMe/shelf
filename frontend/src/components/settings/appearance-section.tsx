import { Check } from "lucide-react"

import { ToggleRow } from "@/components/toggle-row"
import { Bar, Label, Panel, PillButton, SectionHeading, Segmented } from "@/components/settings/ui"
import { ACCENTS } from "@/lib/appearance"
import { resetAppearance, setPrefs, UI_SCALES, usePrefs, type PosterSize } from "@/lib/prefs"
import { cn } from "@/lib/utils"

const POSTER_SIZES: { value: PosterSize; label: string }[] = [
	{ value: "small", label: "Small" },
	{ value: "medium", label: "Medium" },
	{ value: "large", label: "Large" },
]

export function AppearanceSection() {
	const prefs = usePrefs()

	return (
		<div className="space-y-6">
			<SectionHeading title="Appearance" hint="Make Shelf look the way you like. These settings stay on this computer." />

			<Panel>
				<div>
					<Label>Accent color</Label>
					<p className="mt-1 text-xs text-white/40">Buttons, switches and progress bars.</p>
				</div>
				<div className="flex flex-wrap items-center gap-2.5">
					{ACCENTS.map((a) => (
						<Swatch key={a.color} color={a.color} name={a.name} active={prefs.accent === a.color} onClick={() => setPrefs({ accent: a.color })} />
					))}
				</div>
				<div className="flex flex-wrap items-center gap-4 rounded-xl bg-black/20 p-4">
					<PillButton variant="solid">Play</PillButton>
					<Bar accent value={62} className="w-40" />
					<span aria-hidden className="relative h-6 w-10 rounded-full bg-solid">
						<span className="absolute top-0.5 left-0.5 size-5 translate-x-4 rounded-full bg-solid-foreground" />
					</span>
				</div>
			</Panel>

			<Panel>
				<div className="flex flex-wrap items-center justify-between gap-3">
					<div>
						<Label>Poster size</Label>
						<p className="mt-1 text-xs text-white/40">How big the games are in your library.</p>
					</div>
					<Segmented label="Poster size" value={prefs.posterSize} options={POSTER_SIZES} onChange={(posterSize) => setPrefs({ posterSize })} />
				</div>
				<div className="flex flex-wrap items-center justify-between gap-3">
					<div>
						<Label>Interface size</Label>
						<p className="mt-1 text-xs text-white/40">Scales text and spacing, for small screens or big TVs.</p>
					</div>
					<Segmented
						label="Interface size"
						value={prefs.uiScale}
						options={UI_SCALES.map((v) => ({ value: v, label: `${v}%` }))}
						onChange={(uiScale) => setPrefs({ uiScale })}
					/>
				</div>
			</Panel>

			<Panel>
				<div>
					<Label>Home</Label>
					<p className="mt-1 text-xs text-white/40">What the library shows above the full list.</p>
				</div>
				<ToggleRow
					label="Banner"
					hint="The big banner for the game you played last."
					checked={prefs.heroBanner}
					onChange={(heroBanner) => setPrefs({ heroBanner })}
				/>
				<ToggleRow
					label="Continue playing"
					hint="A shelf with your most recently played games."
					checked={prefs.shelfRecent}
					onChange={(shelfRecent) => setPrefs({ shelfRecent })}
				/>
				<ToggleRow
					label="Never played"
					hint="A shelf with installed games you haven't started."
					checked={prefs.shelfUnplayed}
					onChange={(shelfUnplayed) => setPrefs({ shelfUnplayed })}
				/>
			</Panel>

			<Panel>
				<ToggleRow
					label="Reduce motion"
					hint="Turns off animations and slides. It starts on if your system asks for less motion."
					checked={prefs.reduceMotion}
					onChange={(reduceMotion) => setPrefs({ reduceMotion })}
				/>
			</Panel>

			<div>
				<PillButton onClick={resetAppearance}>Reset appearance</PillButton>
			</div>
		</div>
	)
}

function Swatch({ color, name, active, onClick }: { color: string; name: string; active: boolean; onClick: () => void }) {
	return (
		<button
			type="button"
			title={name}
			aria-label={name}
			aria-pressed={active}
			onClick={onClick}
			className={cn(
				"grid size-8 place-items-center rounded-full ring-2 ring-offset-2 ring-offset-[#101010] transition",
				active ? "ring-white" : "ring-white/15 hover:ring-white/40"
			)}
			style={{ background: color }}
		>
			{active && <Check className="size-3.5 text-black/70" />}
		</button>
	)
}
