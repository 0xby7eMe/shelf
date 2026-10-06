import { useState } from "react"
import { Play } from "lucide-react"

import { library } from "../../wailsjs/go/models"
import { formatLastPlayed, formatPlaytime } from "@/lib/format"

interface Props {
  game: library.Game
  onPlay: (game: library.Game) => void
  onDetails: (game: library.Game) => void
}

export function Hero({ game, onPlay, onDetails }: Props) {
  const [heroFailed, setHeroFailed] = useState(false)

  return (
    <section className="relative isolate -mt-16 h-[52vh] min-h-[340px] overflow-hidden">
      {!heroFailed ? (
        <img
          src={`${game.cover}?kind=hero`}
          alt=""
          onError={() => setHeroFailed(true)}
          className="absolute inset-0 size-full animate-in object-cover object-top duration-1000 fade-in"
        />
      ) : (
        <img
          src={game.cover}
          alt=""
          aria-hidden
          className="absolute inset-0 size-full scale-125 object-cover opacity-50 blur-3xl saturate-150"
        />
      )}

      <div className="absolute inset-0 bg-gradient-to-t from-background via-background/50 to-background/10" />
      <div className="absolute inset-0 bg-gradient-to-r from-background/90 via-background/30 to-transparent" />

      <div className="relative flex h-full flex-col justify-end gap-4 px-8 pb-10">
        <p className="text-[11px] font-medium tracking-[0.25em] text-white/50 uppercase">
          {game.lastPlayed > 0 ? "Jump back in" : "Featured"}
        </p>
        <h2 className="max-w-2xl text-4xl font-semibold tracking-tight text-balance sm:text-5xl">
          {game.name}
        </h2>
        <p className="text-sm tabular-nums text-white/60">
          {formatPlaytime(game.playtimeMinutes)} · Last played{" "}
          {formatLastPlayed(game.lastPlayed)}
        </p>

        <div className="flex gap-2 pt-1">
          <button
            onClick={() => onPlay(game)}
            className="inline-flex h-10 items-center gap-2 rounded-full bg-white px-5 text-sm font-medium text-black transition duration-300 hover:shadow-[0_0_40px_rgba(255,255,255,0.35)]"
          >
            <Play className="size-3.5 fill-current" />
            Play
          </button>
          <button
            onClick={() => onDetails(game)}
            className="inline-flex h-10 items-center rounded-full bg-white/10 px-5 text-sm font-medium ring-1 ring-white/10 backdrop-blur-md transition hover:bg-white/15"
          >
            Details
          </button>
        </div>
      </div>
    </section>
  )
}