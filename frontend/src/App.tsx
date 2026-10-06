import { useEffect, useMemo, useRef, useState } from "react"
import { Search, Shuffle } from "lucide-react"
import { GetFavorites, GetGames, Launch, ToggleFavorite } from "../wailsjs/go/main/App"
import { WindowControls } from "@/components/window-controls"
import { library } from "../wailsjs/go/models"
import { GameCard } from "@/components/game-card"
import { GameSheet } from "@/components/game-sheet"
import { Hero } from "@/components/hero"
import { Input } from "@/components/ui/input"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"
import { WindowToggleMaximise } from "../wailsjs/runtime/runtime"

type SortKey = "name" | "playtime" | "recent"
type Filter = "all" | "installed" | "favorites"

const sorters: Record<SortKey, (a: library.Game, b: library.Game) => number> = {
    name: (a, b) => a.name.localeCompare(b.name),
    playtime: (a, b) => b.playtimeMinutes - a.playtimeMinutes,
    recent: (a, b) => b.lastPlayed - a.lastPlayed,
}

function App() {
    const [games, setGames] = useState<library.Game[] | null>(null)
    const [error, setError] = useState("")
    const [query, setQuery] = useState("")
    const [filter, setFilter] = useState<Filter>("all")
    const [sort, setSort] = useState<SortKey>("name")
    const [selected, setSelected] = useState<library.Game | null>(null)
    const [scrolled, setScrolled] = useState(false)
    const [favorites, setFavorites] = useState<Set<string>>(new Set())
    const randomRef = useRef<() => void>(() => {})
    const searchRef = useRef<HTMLInputElement>(null)

    useEffect(() => {
        GetGames()
            .then(setGames)
            .catch((e) => setError(String(e)))
    }, [])

    useEffect(() => {
        GetFavorites()
            .then((ids) => setFavorites(new Set(ids)))
            .catch((e) => setError(String(e)))
    }, [])

    useEffect(() => {
        function onKey(e: KeyboardEvent) {
            const typing = document.activeElement === searchRef.current
            if (e.key === "/" && !typing) {
                e.preventDefault()
                searchRef.current?.focus()
            } else if (e.key === "Escape" && typing) {
                setQuery("")
                searchRef.current?.blur()
            } else if (e.key === "r" && !typing && !e.ctrlKey && !e.metaKey && !e.altKey) {
                e.preventDefault()
                randomRef.current()
            }
        }
        window.addEventListener("keydown", onKey)
        return () => window.removeEventListener("keydown", onKey)
    }, [])

    const visible = useMemo(() => {
        if (!games) return []
        const q = query.trim().toLowerCase()
        return games
            .filter((g) =>
                filter === "installed"
                    ? g.installed
                    : filter === "favorites"
                        ? favorites.has(g.externalId)
                        : true
            )
            .filter((g) => !q || g.name.toLowerCase().includes(q))
            .sort(sorters[sort])
    }, [games, query, filter, sort, favorites])

    function toggleFavorite(game: library.Game) {
        const id = game.externalId
        setFavorites((prev) => {
            const next = new Set(prev)
            if (next.has(id)) next.delete(id)
            else next.add(id)
            return next
        })
        ToggleFavorite(id)
            .then((ids) => setFavorites(new Set(ids)))
            .catch((e) => {
                setError(String(e))
                GetFavorites().then((ids) => setFavorites(new Set(ids)))
            })
    }

    function pickRandom() {
        const installed = visible.filter((g) => g.installed)
        const pool = installed.length ? installed : visible
        const options = pool.length > 1 ? pool.filter((g) => g.id !== selected?.id) : pool
        if (options.length === 0) return
        setSelected(options[Math.floor(Math.random() * options.length)])
    }

    useEffect(() => {
        randomRef.current = pickRandom
    })

    const featured = useMemo(() => {
        if (!games || games.length === 0) return null
        const installed = games.filter((g) => g.installed)
        const pool = installed.length ? installed : games
        return [...pool].sort(
            (a, b) => b.lastPlayed - a.lastPlayed || b.playtimeMinutes - a.playtimeMinutes
        )[0]
    }, [games])

    const totalMinutes = useMemo(
        () => (games ?? []).reduce((n, g) => n + g.playtimeMinutes, 0),
        [games]
    )

    const showHero = !!featured && !query.trim()

    function play(game: library.Game) {
        Launch(game.externalId).catch((e: any) => setError(String(e)))
    }

    return (
        <div
            onScroll={(e) => setScrolled(e.currentTarget.scrollTop > 24)}
            className="relative h-screen overflow-y-auto bg-background text-foreground"
        >
            <header
                onDoubleClick={(e) => {
                    if (e.target === e.currentTarget) WindowToggleMaximise()
                }}
                style={{
                    backdropFilter: scrolled ? "blur(20px) saturate(140%)" : "none",
                    WebkitBackdropFilter: scrolled ? "blur(20px) saturate(140%)" : "none",
                    backgroundColor: scrolled ? "rgba(10, 10, 10, 0.55)" : "transparent",
                }}
                className={cn(
                    "titlebar sticky top-0 z-30 flex h-16 items-center gap-3 border-b px-8 transition-colors duration-300",
                    scrolled ? "border-white/5" : "border-transparent"
                )}
            >
                <h1 className="mr-2 text-sm font-medium tracking-[0.25em] uppercase">Shelf</h1>

                <div className="relative max-w-md flex-1">
                    <Search className="absolute top-1/2 left-3.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        ref={searchRef}
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                        placeholder="Search"
                        className="h-9 rounded-full border-0 bg-white/5 pl-9 text-sm shadow-none ring-1 ring-white/5 backdrop-blur-md focus-visible:bg-white/10 focus-visible:ring-white/20"
                    />
                    {!query && (
                        <kbd className="absolute top-1/2 right-3 -translate-y-1/2 rounded border border-white/10 px-1.5 text-[10px] text-muted-foreground">
                            /
                        </kbd>
                    )}
                </div>

                <div className="flex rounded-full bg-white/5 p-1 ring-1 ring-white/5 backdrop-blur-md">
                    {(["all", "installed", "favorites"] as const).map((f) => (
                        <button
                            key={f}
                            onClick={() => setFilter(f)}
                            className={cn(
                                "rounded-full px-3 py-1 text-xs capitalize transition",
                                filter === f
                                    ? "bg-white/10 text-foreground"
                                    : "text-muted-foreground hover:text-foreground"
                            )}
                        >
                            {f}
                        </button>
                    ))}
                </div>

                <Select value={sort} onValueChange={(v) => setSort(v as SortKey)}>
                    <SelectTrigger className="h-9 w-36 rounded-full border-0 bg-white/5 text-xs shadow-none ring-1 ring-white/5 backdrop-blur-md">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent className="border-white/10 bg-popover/80 backdrop-blur-xl">
                        <SelectItem value="name">Name</SelectItem>
                        <SelectItem value="playtime">Most played</SelectItem>
                        <SelectItem value="recent">Recently played</SelectItem>
                    </SelectContent>
                </Select>

                <button
                    onClick={pickRandom}
                    title="Random game (R)"
                    aria-label="Random game"
                    className="grid size-9 place-items-center rounded-full bg-white/5 text-white/70 ring-1 ring-white/5 backdrop-blur-md transition hover:bg-white/10 hover:text-white"
                >
                    <Shuffle className="size-3.5" />
                </button>

                <WindowControls />
            </header>

            {showHero && featured && (
                <Hero key={featured.id} game={featured} onPlay={play} onDetails={setSelected} />
            )}

            <main className={cn("relative px-8 pb-16", showHero ? "pt-8" : "pt-6")}>
                <div className="mb-5 flex items-baseline justify-between">
                    <h2 className="text-[11px] font-medium tracking-[0.25em] text-muted-foreground uppercase">
                        Library
                    </h2>
                    {games && (
                        <span className="text-xs tabular-nums text-muted-foreground/70">
                            {visible.length} games
                        </span>
                    )}
                </div>

                {error ? (
                    <p className="text-sm text-destructive">{error}</p>
                ) : games === null ? (
                    <Grid>
                        {Array.from({ length: 18 }, (_, i) => (
                            <Skeleton key={i} className="aspect-[2/3] rounded-xl bg-white/5" />
                        ))}
                    </Grid>
                ) : visible.length === 0 ? (
                    <p className="pt-24 text-center text-sm text-muted-foreground">
                        {games.length === 0
                            ? "No Steam games found."
                            : filter === "favorites" && !query
                                ? "No favorites yet. Click the heart on a poster."
                                : "Nothing matches."
                        }
                    </p>
                ) : (
                    <Grid>
                        {visible.map((g, i) => (
                            <GameCard
                                key={g.id}
                                game={g}
                                index={i}
                                favorite={favorites.has(g.externalId)}
                                onSelect={setSelected}
                                onToggleFavorite={toggleFavorite}
                            />
                        ))}
                    </Grid>
                )}
            </main>

            <GameSheet
                game={selected}
                totalMinutes={totalMinutes}
                favorite={selected ? favorites.has(selected.externalId) : false}
                onToggleFavorite={toggleFavorite}
                onClose={() => setSelected(null)}
                onPlay={play}
            />
        </div>
    )
}

function Grid({ children }: { children: React.ReactNode }) {
    return (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-6">
            {children}
        </div>
    )
}

export default App