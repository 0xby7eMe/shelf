import { useMemo } from "react"

import { library } from "../../wailsjs/go/models"
import { Heatmap } from "@/components/heatmap"
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogTitle,
} from "@/components/ui/dialog"
import { formatDuration, formatLastPlayed } from "@/lib/format"

interface Props {
	open: boolean
	stats: library.Stats | null
	games: library.Game[]
	onOpenChange: (open: boolean) => void
	onSelect: (game: library.Game) => void
}

export function ActivityDialog({ open, stats, games, onOpenChange, onSelect }: Props) {
	const byId = useMemo(() => new Map(games.map((g) => [g.externalId, g])), [games])

	const top = useMemo(
		() =>
			(stats?.games ?? [])
				.filter((g) => g.weekMinutes > 0)
				.sort((a, b) => b.weekMinutes - a.weekMinutes)
				.slice(0, 5),
		[stats]
	)
	const maxWeek = top[0]?.weekMinutes ?? 1

	const delta =
		stats && stats.prevWeekMinutes > 0
			? Math.round(((stats.weekMinutes - stats.prevWeekMinutes) / stats.prevWeekMinutes) * 100)
			: null

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent
				style={{
					backdropFilter: "blur(30px)",
					WebkitBackdropFilter: "blur(30px)",
					backgroundColor: "rgba(10, 10, 10, 0.6)",
				}}
				className="gap-6 border-white/10 p-6 sm:max-w-xl"
			>
				<div>
					<DialogTitle className="text-[11px] font-medium tracking-[0.25em] text-muted-foreground uppercase">
						Activity
					</DialogTitle>
					<DialogDescription className="sr-only">
						Your play time recorded by Shelf
					</DialogDescription>
				</div>

				{stats && (
					<>
						<div className="grid grid-cols-3 gap-3">
							<Tile
								label="This week"
								value={formatDuration(stats.weekMinutes)}
								hint={
									delta === null
										? undefined
										: `${delta >= 0 ? "+" : "−"}${Math.abs(delta)}% vs last week`
								}
							/>
							<Tile label="Last week" value={formatDuration(stats.prevWeekMinutes)} />
							<Tile
								label="Tracked"
								value={formatDuration(stats.trackedMinutes)}
								hint={
									stats.since > 0
										? `since ${formatLastPlayed(Math.floor(stats.since / 1000))}`
										: undefined
								}
							/>
						</div>

						<Heatmap days={stats.days} />

						<div>
							<p className="mb-3 text-[11px] tracking-wide text-white/45 uppercase">
								Most played this week
							</p>

							{top.length === 0 ? (
								<p className="text-sm text-muted-foreground">
									{stats.sessions === 0
										? "Nothing recorded yet. Play a game while Shelf is open and it shows up here."
										: "No play time this week."}
								</p>
							) : (
								<ul className="space-y-1">
									{top.map((g) => {
										const game = byId.get(g.appId)
										return (
											<li key={g.appId}>
												<button
													disabled={!game}
													onClick={() => {
														if (!game) return
														onOpenChange(false)
														onSelect(game)
													}}
													className="flex w-full items-center gap-3 rounded-xl px-2 py-1.5 text-left transition hover:bg-white/5 disabled:pointer-events-none"
												>
													{game?.cover ? (
														<img
															src={game.cover}
															alt=""
															loading="lazy"
															className="h-10 w-7 shrink-0 rounded object-cover ring-1 ring-white/10"
														/>
													) : (
														<div className="h-10 w-7 shrink-0 rounded bg-white/5" />
													)}
													<div className="min-w-0 flex-1">
														<div className="flex items-baseline justify-between gap-3">
															<span className="truncate text-sm text-white/85">
																{game?.name ?? `App ${g.appId}`}
															</span>
															<span className="shrink-0 text-xs tabular-nums text-white/45">
																{formatDuration(g.weekMinutes)}
															</span>
														</div>
														<div className="mt-1.5 h-1 overflow-hidden rounded-full bg-white/10">
															<div
																className="h-full rounded-full bg-white/70"
																style={{ width: `${(g.weekMinutes / maxWeek) * 100}%` }}
															/>
														</div>
													</div>
												</button>
											</li>
										)
									})}
								</ul>
							)}
						</div>
					</>
				)}
			</DialogContent>
		</Dialog>
	)
}

function Tile({ label, value, hint }: { label: string; value: string; hint?: string }) {
	return (
		<div className="rounded-xl bg-white/5 px-4 py-3 ring-1 ring-white/[0.06]">
			<p className="text-[11px] tracking-wide text-white/45 uppercase">{label}</p>
			<p className="mt-1 text-base font-medium tabular-nums">{value}</p>
			{hint && <p className="text-[11px] text-white/35">{hint}</p>}
		</div>
	)
}