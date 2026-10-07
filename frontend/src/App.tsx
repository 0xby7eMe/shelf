import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import {
	EpicCancelInstall,
	EpicInstall,
	EpicSyncSaves,
	EpicUninstall,
	EpicUpdate,
	EpicVerify,
	UbisoftInstall,
	GetFavorites,
	GetGames,
	GetNowPlaying,
	Launch,
	ToggleFavorite,
} from "../wailsjs/go/main/App"
import { WindowControls } from "@/components/window-controls"
import { library } from "../wailsjs/go/models"
import { GameCard } from "@/components/game-card"
import { GameSheet, type EpicActions } from "@/components/game-sheet"
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
import { EventsOn, WindowToggleMaximise } from "../wailsjs/runtime/runtime"
import { NowPlaying } from "@/components/now-playing"
import { Shelf } from "@/components/shelf"
import { Activity, Download, RefreshCw, Search, Settings, Shuffle, Terminal } from "lucide-react"
import { ActivityDialog } from "@/components/activity-dialog"
import { useStats } from "@/lib/use-stats"
import { useEpic } from "@/lib/use-epic"
import { toast } from "@/lib/toast"
import { confirm } from "@/lib/confirm"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Toaster } from "@/components/toaster"
import { EpicGameSettings } from "@/components/epic-game-settings"
import { LogPanel } from "@/components/log-panel"
import { PadHints } from "@/components/pad-hints"
import { setLogOpen, startLogs, useLogOpen } from "@/lib/logs"
import { usePrefs } from "@/lib/prefs"
import { SECTIONS, SettingsPage, type SettingsSection } from "@/components/settings/settings-page"
import { useGamepad } from "@/lib/gamepad"
import { useStableHover } from "@/lib/hover"

type SortKey = "name" | "playtime" | "recent"
type Filter = "all" | "installed" | "favorites"
type SourceFilter = "all" | "steam" | "epic" | "ubisoft"

const sorters: Record<SortKey, (a: library.Game, b: library.Game) => number> = {
	name: (a, b) => a.name.localeCompare(b.name),
	playtime: (a, b) => b.playtimeMinutes - a.playtimeMinutes,
	recent: (a, b) => b.lastPlayed - a.lastPlayed,
}

const NO_GAMES: library.Game[] = []

