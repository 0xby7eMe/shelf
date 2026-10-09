import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import {
	EpicCancelInstall,
	EpicInstall,
	EpicSyncSaves,
	EpicUninstall,
	EpicUpdate,
	EpicVerify,
	GetFavorites,
	GetGames,
	GetNowPlaying,
	GogInstall,
	GogUninstall,
	Launch,
	ToggleFavorite,
	UbisoftInstall,
	UbisoftUninstall,
} from "../wailsjs/go/main/App"
import { WindowControls } from "@/components/window-controls"
import { library } from "../wailsjs/go/models"
import { GameCard } from "@/components/game-card"
import { GameSheet, type EpicActions } from "@/components/game-sheet"
import { Hero } from "@/components/hero"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"
import { EventsOn, WindowToggleMaximise } from "../wailsjs/runtime/runtime"
import { NowPlaying } from "@/components/now-playing"
import { Shelf } from "@/components/shelf"
import { Activity, Download, Gauge, RefreshCw, Search, Settings, Share2, Shuffle, Terminal, Trophy, Users } from "lucide-react"
import { AchievementsDialog } from "@/components/achievements-dialog"
import { ActivityDialog } from "@/components/activity-dialog"
import { FriendsDialog } from "@/components/friends-dialog"
import { ShareCardDialog } from "@/components/share-card-dialog"
import { useStats } from "@/lib/use-stats"
import { setIntegrationTab, type Integration } from "@/lib/integrations"
import { useEpic } from "@/lib/use-epic"
import { toast } from "@/lib/toast"
import { confirm } from "@/lib/confirm"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Toaster } from "@/components/toaster"
import { EpicGameSettings } from "@/components/epic-game-settings"
import { LogPanel } from "@/components/log-panel"
import { PadHints } from "@/components/pad-hints"
import { FilterMenu, MoreMenu, headerButton, type MoreItem, type SortKey, type SourceFilter } from "@/components/header-menus"
import { MonitorPage } from "@/components/monitor/monitor-page"
import { UbisoftHint } from "@/components/ubisoft-hint"
import { GogHint } from "@/components/gog-hint"
import { GroupMenu } from "@/components/group-menu"
import { groupLabel, groupMembers, useOrganizer, type Group } from "@/lib/organizer"
import { setLogOpen, startLogs, useLogOpen } from "@/lib/logs"
import { usePrefs } from "@/lib/prefs"
import { SECTIONS, SettingsPage, type SettingsSection } from "@/components/settings/settings-page"
import { useGamepad } from "@/lib/gamepad"
import { useStableHover } from "@/lib/hover"

