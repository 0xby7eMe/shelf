import { useEffect, useMemo, useState } from "react"

import { library, nearby } from "../../wailsjs/go/models"
import { Input } from "@/components/ui/input"
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogTitle,
} from "@/components/ui/dialog"
import { ToggleRow } from "@/components/toggle-row"
import { Label, inputClass } from "@/components/settings/ui"
import { cn } from "@/lib/utils"

interface Props {
	open: boolean
	games: library.Game[]
	snapshot: nearby.Snapshot | null
	error: string
	onSave: (s: nearby.Settings) => void
	onOpenChange: (open: boolean) => void
	onSelect: (game: library.Game) => void
}

const STORE: Record<string, string> = { steam: "Steam", epic: "Epic", gog: "GOG", ubisoft: "Ubisoft" }
const EVERYONE = "*"

function stores(list: string[] | undefined) {
	return (list ?? []).map((s) => STORE[s] ?? s).join(", ")
}

export function NearbyDialog({ open, games, snapshot, error, onSave, onOpenChange, onSelect }: Props) {
	const byId = useMemo(() => new Map(games.map((g) => [g.id, g])), [games])
	const peers = snapshot?.peers ?? []
	const [tab, setTab] = useState(EVERYONE)

	// With one peer there is no "everyone"; a peer that left can't stay picked.
	const valid = tab === EVERYONE ? peers.length > 1 : peers.some((p) => p.id === tab)
	const current = valid ? tab : peers.length > 1 ? EVERYONE : peers[0]?.id
	const peer = peers.find((p) => p.id === current)
	const shared = current === EVERYONE ? (snapshot?.everyone ?? []) : (peer?.common ?? [])
	const them = current === EVERYONE ? "everyone" : (peer?.name ?? "")

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent
				style={{
					backdropFilter: "blur(30px)",
					WebkitBackdropFilter: "blur(30px)",
					backgroundColor: "rgba(10, 10, 10, 0.6)",
				}}
				className="max-h-[85vh] gap-5 overflow-y-auto border-white/10 p-6 sm:max-w-xl"
			>
				<div>
					<DialogTitle className="text-[11px] font-medium tracking-[0.25em] text-muted-foreground uppercase">
						Nearby
					</DialogTitle>
					<DialogDescription className="mt-2 text-sm text-muted-foreground">
						Others running Shelf on your network, and the games you have in common.
					</DialogDescription>
				</div>

				{error && <p className="text-sm text-red-300/80">{error}</p>}
				{snapshot?.message && <p className="text-sm text-amber-200/80">{snapshot.message}</p>}

				{snapshot?.settings.enabled && peers.length === 0 && (
					<div className="flex items-center gap-3 rounded-2xl bg-white/[0.04] p-4 ring-1 ring-white/[0.06]">
						<span className="relative flex size-2.5 shrink-0">
							<span className="absolute inline-flex size-full animate-ping rounded-full bg-sky-400/60" />
							<span className="relative inline-flex size-2.5 rounded-full bg-sky-400" />
						</span>
						<p className="text-sm text-white/60">
							Looking for other Shelfs. They show up here as soon as someone on the same network opens Shelf.
						</p>
					</div>
				)}

				{peers.length > 0 && (
					<div className="space-y-4">
						<div className="flex flex-wrap gap-1.5">
							{peers.length > 1 && (
								<Tab active={current === EVERYONE} onClick={() => setTab(EVERYONE)} count={snapshot?.everyone.length ?? 0}>
									Everyone
								</Tab>
							)}
							{peers.map((p) => (
								<Tab key={p.id} active={current === p.id} onClick={() => setTab(p.id)} count={p.common.length} title={p.host ? `on ${p.host}` : undefined}>
									{p.name}
								</Tab>
							))}
						</div>

						<p className="text-xs text-white/45">
							{current === EVERYONE
								? `${shared.length} ${shared.length === 1 ? "game" : "games"} all ${peers.length + 1} of you have.`
								: `${shared.length} of ${peer?.games ?? 0} games ${peer?.name} has are in your library too.`}
						</p>

						{shared.length === 0 ? (
							<p className="text-sm text-muted-foreground">No games in common{current === EVERYONE ? " with everyone" : ""}.</p>
						) : (
							<ul className="space-y-1">
								{shared.map((s) => (
									<SharedRow
										key={s.key}
										shared={s}
										them={them}
										game={byId.get(s.gameId)}
										onSelect={(g) => {
											onOpenChange(false)
											onSelect(g)
										}}
									/>
								))}
							</ul>
						)}
					</div>
				)}

				{snapshot && <SettingsCard snapshot={snapshot} onSave={onSave} />}
			</DialogContent>
		</Dialog>
	)
}