function App() {
	const [games, setGames] = useState<library.Game[] | null>(null)
	const [error, setError] = useState("")
	const [query, setQuery] = useState("")
	const [filter, setFilter] = useState<Filter>("all")
	const [sort, setSort] = useState<SortKey>("name")
	const [selected, setSelected] = useState<library.Game | null>(null)
	const [scrolled, setScrolled] = useState(false)
	const [favorites, setFavorites] = useState<Set<string>>(new Set())
	const [refreshing, setRefreshing] = useState(false)
	const [playing, setPlaying] = useState<library.Session[]>([])
	const [activityOpen, setActivityOpen] = useState(false)
	const [settings, setSettings] = useState<SettingsSection | null>(null)
	const settingsRef = useRef(settings)
	settingsRef.current = settings
	const [settingsGame, setSettingsGame] = useState<library.Game | null>(null)
	const [source, setSource] = useState<SourceFilter>("all")
	const { account, installs, queue, reloadAccount } = useEpic(games, () => setSettings("epic"))
  	const stats = useStats()
	const reqRef = useRef(0)
	const randomRef = useRef<() => void>(() => {})
	const searchRef = useRef<HTMLInputElement>(null)

	const refresh = useCallback(async () => {
		const id = ++reqRef.current
		const started = Date.now()
		setRefreshing(true)
		try {
		const next = await GetGames()
		if (id !== reqRef.current) return
		setGames(next)
		setError("")
		} catch (e) {
		if (id === reqRef.current) setError(String(e))
		} finally {
		const wait = Math.max(0, 600 - (Date.now() - started))
		setTimeout(() => {
			if (id === reqRef.current) setRefreshing(false)
		}, wait)
		}
	}, [])

	useEffect(() => {
		refresh()
		return EventsOn("library:changed", () => {
		refresh()
		})
	}, [refresh])

	useEffect(() => {
		if (!games) return
		setSelected((prev) => (prev ? (games.find((g) => g.id === prev.id) ?? null) : prev))
	}, [games])

	useEffect(() => {
		GetNowPlaying().then((s) => setPlaying(s ?? []))
		return EventsOn("nowplaying:changed", (s: library.Session[]) => setPlaying(s ?? []))
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
			} else if (
				e.key === "Escape" &&
				settingsRef.current &&
				!document.querySelector('[role="dialog"], [role="listbox"]')
			) {
				setSettings(null)
			} else if (e.key === "r" && !typing && !settingsRef.current && !e.ctrlKey && !e.metaKey && !e.altKey) {
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
			.filter((g) => source === "all" || g.source === source)
			.filter((g) => !q || g.name.toLowerCase().includes(q))
			.sort(sorters[sort])
	}, [games, query, filter, source, sort, favorites])

	// Stable identity, so memoized cards don't re-render when something unrelated changes.
	const toggleFavorite = useCallback((game: library.Game) => {
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
	}, [])

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

	useStableHover()

	const prefs = usePrefs()
	const logOpen = useLogOpen()
	useEffect(() => startLogs(), [])

	const filters: Filter[] = ["all", "installed", "favorites"]
	useGamepad({
		onBack: () => setSettings(null),
		onTab: (dir) => {
			if (settingsRef.current) {
				const i = SECTIONS.findIndex((s) => s.id === settingsRef.current)
				setSettings(SECTIONS[(i + dir + SECTIONS.length) % SECTIONS.length].id)
			} else {
				setFilter((f) => filters[(filters.indexOf(f) + dir + filters.length) % filters.length])
			}
		},
		onMenu: () => setSettings((s) => (s ? null : "epic")),
		onRandom: () => !settingsRef.current && randomRef.current(),
		onSearch: () => !settingsRef.current && searchRef.current?.focus(),
	})

	const featured = useMemo(() => {
		if (!games || games.length === 0) return null
		const installed = games.filter((g) => g.installed)
		const pool = installed.length ? installed : games
		return [...pool].sort(
			(a, b) => b.lastPlayed - a.lastPlayed || b.playtimeMinutes - a.playtimeMinutes
		)[0]
	}, [games])

	const playingIds = useMemo(() => new Set(playing.map((s) => s.appId)), [playing])

	const nowPlaying = useMemo(() => {
		for (const s of playing) {
		const game = games?.find((g) => g.externalId === s.appId)
		if (game) return { game, since: s.since }
		}
		return null
	}, [playing, games])

	const shelves = useMemo(() => {
		const rest = (games ?? []).filter((g) => g.installed && g.id !== featured?.id)

		const recent = rest
		.filter((g) => g.lastPlayed > 0)
		.sort((a, b) => b.lastPlayed - a.lastPlayed)
		.slice(0, 10)

		const unplayedAll = rest.filter((g) => g.lastPlayed === 0 && g.playtimeMinutes === 0)
		const unplayed = [...unplayedAll].sort(sorters.name).slice(0, 15)

		return { recent, unplayed, unplayedTotal: unplayedAll.length }
	}, [games, featured])

	const totalMinutes = useMemo(
		() => (games ?? []).reduce((n, g) => n + g.playtimeMinutes, 0),
		[games]
	)

	const showHero = !!featured && !query.trim()
	const showShelves = showHero && filter === "all"

	function play(game: library.Game) {
		// Epic and Ubisoft games have to be installed through Shelf first.
		if ((game.source === "epic" || game.source === "ubisoft") && !game.installed) {
			setSelected(game)
			return
		}
		Launch(game.id).catch((e: any) => toast.error(`Couldn't start ${game.name}`, { description: String(e) }))
	}

	// Run an Epic action, turning a refusal into a toast.
	function epicTask(game: library.Game, what: string, task: () => Promise<unknown>) {
		task().catch((e: any) => toast.error(`Couldn't ${what} ${game.name}`, { description: String(e) }))
	}

	const epicActions: EpicActions = {
		install: (g) => epicTask(g, "install", () => EpicInstall(g.externalId)),
		installUbisoft: (g) =>
			epicTask(g, "install", async () => {
				await UbisoftInstall(g.externalId)
				toast.info(`Installing ${g.name}`, { description: "Ubisoft Connect is downloading it. Shelf will notice when it's done." })
			}),
		cancel: (g) => EpicCancelInstall(g.externalId),
		update: (g) => epicTask(g, "update", () => EpicUpdate(g.externalId)),
		verify: (g) => epicTask(g, "verify", () => EpicVerify(g.externalId)),
		syncSaves: (g) =>
			epicTask(g, "sync saves for", async () => {
				await EpicSyncSaves(g.externalId)
			}),
		openSettings: setSettingsGame,
		uninstall: async (g) => {
			const ok = await confirm({
				title: "Uninstall this game?",
				description:
					"The game files are removed from this PC. Your saves and Proton prefix are kept, and you can install it again any time.",
				confirmLabel: "Uninstall",
				destructive: true,
				game: { name: g.name, cover: g.cover },
			})
			if (!ok) return
			EpicUninstall(g.externalId)
				.then(() => toast.success(`${g.name} uninstalled`))
				.catch((e: any) => toast.error(`Couldn't uninstall ${g.name}`, { description: String(e) }))
		},
	}

	return (
		<>
		{settings && (
			<SettingsPage
				section={settings}
				onSection={setSettings}
				onBack={() => setSettings(null)}
				account={account}
				onAccountChange={reloadAccount}
				queue={queue}
				games={games ?? NO_GAMES}
				onSelectGame={(g) => {
					setSettings(null)
					setSelected(g)
				}}
			/>
		)}
		<div
			data-scroll-root
			onScroll={(e) => {
				setScrolled(e.currentTarget.scrollTop > 24)
			}}
			className={cn(
				"relative h-screen overflow-y-auto bg-background text-foreground",
				// Kept mounted while settings are open so the scroll position survives.
				settings && "invisible"
			)}
		>
			<header
				onDoubleClick={(e) => {
					if (e.target === e.currentTarget) WindowToggleMaximise()
				}}
				style={{
					backdropFilter: scrolled ? "blur(14px)" : "none",
					WebkitBackdropFilter: scrolled ? "blur(14px)" : "none",
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

				<button
					onClick={() => refresh()}
					title="Rescan library"
					aria-label="Rescan library"
					className="grid size-9 place-items-center rounded-full bg-white/5 text-white/70 ring-1 ring-white/5 backdrop-blur-md transition hover:bg-white/10 hover:text-white"
				>
					<RefreshCw className={cn("size-3.5", refreshing && "animate-spin")} />
				</button>

				<Select value={source} onValueChange={(v) => setSource(v as SourceFilter)}>
					<SelectTrigger className="h-9 w-28 rounded-full border-0 bg-white/5 text-xs shadow-none ring-1 ring-white/5 backdrop-blur-md">
						<SelectValue />
					</SelectTrigger>
					<SelectContent className="border-white/10 bg-popover/80 backdrop-blur-xl">
						<SelectItem value="all">All stores</SelectItem>
						<SelectItem value="steam">Steam</SelectItem>
						<SelectItem value="epic">Epic</SelectItem>
						<SelectItem value="ubisoft">Ubisoft</SelectItem>
					</SelectContent>
				</Select>

				{queue.jobs.length > 0 && (
					<button
						onClick={() => setSettings("downloads")}
						title="Downloads"
						aria-label="Downloads"
						className="flex h-9 items-center gap-2 rounded-full bg-white/10 px-3 text-xs tabular-nums text-white ring-1 ring-white/10 backdrop-blur-md transition hover:bg-white/15"
					>
						<Download className="size-3.5" />
						{queue.jobs.length}
						{queue.jobs[0].state === "installing" && queue.jobs[0].percent > 0 && (
							<span className="text-white/60">{Math.floor(queue.jobs[0].percent)}%</span>
						)}
					</button>
				)}

				{prefs.logWindow && (
					<button
						onClick={() => setLogOpen(!logOpen)}
						title="Log window"
						aria-label="Log window"
						aria-pressed={logOpen}
						className={cn(
							"grid size-9 place-items-center rounded-full ring-1 ring-white/5 backdrop-blur-md transition hover:bg-white/10 hover:text-white",
							logOpen ? "bg-white/15 text-white" : "bg-white/5 text-white/70"
						)}
					>
						<Terminal className="size-3.5" />
					</button>
				)}

				<button
					onClick={() => setSettings("epic")}
					title="Settings"
					aria-label="Settings"
					className="grid size-9 place-items-center rounded-full bg-white/5 text-white/70 ring-1 ring-white/5 backdrop-blur-md transition hover:bg-white/10 hover:text-white"
				>
					<Settings className="size-3.5" />
				</button>

				<button
					onClick={() => setActivityOpen(true)}
					title="Activity"
					aria-label="Activity"
					className="grid size-9 place-items-center rounded-full bg-white/5 text-white/70 ring-1 ring-white/5 backdrop-blur-md transition hover:bg-white/10 hover:text-white"
				>
					<Activity className="size-3.5" />
				</button>

				{nowPlaying && (
					<NowPlaying game={nowPlaying.game} since={nowPlaying.since} onSelect={setSelected} />
				)}

				<WindowControls />
			</header>

			{showHero && featured && (
				<Hero key={featured.id} game={featured} onPlay={play} onDetails={setSelected} />
			)}

			<main className={cn("relative px-8 pb-16", showHero ? "pt-8" : "pt-6")}>
				{showShelves && games && (
					<>
						{shelves.recent.length > 0 && (
						<Shelf
							title="Continue playing"
							games={shelves.recent}
							favorites={favorites}
							onSelect={setSelected}
							onToggleFavorite={toggleFavorite}
							playing={playingIds}
						/>
						)}
						{shelves.unplayed.length > 0 && (
						<Shelf
							title="Never played"
							count={shelves.unplayedTotal}
							games={shelves.unplayed}
							favorites={favorites}
							onSelect={setSelected}
							onToggleFavorite={toggleFavorite}
						/>
						)}
					</>
				)}

				<div className="mb-5 flex items-baseline justify-between">
					<h2 className="text-[11px] font-medium tracking-[0.25em] text-muted-foreground uppercase">
						{showShelves ? "All games" : "Library"}
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
							? "No games found."
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
								playing={playingIds.has(g.externalId)}
								job={installs[g.externalId]}
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
				running={selected ? playingIds.has(selected.externalId) : false}
				onPlay={play}
				activity={selected ? stats?.games.find((g) => g.appId === selected.externalId) : undefined}
				job={selected ? installs[selected.externalId] : undefined}
				epicActions={epicActions}
			/>

			<EpicGameSettings game={settingsGame} onClose={() => setSettingsGame(null)} />

			<ActivityDialog
				open={activityOpen}
				onOpenChange={setActivityOpen}
				stats={stats}
				games={games ?? NO_GAMES}
				onSelect={setSelected}
			/>
		</div>

		{prefs.logWindow && <LogPanel games={games ?? NO_GAMES} />}
		<ConfirmDialog />
		<Toaster />
		<PadHints inSettings={settings !== null} />
		</>
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