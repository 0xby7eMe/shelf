import { useState } from "react"

import { library } from "../../wailsjs/go/models"
import { formatPlaytime } from "@/lib/format"
import { cn } from "@/lib/utils"

interface Props {
  game: library.Game
  index: number
  onSelect: (game: library.Game) => void
  onHover?: (cover: string) => void
}

export function GameCard({ game, index, onSelect, onHover }: Props) {
  const [failed, setFailed] = useState(false)
  const cover = game.cover && !failed ? game.cover : null

  function track(e: React.MouseEvent<HTMLButtonElement>) {
    const r = e.currentTarget.getBoundingClientRect()
    e.currentTarget.style.setProperty("--x", `${e.clientX - r.left}px`)
    e.currentTarget.style.setProperty("--y", `${e.clientY - r.top}px`)
  }

  return (
    <button
      onClick={() => onSelect(game)}
      onMouseMove={track}
      onMouseEnter={() => cover && onHover?.(cover)}
      onFocus={() => cover && onHover?.(cover)}
      style={{ animationDelay: `${Math.min(index, 24) * 25}ms` }}
      className={cn(
        "group block w-full animate-in text-left outline-none duration-500 [animation-fill-mode:backwards] fade-in slide-in-from-bottom-2",
        !game.installed && "opacity-50 saturate-0 transition hover:opacity-100 hover:saturate-100"
      )}
    >
      <span
        className={cn(
          "relative block aspect-[2/3] overflow-hidden rounded-xl bg-white/[0.04]",
          "ring-1 ring-white/[0.08] shadow-[0_8px_24px_-8px_rgba(0,0,0,0.6)]",
          "transition duration-300 ease-out",
          "group-hover:-translate-y-1 group-hover:scale-[1.03] group-hover:ring-white/25",
          "group-hover:shadow-[0_24px_40px_-12px_rgba(0,0,0,0.85)]",
          "group-focus-visible:-translate-y-1 group-focus-visible:ring-white/50"
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
  )
}