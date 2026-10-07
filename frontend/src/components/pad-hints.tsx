import { useInputMode } from "@/lib/gamepad"

function Hint({ button, label }: { button: string; label: string }) {
	return (
		<span className="flex items-center gap-1.5">
			<kbd className="grid min-w-5 place-items-center rounded-full bg-white/15 px-1.5 text-[10px] font-semibold text-white">
				{button}
			</kbd>
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
			<Hint button="A" label="Select" />
			<Hint button="B" label="Back" />
			{!inSettings && <Hint button="X" label="Favorite" />}
			{!inSettings && <Hint button="Y" label="Random" />}
			<Hint button="LB RB" label="Tabs" />
			<Hint button="☰" label="Settings" />
		</div>
	)
}
