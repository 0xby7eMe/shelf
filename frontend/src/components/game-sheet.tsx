import { useEffect, useState } from "react"
import {
	ArrowUpCircle,
	CloudUpload,
	Download,
	ExternalLink,
	FolderOpen,
	Heart,
	Play,
	ShieldCheck,
	SlidersHorizontal,
	Trash2,
	X,
} from "lucide-react"

import { EpicLaunchInfo, OpenInstallFolder, OpenStorePage } from "../../wailsjs/go/main/App"
import { epic, library } from "../../wailsjs/go/models"
import {
	Sheet,
	SheetContent,
	SheetDescription,
	SheetTitle,
} from "@/components/ui/sheet"
import { formatDuration, formatLastPlayed, formatPlaytime, relativeTime } from "@/lib/format"
import { heroSrc } from "@/lib/cover"
import { jobLabel } from "@/lib/use-epic"
import { cn } from "@/lib/utils"
import { LiveDot } from "@/components/live-dot"

// What the sheet can ask the app to do with an Epic game.
export interface EpicActions {
	install: (game: library.Game) => void
	cancel: (game: library.Game) => void
	uninstall: (game: library.Game) => void
	update: (game: library.Game) => void
	verify: (game: library.Game) => void
	syncSaves: (game: library.Game) => void
	openSettings: (game: library.Game) => void
}

interface Props {
	game: library.Game | null
	totalMinutes: number
	favorite: boolean
	onToggleFavorite: (game: library.Game) => void
	onClose: () => void
	onPlay: (game: library.Game) => void
	running: boolean
	activity?: library.GameStat
	job?: epic.Progress
	epicActions: EpicActions
}

export function GameSheet({
	game,
	totalMinutes,
	favorite,
	onToggleFavorite,
	onClose,
	onPlay,
	running,
	activity,
	job,
	epicActions,
}: Props) {
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
					<Body
						key={game.id}
						game={game}
						totalMinutes={totalMinutes}
						favorite={favorite}
						onToggleFavorite={onToggleFavorite}
						onPlay={onPlay}
						running={running}
						activity={activity}
						job={job}
						epicActions={epicActions}
					/>
				)}
			</SheetContent>
		</Sheet>
	)
}

function Reveal({
	i,
	className,
	children,
}: {
	i: number
	className?: string
	children: React.ReactNode
}) {
	return (
		<div
			style={{ animationDelay: `${120 + i * 70}ms` }}
			className={cn(
				"animate-in duration-500 [animation-fill-mode:backwards] fade-in slide-in-from-bottom-3 motion-reduce:animate-none",
				className
			)}
		>
			{children}
		</div>
	)
}

