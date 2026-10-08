import { useEffect, useState } from "react"
import { Lock } from "lucide-react"

import { GetGameAchievements } from "../../wailsjs/go/main/App"
import { achievements, library } from "../../wailsjs/go/models"
import { Bar } from "@/components/settings/ui"
import { formatLastPlayed } from "@/lib/format"
import { cn } from "@/lib/utils"

const PREVIEW = 4

// A game's achievements, in its sheet. Only the launchers that can say show up
// here; for the others nothing is drawn, since the overview already says why.
export function GameAchievements({ game }: { game: library.Game }) {
	const [detail, setDetail] = useState<achievements.Detail | null>(null)
	const [error, setError] = useState("")
	const [all, setAll] = useState(false)

	useEffect(() => {
		let alive = true
		setDetail(null)
		setError("")
		setAll(false)
		GetGameAchievements(game.id)
			.then((d) => alive && setDetail(d))
			.catch((e) => alive && setError(String(e)))
		return () => {
			alive = false
		}
	}, [game.id])

	// Nothing to show: no answer yet, no achievements, or a launcher that can't say.
	if (error || !detail || detail.total === 0) return null

	const shown = all ? detail.achievements : detail.achievements.slice(0, PREVIEW)
	const pct = Math.round((detail.unlocked / detail.total) * 100)

	return (
		<div className="space-y-3">
			<div className="flex items-baseline justify-between">
				<p className="text-[11px] tracking-wide text-white/45 uppercase">Achievements</p>
				<p className="text-xs text-white/55 tabular-nums">
					{detail.unlocked} / {detail.total} · {pct}%
				</p>
			</div>
			<Bar bright value={pct} />

			<ul className={cn("space-y-1", all && "max-h-72 overflow-y-auto pr-1")}>
				{shown.map((a) => (
					<li key={a.id} className={cn("flex items-center gap-3 rounded-xl px-2 py-1.5", !a.unlocked && "opacity-55")}>
						{a.icon ? (
							<img src={a.icon} alt="" loading="lazy" className="size-9 shrink-0 rounded-md bg-white/5 object-cover" />
						) : (
							<div className="grid size-9 shrink-0 place-items-center rounded-md bg-white/5">
								<Lock className="size-3.5 text-white/30" />
							</div>
						)}
						<div className="min-w-0 flex-1">
							<p className="truncate text-sm font-medium">{a.name}</p>
							<p className="truncate text-xs text-white/45">
								{a.description || (a.hidden && !a.unlocked ? "Hidden achievement" : "")}
							</p>
						</div>
						<div className="shrink-0 text-right text-[11px] text-white/35 tabular-nums">
							{a.unlocked && a.unlockedAt ? <p>{formatLastPlayed(a.unlockedAt)}</p> : null}
							{a.percent >= 0 && <p>{a.percent < 10 ? a.percent.toFixed(1) : Math.round(a.percent)}% of players</p>}
						</div>
					</li>
				))}
			</ul>

			{detail.achievements.length > PREVIEW && (
				<button
					onClick={() => setAll((v) => !v)}
					className="text-xs text-white/45 underline-offset-2 transition hover:text-white hover:underline"
				>
					{all ? "Show fewer" : `Show all ${detail.achievements.length}`}
				</button>
			)}
		</div>
	)
}
