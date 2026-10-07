import { memo, useState } from "react"
import { ArrowUpCircle, Heart } from "lucide-react"

import { epic, library } from "../../wailsjs/go/models"
import { formatPlaytime } from "@/lib/format"
import { jobLabel } from "@/lib/use-epic"
import { cn } from "@/lib/utils"
import { LiveDot } from "@/components/live-dot"

interface Props {
	game: library.Game
	index: number
	favorite: boolean
	onSelect: (game: library.Game) => void
	onToggleFavorite: (game: library.Game) => void
	playing?: boolean
	job?: epic.Progress // a running install, update or similar
}

// Only the first screenful fades in; animating hundreds of cards at once is slow.
const ANIMATED_CARDS = 24

// Everything per-card is kept cheap, because a library can hold hundreds of
// them: no backdrop blur, and hover animates only transform and opacity, which
// the compositor can do without repainting.
export const GameCard = memo(function GameCard({
	game,
	index,
	favorite,
	onSelect,
	onToggleFavorite,
	playing,
	job,
}: Props) {
	const [failed, setFailed] = useState(false)
	const cover = game.cover && !failed ? game.cover : null
	const chip =
		"absolute z-10 rounded-full bg-black/70 px-2 py-1 text-[10px] font-medium text-white ring-1 ring-white/10 " +
		"transition-transform duration-300 group-data-[hover]:-translate-y-1"

	return (
		<div
			style={index < ANIMATED_CARDS ? { animationDelay: `${index * 25}ms` } : undefined}
			className={cn(
				"group relative",
				index < ANIMATED_CARDS &&
					"animate-in duration-500 [animation-fill-mode:backwards] fade-in slide-in-from-bottom-2",
				!game.installed &&
					!job &&
					"opacity-50 transition-opacity data-[hover]:opacity-100 [&_img]:saturate-0 data-[hover]:[&_img]:saturate-100"
			)}
		>
			<button onClick={() => onSelect(game)} data-card className="block w-full text-left outline-none">
				<span
					data-poster
					className={cn(
						"relative block aspect-[2/3] rounded-xl shadow-[0_8px_24px_-8px_rgba(0,0,0,0.6)]",
						"transition-[translate,scale] duration-300 ease-out",
						"group-data-[hover]:-translate-y-1 group-data-[hover]:scale-[1.03] group-data-[hover]:will-change-transform",
						"group-has-[:focus-visible]:-translate-y-1"
					)}
				>
					{/* The deep hover shadow fades in; animating box-shadow itself repaints every frame. */}
					<span
						aria-hidden
						className="absolute inset-0 rounded-xl opacity-0 shadow-[0_24px_40px_-12px_rgba(0,0,0,0.85)] transition-opacity duration-300 group-data-[hover]:opacity-100"
					/>
					<span className="absolute inset-0 overflow-hidden rounded-xl bg-white/[0.04] ring-1 ring-white/[0.08]">
						{cover ? (
							<img
								src={cover}
								alt={game.name}
								loading="lazy"
								decoding="async"
								onError={() => setFailed(true)}
								className="size-full object-cover"
							/>
						) : (
							<span className="flex size-full items-center justify-center p-4 text-center text-sm text-muted-foreground">
								{game.name}
							</span>
						)}
						<span
							aria-hidden
							className="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-data-[hover]:opacity-100 bg-[radial-gradient(200px_circle_at_var(--x,50%)_var(--y,0%),rgba(255,255,255,0.16),transparent_70%)]"
						/>
						<span
							aria-hidden
							className="pointer-events-none absolute inset-0 rounded-xl opacity-0 ring-1 ring-white/25 transition-opacity duration-300 ring-inset group-data-[hover]:opacity-100 group-has-[:focus-visible]:opacity-100"
						/>
					</span>
				</span>

				<span className="mt-3 flex items-baseline justify-between gap-2 px-0.5">
					<span className="truncate text-[13px] text-white/70 transition-colors duration-300 group-data-[hover]:text-white">
						{game.name}
					</span>
					<span className="shrink-0 text-[11px] tabular-nums text-white/35">
						{game.playtimeMinutes > 0 ? formatPlaytime(game.playtimeMinutes) : ""}
					</span>
				</span>
			</button>

			<button
				data-fav
				data-nav-skip
				onClick={() => onToggleFavorite(game)}
				aria-label={favorite ? "Remove from favorites" : "Add to favorites"}
				aria-pressed={favorite}
				className={cn(
					"absolute top-2 right-2 grid size-8 place-items-center rounded-full bg-black/60 ring-1 ring-white/10",
					"transition-[translate,opacity,background-color] duration-300 group-data-[hover]:-translate-y-1 hover:bg-black/80",
					"focus-visible:opacity-100 focus-visible:ring-white/50 focus-visible:outline-none",
					favorite ? "text-white opacity-100" : "text-white/70 opacity-0 group-data-[hover]:opacity-100"
				)}
			>
				<Heart className={cn("size-3.5", favorite && "fill-current")} />
			</button>

			{job ? (
				<span className={cn(chip, "bottom-10 left-2 tabular-nums")}>
					{jobLabel(job)}
					{job.state === "installing" && job.percent > 0 && ` ${Math.floor(job.percent)}%`}
				</span>
			) : (
				game.updateAvailable && (
					<span className={cn(chip, "bottom-10 left-2 flex items-center gap-1")}>
						<ArrowUpCircle className="size-3" />
						Update
					</span>
				)
			)}

			{playing && (
				<span className={cn(chip, "top-2 left-2 flex items-center gap-1.5")}>
					<LiveDot />
					Playing
				</span>
			)}
		</div>
	)
})
