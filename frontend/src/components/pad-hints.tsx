import { PadGlyph, type PadAction } from "@/components/pad-glyph"
import { useInputMode } from "@/lib/gamepad"

function Hint({ action, label }: { action: PadAction; label: string }) {
	return (
		<span className="flex items-center gap-1.5">
			<PadGlyph action={action} />
			<span className="text-white/60">{label}</span>
		</span>
	)
}

// Button hints along the bottom while a controller is in use.
export function PadHints({ inSettings }: { inSettings: boolean }) {
	const mode = useInputMode()
	if (mode !== "pad") return null

	return (
		<div
			style={{ backdropFilter: "blur(20px)", WebkitBackdropFilter: "blur(20px)" }}
			className="pointer-events-none fixed bottom-5 left-6 z-[90] flex animate-in items-center gap-4 rounded-full bg-[rgba(10,10,10,0.7)] px-4 py-2 text-xs ring-1 ring-white/10 duration-300 fade-in slide-in-from-bottom-2"
		>
			<Hint action="confirm" label="Select" />
			<Hint action="back" label="Back" />
			{!inSettings && <Hint action="favorite" label="Favorite" />}
			{!inSettings && <Hint action="random" label="Random" />}
			<Hint action="tabs" label="Tabs" />
			<Hint action="menu" label="Settings" />
		</div>
	)
}
