import { useMemo } from "react"
import { ArrowDown, ArrowUp, ChevronsUp, Pause, Play, X } from "lucide-react"

import {
	EpicCancelInstall,
	EpicQueueMove,
	EpicSetQueuePaused,
	EpicUpdate,
	GogUpdate,
} from "../../../wailsjs/go/main/App"
import { epic, library } from "../../../wailsjs/go/models"
import { Bar, IconButton, Panel, PillButton, SectionHeading } from "@/components/settings/ui"
import { jobLabel } from "@/lib/use-epic"
import { toast } from "@/lib/toast"

interface Props {
	queue: epic.QueueState
	games: library.Game[]
}

export function DownloadsSection({ queue, games }: Props) {
	const byApp = useMemo(
		() => new Map(games.filter((g) => g.source === "epic" || g.source === "gog").map((g) => [g.externalId, g])),
		[games]
	)
	const outdated = games.filter(
		(g) =>
			(g.source === "epic" || g.source === "gog") &&
			g.installed &&
			g.updateAvailable &&
			!queue.jobs.some((j) => j.appName === g.externalId)
	)
	const queuedCount = queue.jobs.filter((j) => j.state === "queued").length

	function fail(what: string, e: unknown) {
		toast.error(`Couldn't ${what}`, { description: String(e) })
	}

	function updateAll() {
		for (const g of outdated)
			(g.source === "gog" ? GogUpdate(g.externalId) : EpicUpdate(g.externalId)).catch((e) => fail(`update ${g.name}`, e))
	}

	return (
		<div className="space-y-6">
			<SectionHeading
				title="Downloads"
				hint="Installs, updates and repairs run one at a time, in this order."
			/>

			<div className="flex flex-wrap gap-2">
				<PillButton onClick={() => EpicSetQueuePaused(!queue.paused)} disabled={queue.jobs.length === 0 && !queue.paused}>
					{queue.paused ? <Play className="size-3.5" /> : <Pause className="size-3.5" />}
					{queue.paused ? "Resume queue" : "Pause queue"}
				</PillButton>
				{outdated.length > 0 && (
					<PillButton variant="solid" onClick={updateAll}>
						Update all ({outdated.length})
					</PillButton>
				)}
			</div>

			{queue.paused && (
				<p className="text-sm text-white/55">
					The queue is paused. A download that's already running carries on; nothing new starts.
				</p>
			)}

			{queue.jobs.length === 0 ? (
				<Panel>
					<p className="text-sm text-white/45">Nothing downloading. Installs and updates you start show up here.</p>
				</Panel>
			) : (
				<ul className="space-y-2">
					{queue.jobs.map((job) => {
						const game = byApp.get(job.appName)
						const queued = job.state === "queued"
						return (
							<li
								key={job.appName}
								className="flex items-center gap-4 rounded-2xl bg-white/[0.04] p-4 ring-1 ring-white/[0.06]"
							>
								{game?.cover ? (
									<img src={game.cover} alt="" className="h-14 w-10 shrink-0 rounded-md object-cover ring-1 ring-white/10" />
								) : (
									<div className="h-14 w-10 shrink-0 rounded-md bg-white/5" />
								)}
								<div className="min-w-0 flex-1">
									<p className="truncate text-sm font-medium">{game?.name ?? job.appName}</p>
									<p className="mt-0.5 text-xs text-white/45 tabular-nums">
										{queued
											? `${jobLabel(job)} · #${job.position}`
											: [jobLabel(job), job.percent > 0 && `${Math.floor(job.percent)}%`, job.speed, job.eta && `ETA ${job.eta}`]
													.filter(Boolean)
													.join(" · ")}
									</p>
									{!queued && <Bar value={job.percent} className="mt-2" />}
								</div>
								<div className="flex shrink-0 gap-1.5">
									{queued && queuedCount > 1 && (
										<>
											<IconButton
												label="Run next"
												disabled={job.position === 1}
												onClick={() => EpicQueueMove(job.appName, -1000).catch((e) => fail("reorder", e))}
											>
												<ChevronsUp className="size-4" />
											</IconButton>
											<IconButton
												label="Move up"
												disabled={job.position === 1}
												onClick={() => EpicQueueMove(job.appName, -1).catch((e) => fail("reorder", e))}
											>
												<ArrowUp className="size-4" />
											</IconButton>
											<IconButton
												label="Move down"
												disabled={job.position === queuedCount}
												onClick={() => EpicQueueMove(job.appName, 1).catch((e) => fail("reorder", e))}
											>
												<ArrowDown className="size-4" />
											</IconButton>
										</>
									)}
									<IconButton label="Cancel" onClick={() => EpicCancelInstall(job.appName)}>
										<X className="size-4" />
									</IconButton>
								</div>
							</li>
						)
					})}
				</ul>
			)}
		</div>
	)
}