type Filter = "all" | "installed" | "favorites"

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
	const [shareOpen, setShareOpen] = useState(false)
	const [friendsOpen, setFriendsOpen] = useState(false)
	const [achievementsOpen, setAchievementsOpen] = useState(false)
	const [settings, setSettings] = useState<SettingsSection | null>(null)
	const settingsRef = useRef(settings)
	settingsRef.current = settings
	// The hardware monitor, a page of its own like settings.
	const [monitorOpen, setMonitorOpen] = useState(false)
	const monitorRef = useRef(monitorOpen)
	monitorRef.current = monitorOpen
	// Whether the monitor is switched on in Advanced settings; read by the key handler.
	const monitorEnabledRef = useRef(false)
	// Opens settings on one store's tab.
	const openIntegration = useCallback((tab: Integration) => {
		setIntegrationTab(tab)
		setSettings("integrations")
	}, [])
	const [settingsGame, setSettingsGame] = useState<library.Game | null>(null)
	const [source, setSource] = useState<SourceFilter>("all")
	const [group, setGroup] = useState<Group>(null)
	const organization = useOrganizer()
	const { account, installs, queue, reloadAccount } = useEpic(games, () => openIntegration("epic"))
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

	// A game started from a shelf:// link, a menu entry or the tray that couldn't start.
	useEffect(() => EventsOn("desktop:error", (message: string) => toast.error(message)), [])

	// A newer release is out: say so once, with a way to the page that handles it.
	useEffect(
		() =>
			EventsOn("update:available", (st: { latest?: string }) =>
				toast.info(`Shelf ${st.latest ?? ""} is available`, {
					description: "Open About to read what changed and update.",
					duration: 12000,
					action: { label: "Open", onClick: () => setSettings("about") },
				})
			),
		[]
	)

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
				(settingsRef.current || monitorRef.current) &&
				!document.querySelector('[role="dialog"], [role="listbox"]')
			) {
				setSettings(null)
				setMonitorOpen(false)
			} else if (e.key === "r" && !typing && !settingsRef.current && !monitorRef.current && !e.ctrlKey && !e.metaKey && !e.altKey) {
				e.preventDefault()
				randomRef.current()
			} else if (e.key === "p" && monitorEnabledRef.current && !typing && !settingsRef.current && !e.ctrlKey && !e.metaKey && !e.altKey) {
				// Not while typing in any field, such as a game's settings.
				const tag = (document.activeElement as HTMLElement | null)?.tagName
				if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return
				e.preventDefault()
				setMonitorOpen((open) => !open)
			}
		}
		window.addEventListener("keydown", onKey)
		return () => window.removeEventListener("keydown", onKey)
	}, [])

	// A group that no longer exists, such as a deleted collection, stops filtering.
	const activeGroup = group && groupLabel(organization, group) ? group : null
	const members = useMemo(() => groupMembers(organization, activeGroup), [organization, activeGroup])

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
			.filter((g) => !members || members.has(g.id))
			.filter((g) => !q || g.name.toLowerCase().includes(q))
			.sort(sorters[sort])
	}, [games, query, filter, source, sort, favorites, members])

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
	monitorEnabledRef.current = prefs.hardwareMonitor
	// Switching the monitor off closes it.
	useEffect(() => {
		if (!prefs.hardwareMonitor) setMonitorOpen(false)
	}, [prefs.hardwareMonitor])
	const logOpen = useLogOpen()
	useEffect(() => startLogs(), [])

	const filters: Filter[] = ["all", "installed", "favorites"]
	useGamepad({
		onBack: () => {
			setSettings(null)
			setMonitorOpen(false)
		},
		onTab: (dir) => {
			if (settingsRef.current) {
				const i = SECTIONS.findIndex((s) => s.id === settingsRef.current)
				setSettings(SECTIONS[(i + dir + SECTIONS.length) % SECTIONS.length].id)
			} else {
				setFilter((f) => filters[(filters.indexOf(f) + dir + filters.length) % filters.length])
			}
		},
		onMenu: () => setSettings((s) => (s ? null : "integrations")),
		onRandom: () => !settingsRef.current && !monitorRef.current && randomRef.current(),
		onSearch: () => !settingsRef.current && !monitorRef.current && searchRef.current?.focus(),
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

	const heroBanner = prefs.heroBanner
	const shelves = useMemo(() => {
		// The game in the banner isn't repeated on the shelves; without a banner it is just a game.
		const rest = (games ?? []).filter((g) => g.installed && !(heroBanner && g.id === featured?.id))

		const recent = rest
		.filter((g) => g.lastPlayed > 0)
		.sort((a, b) => b.lastPlayed - a.lastPlayed)
		.slice(0, 10)

		const unplayedAll = rest.filter((g) => g.lastPlayed === 0 && g.playtimeMinutes === 0)
		const unplayed = [...unplayedAll].sort(sorters.name).slice(0, 15)

		return { recent, unplayed, unplayedTotal: unplayedAll.length }
	}, [games, featured, heroBanner])

	const totalMinutes = useMemo(
		() => (games ?? []).reduce((n, g) => n + g.playtimeMinutes, 0),
		[games]
	)

	// The home view: no search and no collection narrowing the library down.
	const homeView = !!featured && !query.trim() && !activeGroup
	const showHero = homeView && prefs.heroBanner
	const showRecent = homeView && filter === "all" && prefs.shelfRecent && shelves.recent.length > 0
	const showUnplayed = homeView && filter === "all" && prefs.shelfUnplayed && shelves.unplayed.length > 0
	const showShelves = showRecent || showUnplayed

	function play(game: library.Game) {
		// Epic, GOG and Ubisoft games have to be installed through Shelf first.
		if ((game.source === "epic" || game.source === "gog" || game.source === "ubisoft") && !game.installed) {
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
		install: (g) => epicTask(g, "install", () => (g.source === "gog" ? GogInstall(g.externalId) : EpicInstall(g.externalId))),
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
			const ubisoft = g.source === "ubisoft"
			const ok = await confirm({
				title: "Uninstall this game?",
				description: ubisoft
					? "Ubisoft Connect removes the game files and asks you to confirm in its own window. Your saves and your Ubisoft library are kept."
					: g.source === "gog"
						? "The game's folder is deleted from this PC, including any saves the game keeps there. Saves in its Proton prefix are kept, and you can install it again any time."
						: "The game files are removed from this PC. Your saves and Proton prefix are kept, and you can install it again any time.",
				confirmLabel: "Uninstall",
				destructive: true,
				game: { name: g.name, cover: g.cover },
			})
			if (!ok) return
			if (ubisoft) {
				UbisoftUninstall(g.externalId)
					.then(() =>
						toast.info(`Uninstalling ${g.name}`, { description: "Confirm in Ubisoft Connect. Shelf updates when it's done." })
					)
					.catch((e: any) => toast.error(`Couldn't uninstall ${g.name}`, { description: String(e) }))
				return
			}
			;(g.source === "gog" ? GogUninstall(g.externalId) : EpicUninstall(g.externalId))
				.then(() => toast.success(`${g.name} uninstalled`))
				.catch((e: any) => toast.error(`Couldn't uninstall ${g.name}`, { description: String(e) }))
		},
	}

	// What used to be a row of icons: used now and then, so kept out of the bar.
	const moreItems: MoreItem[] = [
		{ id: "random", label: "Random game", icon: <Shuffle className="size-3.5" />, hint: "R", onSelect: pickRandom },
		{
			id: "rescan",
			label: "Rescan library",
			icon: <RefreshCw className={cn("size-3.5", refreshing && "animate-spin")} />,
			onSelect: () => refresh(),
		},
		{ id: "activity", label: "Activity", icon: <Activity className="size-3.5" />, onSelect: () => setActivityOpen(true) },
		{ id: "friends", label: "Friends", icon: <Users className="size-3.5" />, onSelect: () => setFriendsOpen(true) },
		{ id: "achievements", label: "Achievements", icon: <Trophy className="size-3.5" />, onSelect: () => setAchievementsOpen(true) },
		{ id: "share", label: "Share card", icon: <Share2 className="size-3.5" />, onSelect: () => setShareOpen(true) },
		...(prefs.hardwareMonitor
			? [{ id: "performance", label: "Performance", icon: <Gauge className="size-3.5" />, hint: "P", onSelect: () => setMonitorOpen(true) }]
			: []),
		...(prefs.logWindow
			? [
					{
						id: "log",
						label: logOpen ? "Hide log window" : "Show log window",
						icon: <Terminal className="size-3.5" />,
						checked: logOpen,
						onSelect: () => setLogOpen(!logOpen),
					},
				]
			: []),
	]

	return (
		<>
		{monitorOpen && prefs.hardwareMonitor && !settings && <MonitorPage onBack={() => setMonitorOpen(false)} />}
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
				(settings || (monitorOpen && prefs.hardwareMonitor)) && "invisible"
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

				<div className="relative min-w-24 max-w-md flex-1">
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

				<FilterMenu source={source} onSource={setSource} sort={sort} onSort={setSort} />

				<GroupMenu group={activeGroup} onGroup={setGroup} />

				<button
					onClick={() => setSettings("integrations")}
					title="Settings"
					aria-label="Settings"
					className={headerButton}
				>
					<Settings className="size-3.5" />
				</button>

				<MoreMenu items={moreItems} />

				{/* What is going on right now sits at the far side. */}
				<div className="ml-auto flex items-center gap-3">
					{nowPlaying && (
						<NowPlaying game={nowPlaying.game} since={nowPlaying.since} onSelect={setSelected} />
					)}

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
				</div>

				<WindowControls />
			</header>

			{showHero && featured && (
				<Hero key={featured.id} game={featured} onPlay={play} onDetails={setSelected} />
			)}

			<main className={cn("relative px-8 pb-16", showHero ? "pt-8" : "pt-6")}>
				{showShelves && games && (
					<>
						{showRecent && (
						<Shelf
							title="Continue playing"
							games={shelves.recent}
							favorites={favorites}
							onSelect={setSelected}
							onToggleFavorite={toggleFavorite}
							playing={playingIds}
						/>
						)}
						{showUnplayed && (
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
				) : visible.length === 0 && source === "ubisoft" && !games.some((g) => g.source === "ubisoft") ? (
					<UbisoftHint onSettings={() => openIntegration("ubisoft")} />
				) : visible.length === 0 && source === "gog" && !games.some((g) => g.source === "gog") ? (
					<GogHint onSettings={() => openIntegration("gog")} />
				) : visible.length === 0 ? (
					<p className="pt-24 text-center text-sm text-muted-foreground">
						{games.length === 0
							? "No games found."
							: activeGroup && !query
								? `Nothing in ${groupLabel(organization, activeGroup)} matches these filters.`
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

			<FriendsDialog
				open={friendsOpen}
				onOpenChange={setFriendsOpen}
				games={games ?? NO_GAMES}
				onSelect={setSelected}
				onSetup={(source) => openIntegration(source as Integration)}
			/>

			<AchievementsDialog
				open={achievementsOpen}
				onOpenChange={setAchievementsOpen}
				games={games ?? NO_GAMES}
				onSelect={setSelected}
				onSetup={(source) => openIntegration(source as Integration)}
			/>

			<ShareCardDialog
				open={shareOpen}
				onOpenChange={setShareOpen}
				games={games ?? NO_GAMES}
				favorites={favorites}
			/>
		</div>

		{prefs.logWindow && <LogPanel games={games ?? NO_GAMES} />}
		<ConfirmDialog />
		<Toaster />
		<PadHints inSettings={settings !== null || (monitorOpen && prefs.hardwareMonitor)} />
		</>
	)
}

function Grid({ children }: { children: React.ReactNode }) {
	return (
		<div className="grid grid-cols-[repeat(auto-fill,minmax(var(--poster-min),1fr))] gap-6">
			{children}
		</div>
	)
}

export default App