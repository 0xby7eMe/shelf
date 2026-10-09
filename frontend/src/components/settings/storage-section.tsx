import { useCallback, useEffect, useMemo, useState } from "react"
import { RefreshCw, Trash2 } from "lucide-react"

import { EpicDeletePrefix, GetStorage, UbisoftReset } from "../../../wailsjs/go/main/App"
import { library, main } from "../../../wailsjs/go/models"
import { Bar, IconButton, Label, Panel, PillButton, SectionHeading } from "@/components/settings/ui"
import { formatBytes } from "@/lib/format"
import { confirm } from "@/lib/confirm"
import { toast } from "@/lib/toast"
import { cn } from "@/lib/utils"

interface Props {
	games: library.Game[]
	onSelect: (game: library.Game) => void
}

type Store = "all" | "steam" | "epic" | "gog" | "ubisoft"

const STORE_LABELS: Record<Store, string> = { all: "All", steam: "Steam", epic: "Epic", gog: "GOG", ubisoft: "Ubisoft" }

const STAT_COLUMNS: Record<number, string> = {
	1: "sm:grid-cols-1",
	2: "sm:grid-cols-2",
	3: "sm:grid-cols-3",
	4: "sm:grid-cols-4",
	5: "sm:grid-cols-5",
}

const sizeOf = (g: library.Game) => g.sizeBytes ?? 0

