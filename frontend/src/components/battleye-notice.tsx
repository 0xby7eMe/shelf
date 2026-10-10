import { Download, ShieldAlert } from "lucide-react"

import { library } from "../../wailsjs/go/models"
import type { useBattlEyeRuntime } from "@/lib/use-battleye"
import { platformNow } from "@/lib/platform"

type BattlEye = ReturnType<typeof useBattlEyeRuntime>

/** Whether a game's page should warn about the missing BattlEye runtime. */
export function needsBattlEyeNotice(game: library.Game, be: BattlEye): boolean {
	// Only Proton needs the runtime; Windows runs BattlEye itself.
	return platformNow().proton && game.antiCheat === "BattlEye" && game.installed && !!be.runtime && !be.runtime.installed
}

// On the page of a game that uses BattlEye, when the runtime it needs isn't
// there. Without it the game refuses to start.
export function BattlEyeNotice({ game, be }: { game: library.Game; be: BattlEye }) {
	const { install, installing, start } = be

	return (
		<div className="space-y-3 rounded-xl bg-amber-400/10 px-4 py-3 ring-1 ring-amber-300/20">
			<div className="flex items-start gap-3">
				<ShieldAlert className="mt-0.5 size-4 shrink-0 text-amber-300" />
				<p className="text-xs leading-relaxed text-white/70">
					{game.name} uses BattlEye. On Linux it needs the Proton BattlEye Runtime, a small download (about 9 MB), or
					it won't start.
				</p>
			</div>
			{installing && (
				<div className="h-1 overflow-hidden rounded-full bg-white/10">
					<div
						className="h-full rounded-full bg-white/70 transition-[width] duration-300"
						style={{ width: `${install?.percent ?? 0}%` }}
					/>
				</div>
			)}
			{install?.state === "failed" && <p className="text-xs break-words text-destructive">{install.error}</p>}
			<button
				onClick={start}
				disabled={installing}
				className="inline-flex h-9 items-center gap-2 rounded-full bg-white/10 px-4 text-xs font-medium text-white ring-1 ring-white/15 transition hover:bg-white/15 disabled:pointer-events-none disabled:opacity-50"
			>
				<Download className="size-3.5" />
				{installing ? `${install?.message ?? "Installing"}…` : "Install BattlEye runtime"}
			</button>
		</div>
	)
}