function Body({
	game,
	totalMinutes,
	favorite,
	onToggleFavorite,
	onPlay,
	running,
	activity,
	job,
	epicActions,
}: {
	game: library.Game
	totalMinutes: number
	favorite: boolean
	onToggleFavorite: (game: library.Game) => void
	onPlay: (game: library.Game) => void
	running: boolean
	activity?: library.GameStat
	job?: epic.Progress
	epicActions: EpicActions
}) {
	const [heroFailed, setHeroFailed] = useState(false)
	const [armed, setArmed] = useState(false)
	const isEpic = game.source === "epic"
	const [proton, setProton] = useState("")
	const share = totalMinutes > 0 ? (game.playtimeMinutes / totalMinutes) * 100 : 0

	// The share bar fills once the content above it has appeared.
	useEffect(() => {
		const t = setTimeout(() => setArmed(true), 600)
		return () => clearTimeout(t)
	}, [])

	useEffect(() => {
		if (!isEpic || !game.installed) return
		EpicLaunchInfo(game.externalId)
			.then((i) => setProton(i.proton))
			.catch(() => setProton(""))
	}, [isEpic, game.installed, game.externalId])

	return (
		<>
			{/* Banner */}
			<div className="relative h-52 shrink-0 overflow-hidden">
				{!heroFailed ? (
					<img
						src={heroSrc(game)}
						alt=""
						onError={() => setHeroFailed(true)}
						className="size-full animate-in object-cover duration-1000 ease-out fade-in zoom-in-110 motion-reduce:animate-none"
					/>
				) : (
					<img
						src={game.cover}
						alt=""
						aria-hidden
						className="size-full scale-125 animate-in object-cover opacity-60 blur-3xl saturate-150 duration-700 fade-in"
					/>
				)}
				<div className="absolute inset-0 bg-gradient-to-t from-[rgba(10,10,10,0.95)] via-[rgba(10,10,10,0.3)] to-transparent" />
			</div>

			{/* Poster overlapping the banner, title beside it */}
			<Reveal i={1} className="relative -mt-20 flex items-end gap-4 px-6">
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
					<SheetDescription className="sr-only">Details for {game.name}</SheetDescription>
				</div>
			</Reveal>

			<div className="space-y-6 px-6 pt-6 pb-8">
				<Reveal i={2}>
					{job ? (
						<div className="flex items-center gap-2">
							<div className="relative h-11 flex-1 overflow-hidden rounded-full bg-white/10 ring-1 ring-white/10">
								<div
									className="absolute inset-y-0 left-0 bg-white/25 transition-[width] duration-300"
									style={{ width: `${job.percent}%` }}
								/>
								<span className="relative flex h-full items-center justify-center gap-2 text-sm tabular-nums">
									{jobLabel(job)}
									{job.state === "installing" && job.percent > 0 && ` ${Math.floor(job.percent)}%`}
									{job.speed && <span className="text-white/50">{job.speed}</span>}
									{job.eta && <span className="text-white/50">ETA {job.eta}</span>}
								</span>
							</div>
							<button
								onClick={() => epicActions.cancel(game)}
								aria-label="Cancel"
								title="Cancel"
								className="grid size-11 place-items-center rounded-full bg-white/5 text-white/70 ring-1 ring-white/10 transition hover:bg-white/10 hover:text-white"
							>
								<X className="size-4" />
							</button>
						</div>
					) : (
						<div className="flex gap-2">
							<button
								onClick={() => (isEpic && !game.installed ? epicActions.install(game) : onPlay(game))}
								disabled={running}
								className={cn(
									"flex h-11 min-w-0 flex-1 items-center justify-center gap-2 rounded-full text-sm font-medium transition duration-300",
									running
										? "bg-white/10 text-white/80 ring-1 ring-white/10"
										: "bg-white text-black hover:shadow-[0_0_40px_rgba(255,255,255,0.3)]"
								)}
							>
								{running ? (
									<>
										<LiveDot />
										Running
									</>
								) : isEpic && !game.installed ? (
									<>
										<Download className="size-3.5" />
										Install
									</>
								) : (
									<>
										<Play className="size-3.5 fill-current" />
										{game.installed ? "Play" : "Install in Steam"}
									</>
								)}
							</button>
							{isEpic && game.installed && game.updateAvailable && (
								<button
									onClick={() => epicActions.update(game)}
									disabled={running}
									className="flex h-11 items-center gap-2 rounded-full bg-white/10 px-5 text-sm font-medium ring-1 ring-white/15 transition hover:bg-white/15 disabled:pointer-events-none disabled:opacity-40"
								>
									<ArrowUpCircle className="size-4" />
									Update
								</button>
							)}
						</div>
					)}
				</Reveal>

				<Reveal i={3} className="grid grid-cols-2 gap-3">
					<Stat label="Playtime" value={formatPlaytime(game.playtimeMinutes)} />
					<Stat
						label="Last played"
						value={relativeTime(game.lastPlayed)}
						hint={game.lastPlayed > 0 ? formatLastPlayed(game.lastPlayed) : undefined}
					/>
				</Reveal>

				{game.playtimeMinutes > 0 && (
					<Reveal i={4}>
						<div className="mb-2 flex justify-between text-[11px] tracking-wide text-white/45 uppercase">
							<span>Share of library</span>
							<span className="tabular-nums">{share < 1 ? "<1" : Math.round(share)}%</span>
						</div>
						<div className="h-1 overflow-hidden rounded-full bg-white/10">
							<div
								className="h-full rounded-full bg-white/70 transition-[width] duration-1000 ease-out"
								style={{ width: armed ? `${Math.max(share, 1)}%` : "0%" }}
							/>
						</div>
					</Reveal>
				)}

				<Reveal i={5} className="flex gap-2">
					<Action icon={Heart} active={favorite} onClick={() => onToggleFavorite(game)}>
						{favorite ? "Favorited" : "Favorite"}
					</Action>
					<Action icon={ExternalLink} onClick={() => OpenStorePage(game.id)}>
						Store
					</Action>
					<Action
						icon={FolderOpen}
						disabled={!game.installed}
						onClick={() => OpenInstallFolder(game.id)}
					>
						Folder
					</Action>
					{isEpic && game.installed && (
						<Action icon={Trash2} disabled={running || !!job} onClick={() => epicActions.uninstall(game)}>
							Uninstall
						</Action>
					)}
				</Reveal>

				<Reveal i={3} className="grid grid-cols-2 gap-3">
					<Stat label="Playtime" value={formatPlaytime(game.playtimeMinutes)} />
					<Stat
						label="Last played"
						value={relativeTime(game.lastPlayed)}
						hint={game.lastPlayed > 0 ? formatLastPlayed(game.lastPlayed) : undefined}
					/>
					{activity && activity.sessions > 0 && (
						<>
						<Stat label="This week" value={formatDuration(activity.weekMinutes)} />
						<Stat
							label="Sessions"
							value={String(activity.sessions)}
							hint={`avg ${formatDuration(Math.round(activity.trackedMinutes / activity.sessions))}`}
						/>
						</>
					)}
				</Reveal>

				{isEpic && game.installed && (
					<Reveal i={6} className="flex gap-2">
						<Action
							icon={ShieldCheck}
							disabled={running || !!job}
							onClick={() => epicActions.verify(game)}
						>
							Verify
						</Action>
						{game.cloudSaves && (
							<Action
								icon={CloudUpload}
								disabled={running || !!job}
								onClick={() => epicActions.syncSaves(game)}
							>
								Sync saves
							</Action>
						)}
						<Action icon={SlidersHorizontal} onClick={() => epicActions.openSettings(game)}>
							Settings
						</Action>
					</Reveal>
				)}

				{isEpic && game.installed && (proton || game.version) && (
					<Reveal i={6}>
						<p className="text-[11px] text-white/30">
							{[game.version && `Version ${game.version}`, proton && `Runs with ${proton}`]
								.filter(Boolean)
								.join(" · ")}
						</p>
					</Reveal>
				)}

				{game.installPath && (
					<Reveal i={6}>
						<p className="text-[11px] break-all text-white/30">{game.installPath}</p>
					</Reveal>
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
	active,
	onClick,
}: {
	icon: React.ComponentType<{ className?: string }>
	children: React.ReactNode
	disabled?: boolean
	active?: boolean
	onClick: () => void
}) {
	return (
		<button
			onClick={onClick}
			disabled={disabled}
			className={cn(
				"flex h-10 flex-1 items-center justify-center gap-2 rounded-full text-xs ring-1 ring-white/10 transition",
				"disabled:pointer-events-none disabled:opacity-30",
				active
					? "bg-white/15 text-white"
					: "bg-white/5 text-white/80 hover:bg-white/10 hover:text-white"
			)}
		>
			<Icon className={cn("size-3.5", active && "fill-current")} />
			{children}
		</button>
	)
}