import { useState } from "react"
import { Heart } from "lucide-react"

import { library } from "../../wailsjs/go/models"
import { formatPlaytime } from "@/lib/format"
import { cn } from "@/lib/utils"
import { LiveDot } from "@/components/live-dot"

interface Props {
	game: library.Game
	index: number
	favorite: boolean
	onSelect: (game: library.Game) => void
	onToggleFavorite: (game: library.Game) => void
	playing?: boolean
	installing?: number // percent, while an install is running
}

export function GameCard({ game, index, favorite, onSelect, onToggleFavorite, playing, installing }: Props) {
	const [failed, setFailed] = useState(false)
	const cover = game.cover && !failed ? game.cover : null

	function track(e: React.MouseEvent<HTMLButtonElement>) {
		const r = e.currentTarget.getBoundingClientRect()
		e.currentTarget.style.setProperty("--x", `${e.clientX - r.left}px`)
		e.currentTarget.style.setProperty("--y", `${e.clientY - r.top}px`)
	}

	return (
		<div
			style={{ animationDelay: `${Math.min(index, 24) * 25}ms` }}
			className={cn(
				"group relative animate-in duration-500 [animation-fill-mode:backwards] fade-in slide-in-from-bottom-2",
				!game.installed && installing === undefined && "opacity-50 saturate-0 transition hover:opacity-100 hover:saturate-100"
			)}
		>
			<button
				onClick={() => onSelect(game)}
				onMouseMove={track}
				className="block w-full text-left outline-none"
			>
				<span
					className={cn(
						"relative block aspect-[2/3] overflow-hidden rounded-xl bg-white/[0.04]",
						"ring-1 ring-white/[0.08] shadow-[0_8px_24px_-8px_rgba(0,0,0,0.6)]",
						"transition duration-300 ease-out",
						"group-hover:-translate-y-1 group-hover:scale-[1.03] group-hover:ring-white/25",
						"group-hover:shadow-[0_24px_40px_-12px_rgba(0,0,0,0.85)]",
						"group-has-[:focus-visible]:-translate-y-1 group-has-[:focus-visible]:ring-white/50"
					)}
				>
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
						className="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-hover:opacity-100 bg-[radial-gradient(200px_circle_at_var(--x,50%)_var(--y,0%),rgba(255,255,255,0.16),transparent_70%)]"
					/>
				</span>

				<span className="mt-3 flex items-baseline justify-between gap-2 px-0.5">
					<span className="truncate text-[13px] text-white/70 transition-colors duration-300 group-hover:text-white">
						{game.name}
					</span>
					<span className="shrink-0 text-[11px] tabular-nums text-white/35">
						{game.playtimeMinutes > 0 ? formatPlaytime(game.playtimeMinutes) : ""}
					</span>
				</span>
			</button>

			<button
				onClick={() => onToggleFavorite(game)}
				aria-label={favorite ? "Remove from favorites" : "Add to favorites"}
				aria-pressed={favorite}
				className={cn(
					"absolute top-2 right-2 grid size-8 place-items-center rounded-full bg-black/40 ring-1 ring-white/10 backdrop-blur-md",
					"transition duration-300 group-hover:-translate-y-1 hover:bg-black/60",
					"focus-visible:opacity-100 focus-visible:ring-white/50 focus-visible:outline-none",
					favorite ? "text-white opacity-100" : "text-white/70 opacity-0 group-hover:opacity-100"
				)}
			>
				<Heart className={cn("size-3.5", favorite && "fill-current")} />
			</button>

			{installing !== undefined && (
				<span className="absolute bottom-10 left-2 z-10 rounded-full bg-black/60 px-2 py-1 text-[10px] font-medium tabular-nums text-white ring-1 ring-white/10 backdrop-blur-md">
					Installing {Math.floor(installing)}%
				</span>
			)}

			{playing && (
				<span className="absolute top-2 left-2 z-10 flex items-center gap-1.5 rounded-full bg-black/50 px-2 py-1 text-[10px] font-medium text-white ring-1 ring-white/10 backdrop-blur-md transition duration-300 group-hover:-translate-y-1">
					<LiveDot />
					Playing
				</span>
			)}
		</div>
	)
}