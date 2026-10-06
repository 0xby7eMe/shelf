import { useEffect, useState } from "react"

import { library } from "../../wailsjs/go/models"
import { LiveDot } from "@/components/live-dot"
import { formatElapsed } from "@/lib/format"

function useElapsed(since: number) {
	const [now, setNow] = useState(() => Date.now())
	useEffect(() => {
		const t = setInterval(() => setNow(Date.now()), 1000)
		return () => clearInterval(t)
	}, [])
	return Math.max(0, Math.floor((now - since) / 1000))
}

interface Props {
	game: library.Game
	since: number
	onSelect: (game: library.Game) => void
}

export function NowPlaying({ game, since, onSelect }: Props) {
	const elapsed = useElapsed(since)

	return (
		<button
			onClick={() => onSelect(game)}
			title={`Playing ${game.name}`}
			className="hidden h-9 max-w-52 animate-in items-center gap-2 rounded-full bg-emerald-500/30 pr-3.5 pl-3 text-xs ring-1 ring-emerald-500/50 backdrop-blur-md transition duration-300 fade-in slide-in-from-top-1 hover:bg-emerald-500/20 md:flex"
		>
			<LiveDot />
			<span className="truncate text-white/80">{game.name}</span>
			<span className="shrink-0 tabular-nums text-white/45">{formatElapsed(elapsed)}</span>
		</button>
	)
}