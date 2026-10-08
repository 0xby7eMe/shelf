import { useMemo, useState } from "react"
import { ChevronDown, ExternalLink, RefreshCw } from "lucide-react"

import { friends, library } from "../../wailsjs/go/models"
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime"
import { Input } from "@/components/ui/input"
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogTitle,
} from "@/components/ui/dialog"
import { Label, PillButton, inputClass } from "@/components/settings/ui"
import { useFriends } from "@/lib/use-friends"
import { cn } from "@/lib/utils"

interface Props {
	open: boolean
	games: library.Game[]
	onOpenChange: (open: boolean) => void
	onSelect: (game: library.Game) => void
	// Opens the settings page where a launcher is set up.
	onSetup: (source: string) => void
}

// Launchers whose friends need something set up on a settings tab.
const SETUP_TABS = new Set(["steam"])

const DOT: Record<string, string> = {
	playing: "bg-emerald-400",
	online: "bg-sky-400",
	away: "bg-amber-400",
	offline: "bg-white/20",
}

export function FriendsDialog({ open, games, onOpenChange, onSelect, onSetup }: Props) {
	const { snapshot, loading, error, refresh, save, disconnect } = useFriends(open)
	const [showOffline, setShowOffline] = useState(false)

	// A friend's game is matched to yours by the same store and id Shelf uses.
	const owned = useMemo(() => new Map(games.map((g) => [`${g.source}:${g.externalId}`, g])), [games])

	const all = snapshot?.friends ?? []
	const live = all.filter((f) => f.status !== "offline")
	const offline = all.filter((f) => f.status === "offline")
	const ready = snapshot?.providers.some((p) => p.ready && !p.message) ?? false

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
				<div className="flex items-start justify-between gap-4">
					<div>
						<DialogTitle className="text-[11px] font-medium tracking-[0.25em] text-muted-foreground uppercase">
							Friends
						</DialogTitle>
						<DialogDescription className="mt-2 text-sm text-muted-foreground">
							Who is online and what they are playing, from the launchers that allow it.
						</DialogDescription>
					</div>
					<button
						onClick={refresh}
						disabled={loading}
						aria-label="Refresh"
						className="mt-0.5 rounded-full p-2 text-white/50 transition hover:bg-white/5 hover:text-white disabled:opacity-40"
					>
						<RefreshCw className={cn("size-4", loading && "animate-spin")} />
					</button>
				</div>

				{error && <p className="text-sm text-red-300/80">{error}</p>}

				{ready && (
					<div className="space-y-5">
						{live.length === 0 ? (
							<p className="text-sm text-muted-foreground">
								{all.length === 0 ? "No friends found." : "Nobody is online right now."}
							</p>
						) : (
							<ul className="space-y-1">
								{live.map((f) => (
									<FriendRow key={f.source + f.id} friend={f} owned={owned} onSelect={(g) => { onOpenChange(false); onSelect(g) }} />
								))}
							</ul>
						)}

						{offline.length > 0 && (
							<div>
								<button
									onClick={() => setShowOffline((v) => !v)}
									className="flex items-center gap-1.5 text-[11px] tracking-wide text-white/45 uppercase transition hover:text-white/70"
								>
									<ChevronDown className={cn("size-3.5 transition", !showOffline && "-rotate-90")} />
									Offline · {offline.length}
								</button>
								{showOffline && (
									<ul className="mt-2 space-y-1">
										{offline.map((f) => (
											<FriendRow key={f.source + f.id} friend={f} owned={owned} onSelect={() => {}} />
										))}
									</ul>
								)}
							</div>
						)}
					</div>
				)}

				<div className="space-y-3">
					{snapshot?.providers.map((p) => (
						<ProviderCard
							key={p.source}
							provider={p}
							onSave={save}
							onDisconnect={disconnect}
							onSetup={SETUP_TABS.has(p.source) ? () => { onOpenChange(false); onSetup(p.source) } : undefined}
						/>
					))}
				</div>
			</DialogContent>
		</Dialog>
	)
}

function Avatar({ friend }: { friend: friends.Friend }) {
	return (
		<span className="relative shrink-0">
			{friend.avatar ? (
				<img src={friend.avatar} alt="" loading="lazy" className="size-9 rounded-full object-cover ring-1 ring-white/10" />
			) : (
				<span className="grid size-9 place-items-center rounded-full bg-white/10 text-xs font-medium">
					{friend.name.slice(0, 1).toUpperCase()}
				</span>
			)}
			<span className={cn("absolute -right-0.5 -bottom-0.5 size-3 rounded-full ring-2 ring-[#0a0a0a]", DOT[friend.status])} />
		</span>
	)
}