export function StorageSection({ games, onSelect }: Props) {
	const [storage, setStorage] = useState<main.Storage | null>(null)
	const [loading, setLoading] = useState(false)
	const [store, setStore] = useState<Store>("all")

	const load = useCallback(() => {
		setLoading(true)
		GetStorage()
			.then(setStorage)
			.catch((e) => toast.error("Couldn't read disk usage", { description: String(e) }))
			.finally(() => setLoading(false))
	}, [])
	useEffect(load, [load])

	const prefixes = useMemo(
		() => new Map((storage?.prefixes ?? []).map((p) => [p.appName, p])),
		[storage]
	)

	const rows = useMemo(
		() =>
			games
				.filter((g) => g.installed && (store === "all" || g.source === store))
				.map((g) => ({ game: g, prefix: g.source === "epic" || g.source === "gog" ? (prefixes.get(g.externalId)?.bytes ?? 0) : 0 }))
				.sort((a, b) => sizeOf(b.game) + b.prefix - (sizeOf(a.game) + a.prefix)),
		[games, prefixes, store]
	)
	const largest = Math.max(1, ...rows.map((r) => sizeOf(r.game) + r.prefix))

	const sum = (src: string) =>
		games.filter((g) => g.installed && g.source === src).reduce((n, g) => n + sizeOf(g), 0)
	// Only stores with something installed get a total, so the row stays one line.
	const stats = (["steam", "epic", "gog", "ubisoft"] as const)
		.map((id) => ({ id, bytes: sum(id) }))
		.filter((st) => st.bytes > 0)
	const prefixTotal = (storage?.prefixes ?? []).reduce((n, p) => n + p.bytes, 0)
	// Ubisoft's games all live inside one prefix, shown on its own: deleting it
	// removes every one of them, so it is never offered as a leftover.
	const shared = (storage?.prefixes ?? []).find((p) => p.shared)
	const leftovers = (storage?.prefixes ?? []).filter((p) => !p.installed && !p.shared)

	async function removePrefix(appName: string, title: string, installed: boolean, cover?: string) {
		const ok = await confirm({
			title: installed ? "Reset the Proton prefix?" : "Delete the leftover prefix?",
			description: installed
				? "Windows settings and any saves stored only inside the prefix are lost. Cloud saves are not affected, and the game rebuilds the prefix on its next start."
				: "This game is no longer installed, so nothing uses this prefix. Deleting it frees the space.",
			confirmLabel: installed ? "Reset prefix" : "Delete",
			destructive: true,
			game: { name: title, cover },
		})
		if (!ok) return
		try {
			await EpicDeletePrefix(appName)
			toast.success(installed ? "Prefix reset" : "Prefix deleted", { description: title })
			load()
		} catch (e) {
			toast.error("Couldn't delete the prefix", { description: String(e) })
		}
	}

	async function resetUbisoft() {
		const ok = await confirm({
			title: "Reset Ubisoft Connect?",
			description:
				"This deletes Ubisoft Connect's Proton prefix: its login and every Ubisoft game installed inside it, which means downloading them again. Your Ubisoft account and your library in Shelf are not affected.",
			confirmLabel: "Reset and delete games",
			destructive: true,
		})
		if (!ok) return
		try {
			await UbisoftReset()
			toast.success("Ubisoft Connect was reset")
			load()
		} catch (e) {
			toast.error("Couldn't reset Ubisoft Connect", { description: String(e) })
		}
	}

	return (
		<div className="space-y-6">
			<SectionHeading title="Storage" hint="What your installed games and their Proton prefixes take up." />

			<div className={cn("grid grid-cols-2 gap-3", STAT_COLUMNS[stats.length + 1])}>
				{stats.map((st) => (
					<Stat key={st.id} label={`${STORE_LABELS[st.id]} games`} value={formatBytes(st.bytes)} />
				))}
				<Stat label="Proton prefixes" value={formatBytes(prefixTotal)} />
			</div>

			{storage && storage.volumes.length > 0 && (
				<Panel>
					<Label>Disks</Label>
					<div className="space-y-4">
						{storage.volumes.map((v) => {
							const used = v.total - v.free
							const pct = v.total > 0 ? (used / v.total) * 100 : 0
							return (
								<div key={v.path}>
									<div className="mb-1.5 flex justify-between gap-4 text-xs">
										<span className="truncate text-white/70">{v.path}</span>
										<span className="shrink-0 tabular-nums text-white/45">
											{formatBytes(v.free)} free of {formatBytes(v.total)}
										</span>
									</div>
									<Bar value={pct} className={cn(pct > 90 && "[&>div]:bg-red-400/80")} />
								</div>
							)
						})}
					</div>
				</Panel>
			)}

			<div className="flex items-center justify-between">
				<div className="flex rounded-full bg-white/5 p-1 ring-1 ring-white/5">
					{(["all", "steam", "epic", "gog", "ubisoft"] as const).map((s) => (
						<button
							key={s}
							onClick={() => setStore(s)}
							className={cn(
								"rounded-full px-3 py-1 text-xs transition",
								store === s ? "bg-white/10 text-foreground" : "text-muted-foreground hover:text-foreground"
							)}
						>
							{STORE_LABELS[s]}
						</button>
					))}
				</div>
				<IconButton label="Recalculate" onClick={load} disabled={loading}>
					<RefreshCw className={cn("size-3.5", loading && "animate-spin")} />
				</IconButton>
			</div>

			{rows.length === 0 ? (
				<Panel>
					<p className="text-sm text-white/45">No installed games here.</p>
				</Panel>
			) : (
				<ul className="space-y-2">
					{rows.map(({ game, prefix }) => (
						<li key={game.id} className="rounded-2xl bg-white/[0.04] ring-1 ring-white/[0.06]">
							<div className="flex items-center gap-4 p-4">
								<button
									onClick={() => onSelect(game)}
									className="flex min-w-0 flex-1 items-center gap-4 text-left outline-none"
								>
									{game.cover ? (
										<img src={game.cover} alt="" className="h-12 w-8 shrink-0 rounded object-cover ring-1 ring-white/10" />
									) : (
										<div className="h-12 w-8 shrink-0 rounded bg-white/5" />
									)}
									<span className="min-w-0 flex-1">
										<span className="block truncate text-sm font-medium">{game.name}</span>
										<span className="mt-0.5 block text-[11px] tracking-wide text-white/40 uppercase">
											{game.source}
										</span>
										<Bar value={((sizeOf(game) + prefix) / largest) * 100} className="mt-2" />
									</span>
								</button>
								<div className="shrink-0 text-right tabular-nums">
									<p className="text-sm font-medium">{sizeOf(game) > 0 ? formatBytes(sizeOf(game)) : "—"}</p>
									{prefix > 0 && <p className="text-[11px] text-white/40">+ {formatBytes(prefix)} prefix</p>}
								</div>
								{prefix > 0 && (
									<IconButton label="Reset prefix" onClick={() => removePrefix(game.externalId, game.name, true, game.cover)}>
										<Trash2 className="size-3.5" />
									</IconButton>
								)}
							</div>
						</li>
					))}
				</ul>
			)}

			{shared && (
				<Panel>
					<div className="flex items-center justify-between gap-3">
						<div className="min-w-0">
							<Label>Ubisoft Connect</Label>
							<p className="mt-1 text-sm">
								<span className="font-medium tabular-nums">{formatBytes(shared.bytes)}</span>
								<span className="text-white/45"> for Connect and its Proton prefix</span>
							</p>
							<p className="mt-1 text-xs text-white/40">
								Ubisoft's games are installed inside this prefix and are counted on their own above. Resetting it
								deletes them too.
							</p>
						</div>
						<PillButton variant="danger" className="h-8 shrink-0 px-3.5" onClick={resetUbisoft}>
							Reset
						</PillButton>
					</div>
				</Panel>
			)}

			{leftovers.length > 0 && (
				<Panel>
					<div>
						<Label>Leftover prefixes</Label>
						<p className="mt-1 text-xs text-white/40">From games you uninstalled. Safe to delete.</p>
					</div>
					<ul className="space-y-2">
						{leftovers.map((p) => (
							<li key={p.appName} className="flex items-center justify-between gap-3">
								<div className="min-w-0">
									<p className="truncate text-sm">{p.title}</p>
									<p className="text-[11px] text-white/40 tabular-nums">{formatBytes(p.bytes)}</p>
								</div>
								<PillButton variant="danger" className="h-8 px-3.5" onClick={() => removePrefix(p.appName, p.title, false)}>
									Delete
								</PillButton>
							</li>
						))}
					</ul>
				</Panel>
			)}
		</div>
	)
}

function Stat({ label, value }: { label: string; value: string }) {
	return (
		<div className="rounded-xl bg-white/5 px-4 py-3 ring-1 ring-white/[0.06]">
			<p className="text-[11px] tracking-wide text-white/45 uppercase">{label}</p>
			<p className="mt-1 text-base font-medium tabular-nums">{value}</p>
		</div>
	)
}
