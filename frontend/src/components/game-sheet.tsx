import { useState } from "react"
import { ExternalLink, FolderOpen, Play } from "lucide-react"

import { OpenInstallFolder, OpenStorePage } from "../../wailsjs/go/main/App"
import { library } from "../../wailsjs/go/models"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
} from "@/components/ui/sheet"
import { formatLastPlayed, formatPlaytime, relativeTime } from "@/lib/format"
import { cn } from "@/lib/utils"

interface Props {
  game: library.Game | null
  totalMinutes: number
  onClose: () => void
  onPlay: (game: library.Game) => void
}

export function GameSheet({ game, totalMinutes, onClose, onPlay }: Props) {
  return (
    <Sheet open={game !== null} onOpenChange={(open) => !open && onClose()}>
      <SheetContent
        style={{
          backdropFilter: "blur(30px)",
          WebkitBackdropFilter: "blur(30px)",
          backgroundColor: "rgba(10, 10, 10, 0.6)",
        }}
        className="gap-0 overflow-y-auto border-white/10 p-0 sm:max-w-md"
      >
        {game && (
          <Body key={game.id} game={game} totalMinutes={totalMinutes} onPlay={onPlay} />
        )}
      </SheetContent>
    </Sheet>
  )
}

function Body({
  game,
  totalMinutes,
  onPlay,
}: {
  game: library.Game
  totalMinutes: number
  onPlay: (game: library.Game) => void
}) {
  const [heroFailed, setHeroFailed] = useState(false)
  const share = totalMinutes > 0 ? (game.playtimeMinutes / totalMinutes) * 100 : 0

  return (
    <>
      {/* Banner */}
      <div className="relative h-52 shrink-0 overflow-hidden">
        {!heroFailed ? (
          <img
            src={`${game.cover}?kind=hero`}
            alt=""
            onError={() => setHeroFailed(true)}
            className="size-full animate-in object-cover duration-700 fade-in"
          />
        ) : (
          <img
            src={game.cover}
            alt=""
            aria-hidden
            className="size-full scale-125 object-cover opacity-60 blur-3xl saturate-150"
          />
        )}
        <div className="absolute inset-0 bg-gradient-to-t from-[rgba(10,10,10,0.95)] via-[rgba(10,10,10,0.3)] to-transparent" />
      </div>

      {/* Poster overlapping the banner, title beside it */}
      <div className="relative -mt-20 flex items-end gap-4 px-6">
        <img
          src={game.cover}
          alt=""
          className="aspect-[2/3] w-28 shrink-0 rounded-xl object-cover shadow-[0_16px_32px_-8px_rgba(0,0,0,0.8)] ring-1 ring-white/15"
        />
        <div className="min-w-0 pb-1">
          <p className="mb-1.5 text-[11px] tracking-[0.2em] text-white/45 uppercase">
            {game.source} · {game.installed ? "Installed" : "Not installed"}
          </p>
          <SheetTitle className="text-xl leading-tight font-semibold tracking-tight text-balance">
            {game.name}
          </SheetTitle>
          <SheetDescription className="sr-only">
            Details for {game.name}
          </SheetDescription>
        </div>
      </div>

      <div className="space-y-6 px-6 pt-6 pb-8">
        <button
          onClick={() => onPlay(game)}
          className="flex h-11 w-full items-center justify-center gap-2 rounded-full bg-white text-sm font-medium text-black transition duration-300 hover:shadow-[0_0_40px_rgba(255,255,255,0.3)]"
        >
          <Play className="size-3.5 fill-current" />
          {game.installed ? "Play" : "Install in Steam"}
        </button>

        <div className="grid grid-cols-2 gap-3">
          <Stat label="Playtime" value={formatPlaytime(game.playtimeMinutes)} />
          <Stat
            label="Last played"
            value={relativeTime(game.lastPlayed)}
            hint={game.lastPlayed > 0 ? formatLastPlayed(game.lastPlayed) : undefined}
          />
        </div>

        {game.playtimeMinutes > 0 && (
          <div>
            <div className="mb-2 flex justify-between text-[11px] tracking-wide text-white/45 uppercase">
              <span>Share of library</span>
              <span className="tabular-nums">
                {share < 1 ? "<1" : Math.round(share)}%
              </span>
            </div>
            <div className="h-1 overflow-hidden rounded-full bg-white/10">
              <div
                className="h-full rounded-full bg-white/70 transition-[width] duration-700 ease-out"
                style={{ width: `${Math.max(share, 1)}%` }}
              />
            </div>
          </div>
        )}

        <div className="flex gap-2">
          <Action icon={ExternalLink} onClick={() => OpenStorePage(game.externalId)}>
            Store page
          </Action>
          <Action
            icon={FolderOpen}
            disabled={!game.installed}
            onClick={() => OpenInstallFolder(game.externalId)}
          >
            Open folder
          </Action>
        </div>

        {game.installPath && (
          <p className="text-[11px] break-all text-white/30">{game.installPath}</p>
        )}
      </div>
    </>
  )
}

function Stat({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="rounded-xl bg-white/5 px-4 py-3 ring-1 ring-white/[0.06]">
      <p className="text-[11px] tracking-wide text-white/45 uppercase">{label}</p>
      <p className="mt-1 text-base font-medium tabular-nums">{value}</p>
      {hint && <p className="text-[11px] text-white/35">{hint}</p>}
    </div>
  )
}

function Action({
  icon: Icon,
  children,
  disabled,
  onClick,
}: {
  icon: React.ComponentType<{ className?: string }>
  children: React.ReactNode
  disabled?: boolean
  onClick: () => void
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={cn(
        "flex h-10 flex-1 items-center justify-center gap-2 rounded-full bg-white/5 text-xs text-white/80 ring-1 ring-white/10 transition",
        "hover:bg-white/10 hover:text-white disabled:pointer-events-none disabled:opacity-30"
      )}
    >
      <Icon className="size-3.5" />
      {children}
    </button>
  )
}