import { useMemo } from "react"
import { RefreshCw, Trophy } from "lucide-react"

import { achievements, library } from "../../wailsjs/go/models"
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogTitle,
} from "@/components/ui/dialog"
import { Bar, PillButton } from "@/components/settings/ui"
import { useAchievements } from "@/lib/use-achievements"
import { cn } from "@/lib/utils"

interface Props {
	open: boolean
	games: library.Game[]
	onOpenChange: (open: boolean) => void
	onSelect: (game: library.Game) => void
	// Opens the settings page where a launcher is set up.
	onSetup: (source: string) => void
}

// Launchers whose achievements need something set up on a settings tab.
const SETUP_TABS = new Set(["steam"])
const CLOSEST = 8

export function AchievementsDialog({ open, games, onOpenChange, onSelect, onSetup }: Props) {
	const { overview, rescan } = useAchievements(open)
	const byKey = useMemo(() => new Map(games.map((g) => [`${g.source}:${g.externalId}`, g])), [games])

	const closest = useMemo(
		() => (overview?.games ?? []).filter((g) => g.unlocked < g.total).slice(0, CLOSEST),
		[overview]
	)
	const percent = overview && overview.total > 0 ? Math.round((overview.unlocked / overview.total) * 100) : 0
	const anyReady = overview?.providers.some((p) => p.ready) ?? false
	const scanning = !!overview?.scanning

	function openGame(g: achievements.GameProgress) {
		const game = byKey.get(`${g.source}:${g.gameId}`)
		if (!game) return
		onOpenChange(false)
		onSelect(game)
	}

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent
				style={{
					backdropFilter: "blur(30px)",
					WebkitBackdropFilter: "blur(30px)",
					backgroundColor: "rgba(10, 10, 10, 0.6)",
				}}
				className="max-h-[85vh] gap-5 overflow-y-auto border-white/10 p-6 sm:max-w-xl"
			>
				<div className="flex items-start justify-between gap-4">
					<div>
						<DialogTitle className="text-[11px] font-medium tracking-[0.25em] text-muted-foreground uppercase">
							Achievements
						</DialogTitle>
						<DialogDescription className="mt-2 text-sm text-muted-foreground">
							How far you are in your games, from the launchers that allow it.
						</DialogDescription>
					</div>
					<button
						onClick={rescan}
						disabled={scanning}
						aria-label="Scan again"
						className="mt-0.5 rounded-full p-2 text-white/50 transition hover:bg-white/5 hover:text-white disabled:opacity-40"
					>
						<RefreshCw className={cn("size-4", scanning && "animate-spin")} />
					</button>
				</div>

				{anyReady && overview && (
					<>
						<div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
							<Tile label="Unlocked" value={overview.unlocked.toLocaleString()} />
							<Tile label="Available" value={overview.total.toLocaleString()} />
							<Tile label="Completed" value={overview.perfect.toLocaleString()} hint="games at 100%" />
							<Tile label="Overall" value={`${percent}%`} />
						</div>

						{scanning && (
							<div className="space-y-2">
								<Bar accent value={overview.pending > 0 ? (overview.done / overview.pending) * 100 : 0} />
								<p className="text-xs text-white/45">
									Scanning your games… {overview.done} of {overview.pending}
								</p>
							</div>
						)}

						{closest.length > 0 ? (
							<div>
								<p className="mb-3 text-[11px] tracking-wide text-white/45 uppercase">Closest to 100%</p>
								<ul className="space-y-1">
									{closest.map((g) => {
										const game = byKey.get(`${g.source}:${g.gameId}`)
										return (
											<li key={g.source + g.gameId}>
												<button
													disabled={!game}
													onClick={() => openGame(g)}
													className="flex w-full items-center gap-3 rounded-xl px-2 py-1.5 text-left transition hover:bg-white/5 disabled:pointer-events-none"
												>
													{game?.cover ? (
														<img src={game.cover} alt="" loading="lazy" className="h-10 w-7 shrink-0 rounded object-cover ring-1 ring-white/10" />
													) : (
														<div className="h-10 w-7 shrink-0 rounded bg-white/5" />
													)}
													<div className="min-w-0 flex-1">
														<p className="truncate text-sm font-medium">{g.name}</p>
														<div className="mt-1.5 flex items-center gap-3">
															<Bar accent value={(g.unlocked / g.total) * 100} className="flex-1" />
															<span className="shrink-0 text-xs text-white/45 tabular-nums">
																{g.unlocked}/{g.total}
															</span>
														</div>
													</div>
												</button>
											</li>
										)
									})}
								</ul>
							</div>
						) : (
							!scanning && (
								<p className="text-sm text-muted-foreground">
									{overview.total === 0
										? "No achievements found yet. Games you have played and that have achievements show up here."
										: "Every game you have played is at 100%."}
								</p>
							)
						)}
					</>
				)}

				<div className="space-y-3">
					{overview?.providers.map((p) => (
						<div key={p.source} className="rounded-2xl bg-white/[0.04] p-4 ring-1 ring-white/[0.06]">
							<div className="flex items-center justify-between gap-3">
								<p className="flex items-center gap-2 text-sm font-medium">
									<Trophy className="size-3.5 text-white/40" />
									{p.name}
								</p>
								{p.ready && !p.message && (
									<span className="text-xs text-white/45">
										{p.games} {p.games === 1 ? "game" : "games"}
									</span>
								)}
							</div>
							{p.message && <p className="mt-1.5 text-xs leading-relaxed text-white/50">{p.message}</p>}
							{SETUP_TABS.has(p.source) && (!p.ready || !!p.message) && (
								<div className="mt-3">
									<PillButton
										onClick={() => {
											onOpenChange(false)
											onSetup(p.source)
										}}
									>
										Open {p.name} settings
									</PillButton>
								</div>
							)}
						</div>
					))}
				</div>
			</DialogContent>
		</Dialog>
	)
}

function Tile({ label, value, hint }: { label: string; value: string; hint?: string }) {
	return (
		<div className="rounded-xl bg-white/[0.04] px-4 py-3 ring-1 ring-white/[0.06]">
			<p className="text-lg font-semibold tracking-tight tabular-nums">{value}</p>
			<p className="text-[11px] tracking-wide text-white/45 uppercase">{label}</p>
			{hint && <p className="text-[11px] text-white/30">{hint}</p>}
		</div>
	)
}