function FriendRow({
	friend,
	owned,
	onSelect,
}: {
	friend: friends.Friend
	owned: Map<string, library.Game>
	onSelect: (game: library.Game) => void
}) {
	const playing = friend.playing
	const game = playing ? owned.get(`${playing.source}:${playing.id}`) : undefined

	return (
		<li className={cn("flex items-center gap-3 rounded-xl px-2 py-1.5", friend.status === "offline" && "opacity-50")}>
			<Avatar friend={friend} />
			<div className="min-w-0 flex-1">
				<p className="truncate text-sm font-medium">{friend.name}</p>
				<p className={cn("truncate text-xs", playing ? "text-emerald-300/90" : "text-white/45")}>
					{playing ? playing.name : friend.status === "away" ? "Away" : friend.status === "online" ? "Online" : "Offline"}
				</p>
			</div>
			{playing && (
				game ? (
					<button
						onClick={() => onSelect(game)}
						className="shrink-0 rounded-full bg-white/10 px-3 py-1 text-[11px] font-medium transition hover:bg-white/15"
					>
						In your library
					</button>
				) : (
					<span className="shrink-0 text-[11px] text-white/35">Not in your library</span>
				)
			)}
			{friend.profileUrl && (
				<button
					onClick={() => BrowserOpenURL(friend.profileUrl!)}
					aria-label={`Open ${friend.name}'s profile`}
					className="shrink-0 rounded-full p-1.5 text-white/35 transition hover:bg-white/5 hover:text-white"
				>
					<ExternalLink className="size-3.5" />
				</button>
			)}
		</li>
	)
}

// One launcher: what it needs from you, or why it can't be shown.
function ProviderCard({
	provider,
	onSave,
	onDisconnect,
	onSetup,
}: {
	provider: friends.ProviderInfo
	onSave: (source: string, values: Record<string, string>) => void
	onDisconnect: (source: string) => void
	onSetup?: () => void
}) {
	const [values, setValues] = useState<Record<string, string>>({})
	const hasFields = provider.fields.length > 0
	const connected = provider.fields.some((f) => f.set)
	const [editing, setEditing] = useState(false)
	const showForm = hasFields && (!provider.ready || editing || (provider.ready && !!provider.message))

	return (
		<div className="rounded-2xl bg-white/[0.04] p-4 ring-1 ring-white/[0.06]">
			<div className="flex items-center justify-between gap-3">
				<p className="text-sm font-medium">{provider.name}</p>
				{provider.ready && !provider.message && (
					<span className="flex items-center gap-2 text-xs text-white/45">
						{provider.count} {provider.count === 1 ? "friend" : "friends"}
						{hasFields && (
							<button onClick={() => setEditing((v) => !v)} className="underline-offset-2 hover:text-white hover:underline">
								{editing ? "Close" : "Settings"}
							</button>
						)}
					</span>
				)}
			</div>

			{provider.message && <p className="mt-1.5 text-xs leading-relaxed text-white/50">{provider.message}</p>}

			{onSetup && !provider.fields.length && (!provider.ready || !!provider.message) && (
				<div className="mt-3">
					<PillButton onClick={onSetup}>Open {provider.name} settings</PillButton>
				</div>
			)}

			{showForm && (
				<form
					className="mt-4 space-y-4"
					onSubmit={(e) => {
						e.preventDefault()
						onSave(provider.source, values)
						setValues({})
						setEditing(false)
					}}
				>
					{provider.fields.map((f) => (
						<div key={f.key} className="space-y-2">
							<Label>{f.label}</Label>
							<Input
								type={f.secret ? "password" : "text"}
								autoComplete="off"
								spellCheck={false}
								value={values[f.key] ?? ""}
								placeholder={f.set ? "Saved. Type to replace it." : ""}
								onChange={(e) => setValues((v) => ({ ...v, [f.key]: e.target.value }))}
								className={inputClass}
							/>
							{(f.help || f.helpUrl) && (
								<p className="text-xs leading-relaxed text-white/40">
									{f.help}{" "}
									{f.helpUrl && (
										<button type="button" onClick={() => BrowserOpenURL(f.helpUrl!)} className="underline underline-offset-2 hover:text-white/70">
											Get one
										</button>
									)}
								</p>
							)}
						</div>
					))}
					<div className="flex gap-2">
						<PillButton type="submit" variant="solid">
							Save
						</PillButton>
						{connected && (
							<PillButton type="button" variant="danger" onClick={() => onDisconnect(provider.source)}>
								Disconnect
							</PillButton>
						)}
					</div>
				</form>
			)}
		</div>
	)
}
