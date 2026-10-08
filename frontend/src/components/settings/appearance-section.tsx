import { ToggleRow } from "@/components/toggle-row"
import { Label, Panel, PillButton, SectionHeading, Segmented } from "@/components/settings/ui"
import { resetAppearance, setPrefs, UI_SCALES, usePrefs, type PosterSize } from "@/lib/prefs"

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
