import { useState } from "react"
import { ArrowLeft } from "lucide-react"

import { ResourcePanel, Tile, useResources, type ResourceId } from "@/components/monitor/panels"
import { WindowControls } from "@/components/window-controls"
import { useHardware } from "@/lib/hardware"

/**
 * What the machine is doing, in the manner of a task manager: a graph for each
 * part of the hardware. Sampling runs only while this page is open.
 */
export function MonitorPage({ onBack }: { onBack: () => void }) {
	const [selected, setSelected] = useState<ResourceId>("cpu")
	const hw = useHardware()
	const resources = useResources(hw)

	// A resource that has gone (an unplugged adapter) leaves nothing selected.
	const active = resources.some((r) => r.id === selected) ? selected : "cpu"

	return (
		<div className="fixed inset-0 z-40 flex animate-in flex-col bg-background text-foreground duration-200 fade-in">
			<header className="titlebar flex h-16 shrink-0 items-center gap-3 border-b border-white/5 bg-[rgba(10,10,10,0.55)] px-8 backdrop-blur-xl">
				<button
					onClick={onBack}
					className="flex h-9 items-center gap-2 rounded-full bg-white/5 pr-4 pl-3 text-xs text-white/80 ring-1 ring-white/10 transition hover:bg-white/10 hover:text-white"
				>
					<ArrowLeft className="size-3.5" />
					Library
				</button>
				<h1 className="ml-2 text-sm font-medium tracking-[0.25em] uppercase">Performance</h1>

				<div className="ml-auto flex items-center gap-4">
					{hw.info && <span className="hidden text-xs text-white/40 sm:block">{hw.info.host.hostname}</span>}
					<WindowControls />
				</div>
			</header>

			<main className="mx-auto flex min-h-0 w-full max-w-6xl flex-1 gap-6 px-8 py-6">
				<nav aria-label="Hardware" className="w-72 shrink-0 space-y-2 overflow-y-auto pr-1">
					{resources.length === 0 && <p className="px-2 text-xs text-white/40">Reading the hardware…</p>}
					{resources.map((r) => (
						<Tile key={r.id} r={r} selected={active === r.id} onSelect={() => setSelected(r.id)} />
					))}
				</nav>
				<div key={active} className="min-w-0 flex-1 animate-in overflow-y-auto pr-1 duration-300 fade-in">
					<ResourcePanel id={active} hw={hw} />
				</div>
			</main>
		</div>
	)
}
