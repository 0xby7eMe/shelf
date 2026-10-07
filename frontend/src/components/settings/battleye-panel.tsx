import { Check, Download, ShieldAlert } from "lucide-react"

import { Label, Panel, PillButton } from "@/components/settings/ui"
import { useBattlEyeRuntime } from "@/lib/use-battleye"
import { cn } from "@/lib/utils"

// The BattlEye runtime some games need to start on Linux.
export function BattlEyePanel() {
	const { runtime, install, installing, start } = useBattlEyeRuntime()
	const ready = !!runtime?.installed

	return (
		<Panel>
			<Label>BattlEye runtime</Label>
			<div className="flex items-start gap-3">
				<span
					className={cn(
						"mt-0.5 grid size-7 shrink-0 place-items-center rounded-full ring-1",
						ready ? "bg-emerald-400/15 text-emerald-300 ring-emerald-400/20" : "bg-amber-400/10 text-amber-300 ring-amber-300/20"
					)}
				>
					{ready ? <Check className="size-3.5" /> : <ShieldAlert className="size-3.5" />}
				</span>
				<div className="min-w-0">
					<p className="text-sm font-medium">{ready ? "Proton BattlEye Runtime is installed" : "Not installed"}</p>
					<p className="mt-1 text-sm text-white/55">
						{ready
							? runtime?.source === "steam"
								? "Using the copy from Steam."
								: "Downloaded by Shelf."
							: "Games that use BattlEye, such as Riders Republic, won't start without it. Steam's copy is used if you have it, otherwise it's about a 9 MB download."}
					</p>
				</div>
			</div>
			{installing && (
				<div className="h-1 overflow-hidden rounded-full bg-white/10">
					<div
						className="h-full rounded-full bg-white/70 transition-[width] duration-300"
						style={{ width: `${install?.percent ?? 0}%` }}
					/>
				</div>
			)}
			{install?.state === "failed" && <p className="text-sm break-words text-destructive">{install.error}</p>}
			{(!ready || runtime?.source === "shelf") && (
				<PillButton variant={ready ? "soft" : "solid"} disabled={installing} onClick={start}>
					<Download className={cn("size-3.5", installing && "animate-pulse")} />
					{installing ? `${install?.message ?? "Installing"}…` : ready ? "Reinstall" : "Install BattlEye runtime"}
				</PillButton>
			)}
		</Panel>
	)
}
