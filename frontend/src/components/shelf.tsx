import { useCallback, useEffect, useRef, useState } from "react"
import { ChevronLeft, ChevronRight } from "lucide-react"

import { library } from "../../wailsjs/go/models"
import { GameCard } from "@/components/game-card"

interface Props {
    title: string
    count?: number
    games: library.Game[]
    favorites: Set<string>
    onSelect: (game: library.Game) => void
    onToggleFavorite: (game: library.Game) => void
}

export function Shelf({ title, count, games, favorites, onSelect, onToggleFavorite }: Props) {
    const ref = useRef<HTMLDivElement>(null)
    const [edge, setEdge] = useState({ start: true, end: true })

    const update = useCallback(() => {
        const el = ref.current
        if (!el) return
        const start = el.scrollLeft <= 2
        const end = el.scrollLeft + el.clientWidth >= el.scrollWidth - 2
        // Return the old object when nothing changed, so scrolling doesn't re-render.
        setEdge((prev) => (prev.start === start && prev.end === end ? prev : { start, end }))
    }, [])

    useEffect(() => {
        update()
        const el = ref.current
        if (!el) return
        const ro = new ResizeObserver(update)
        ro.observe(el)
        return () => ro.disconnect()
    }, [update, games.length])

    function scrollBy(dir: 1 | -1) {
        const el = ref.current
        el?.scrollBy({ left: dir * el.clientWidth * 0.8, behavior: "smooth" })
    }

    const overflowing = !(edge.start && edge.end)

    return (
        <section className="mb-10">
            <div className="mb-4 flex items-center justify-between">
                <div className="flex items-baseline gap-3">
                    <h2 className="text-[11px] font-medium tracking-[0.25em] text-muted-foreground uppercase">
                        {title}
                    </h2>
                    {count !== undefined && (
                        <span className="text-xs tabular-nums text-muted-foreground/60">{count}</span>
                    )}
                </div>

                {overflowing && (
                    <div className="flex gap-1">
                        <Arrow label="Scroll left" disabled={edge.start} onClick={() => scrollBy(-1)}>
                            <ChevronLeft className="size-4" />
                        </Arrow>
                        <Arrow label="Scroll right" disabled={edge.end} onClick={() => scrollBy(1)}>
                            <ChevronRight className="size-4" />
                        </Arrow>
                    </div>
                )}
            </div>

            {/* Negative margins + padding give hover lift and shadows room to
                    render, because overflow-x also clips vertically. */}
            <div
                ref={ref}
                onScroll={update}
                className="shelf -mx-8 -mt-3 -mb-8 flex snap-x snap-proximity scroll-px-8 gap-5 overflow-x-auto px-8 pt-3 pb-8"
            >
                {games.map((g, i) => (
                    <div key={g.id} className="w-40 shrink-0 snap-start">
                        <GameCard
                            game={g}
                            index={i}
                            favorite={favorites.has(g.externalId)}
                            onSelect={onSelect}
                            onToggleFavorite={onToggleFavorite}
                        />
                    </div>
                ))}
            </div>
        </section>
    )
}

function Arrow({
    label,
    disabled,
    onClick,
    children,
}: {
    label: string
    disabled: boolean
    onClick: () => void
    children: React.ReactNode
}) {
    return (
        <button
            onClick={onClick}
            disabled={disabled}
            aria-label={label}
            className="grid size-7 place-items-center rounded-full bg-white/5 text-white/60 ring-1 ring-white/5 transition hover:bg-white/10 hover:text-white disabled:pointer-events-none disabled:opacity-30"
        >
            {children}
        </button>
    )
}