function Tab({
	active,
	count,
	title,
	onClick,
	children,
}: {
	active: boolean
	count: number
	title?: string
	onClick: () => void
	children: React.ReactNode
}) {
	return (
		<button
			onClick={onClick}
			title={title}
			className={cn(
				"flex h-8 max-w-48 items-center gap-2 rounded-full px-3.5 text-xs ring-1 transition",
				active ? "bg-white text-black ring-transparent" : "bg-white/5 text-white/70 ring-white/10 hover:bg-white/10 hover:text-white"
			)}
		>
			<span className="truncate">{children}</span>
			<span className={cn("tabular-nums", active ? "text-black/50" : "text-white/40")}>{count}</span>
		</button>
	)
}

function SharedRow({
	shared,
	them,
	game,
	onSelect,
}: {
	shared: nearby.Shared
	them: string
	game?: library.Game
	onSelect: (game: library.Game) => void
}) {
	const together = shared.installed && shared.theirInstalled
	const status = together
		? "Installed on both sides"
		: shared.installed
			? `Not installed for ${them}`
			: shared.theirInstalled
				? "Not installed on yours"
				: "Not installed on either side"

	return (
		<li>
			<button
				disabled={!game}
				onClick={() => game && onSelect(game)}
				className="flex w-full items-center gap-3 rounded-xl px-2 py-1.5 text-left transition hover:bg-white/5 disabled:pointer-events-none"
			>
				{game?.cover ? (
					<img src={game.cover} alt="" loading="lazy" className="h-10 w-7 shrink-0 rounded object-cover ring-1 ring-white/10" />
				) : (
					<div className="h-10 w-7 shrink-0 rounded bg-white/5" />
				)}
				<div className="min-w-0 flex-1">
					<p className="truncate text-sm font-medium">{shared.name}</p>
					<p className="truncate text-xs text-white/40">
						You: {stores(shared.sources)} · {them === "everyone" ? "Them" : them}: {stores(shared.theirSources)}
					</p>
				</div>
				<span
					className={cn(
						"shrink-0 rounded-full px-2.5 py-1 text-[11px]",
						together ? "bg-emerald-400/15 text-emerald-300" : "text-white/35"
					)}
				>
					{together ? "Ready to play" : status}
				</span>
			</button>
		</li>
	)
}

// Whether to take part, and the name the others see.
function SettingsCard({ snapshot, onSave }: { snapshot: nearby.Snapshot; onSave: (s: nearby.Settings) => void }) {
	const s = snapshot.settings
	const [name, setName] = useState(s.name)
	useEffect(() => setName(s.name), [s.name])

	function saveName() {
		if (name.trim() !== s.name) onSave(nearby.Settings.createFrom({ ...s, name: name.trim() }))
	}

	return (
		<div className="space-y-4 rounded-2xl bg-white/[0.04] p-4 ring-1 ring-white/[0.06]">
			<ToggleRow
				label="Find Shelfs on this network"
				hint="Shows this Shelf to the others on your network and shares your list of games with them: names, stores and what is installed. Nothing else, and nothing leaves the network."
				checked={s.enabled}
				onChange={(v) => onSave(nearby.Settings.createFrom({ ...s, enabled: v }))}
			/>
			{s.enabled && (
				<form
					className="space-y-2"
					onSubmit={(e) => {
						e.preventDefault()
						saveName()
					}}
				>
					<Label>Your name on the network</Label>
					<Input
						value={name}
						maxLength={40}
						spellCheck={false}
						placeholder={snapshot.name}
						onChange={(e) => setName(e.target.value)}
						onBlur={saveName}
						className={inputClass}
					/>
					<p className="text-xs text-white/40">
						Others see you as <span className="text-white/70">{snapshot.name}</span>.
						{snapshot.port ? ` If a firewall is on, allow UDP port 5353 and TCP port ${snapshot.port}.` : ""}
					</p>
				</form>
			)}
		</div>
	)
}
