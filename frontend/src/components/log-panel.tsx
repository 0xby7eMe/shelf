import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react"
import { Copy, FolderOpen, Trash2, X } from "lucide-react"

import { OpenLogsFolder } from "../../wailsjs/go/main/App"
import { applog, library } from "../../wailsjs/go/models"
import { clearLogs, setLogOpen, useLogLines, useLogOpen } from "@/lib/logs"
import { setPrefs, usePrefs } from "@/lib/prefs"
import { toast } from "@/lib/toast"
import { cn } from "@/lib/utils"

const GROUPS = {
	all: { label: "All", sources: null },
	downloads: { label: "Downloads", sources: ["install", "update", "repair", "verify", "import", "updates"] },
	launch: { label: "Launch", sources: ["launch"] },
	saves: { label: "Saves", sources: ["saves"] },
	other: { label: "Other", sources: ["legendary", "ubisoft", "app"] },
} as const

type Group = keyof typeof GROUPS

const levelColor: Record<string, string> = {
	error: "text-red-300",
	warn: "text-amber-300",
	info: "text-white/70",
}

const clock = new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false })

// A console along the bottom of the window showing what downloads, launches and
// cloud saves are doing, in the manner of a game launcher's output window.
export function LogPanel({ games }: { games: library.Game[] }) {
	const open = useLogOpen()
	const { logHeight } = usePrefs()
	const lines = useLogLines()
	const [group, setGroup] = useState<Group>("all")
	const bodyRef = useRef<HTMLDivElement>(null)
	const stick = useRef(true) // follow new lines unless the user scrolled up

	const titles = useMemo(
		() => new Map(games.filter((g) => g.source === "epic" || g.source === "ubisoft").map((g) => [g.externalId, g.name])),
		[games]
	)

	const shown = useMemo(() => {
		const sources = GROUPS[group].sources as readonly string[] | null
		return sources ? lines.filter((l) => sources.includes(l.source)) : lines
	}, [lines, group])

	useLayoutEffect(() => {
		const el = bodyRef.current
		if (open && el && stick.current) el.scrollTop = el.scrollHeight
	}, [shown, open])

	// Opening the panel always starts at the newest line.
	useEffect(() => {
		if (open) stick.current = true
	}, [open])

	function startResize(e: React.PointerEvent<HTMLDivElement>) {
		const startY = e.clientY
		const startH = logHeight
		const target = e.currentTarget
		target.setPointerCapture(e.pointerId)
		const move = (ev: PointerEvent) =>
			setPrefs({ logHeight: Math.round(Math.min(window.innerHeight * 0.75, Math.max(120, startH + startY - ev.clientY))) })
		const up = () => {
			target.removeEventListener("pointermove", move)
			target.removeEventListener("pointerup", up)
		}
		target.addEventListener("pointermove", move)
		target.addEventListener("pointerup", up)
	}

	function text(l: applog.Line) {
		const app = l.app ? ` ${titles.get(l.app) ?? l.app}` : ""
		return `[${clock.format(l.time)}] ${l.source}${app}: ${l.text}`
	}

	async function copy() {
		try {
			await navigator.clipboard.writeText(shown.map(text).join("\n"))
			toast.success("Log copied", { description: `${shown.length} lines`, duration: 2500 })
		} catch {
			toast.error("Couldn't copy the log")
		}
	}

	if (!open) return null

	return (
		<section
			aria-label="Log"
			data-nav-skip
			style={{ height: logHeight, backdropFilter: "blur(14px)", WebkitBackdropFilter: "blur(14px)" }}
			className="fixed inset-x-0 bottom-0 z-[45] flex animate-in flex-col border-t border-white/10 bg-[rgba(8,8,8,0.92)] duration-200 slide-in-from-bottom-4 fade-in"
		>
			<div
				onPointerDown={startResize}
				role="separator"
				aria-orientation="horizontal"
				aria-label="Resize log"
				className="absolute inset-x-0 -top-1 z-10 h-2 cursor-ns-resize"
			/>

			<header className="flex h-10 shrink-0 items-center gap-3 border-b border-white/5 px-4">
				<h2 className="text-[11px] font-medium tracking-[0.25em] text-white/60 uppercase">Log</h2>
				<div className="flex rounded-full bg-white/5 p-0.5 ring-1 ring-white/5">
					{(Object.keys(GROUPS) as Group[]).map((g) => (
						<button
							key={g}
							onClick={() => setGroup(g)}
							className={cn(
								"rounded-full px-2.5 py-0.5 text-[11px] transition",
								group === g ? "bg-white/10 text-white" : "text-white/45 hover:text-white"
							)}
						>
							{GROUPS[g].label}
						</button>
					))}
				</div>
				<span className="text-[11px] text-white/30 tabular-nums">{shown.length} lines</span>
				<div className="ml-auto flex items-center gap-1">
					<PanelButton label="Copy log" onClick={copy}>
						<Copy className="size-3.5" />
					</PanelButton>
					<PanelButton label="Open log files" onClick={() => OpenLogsFolder().catch(() => {})}>
						<FolderOpen className="size-3.5" />
					</PanelButton>
					<PanelButton label="Clear log" onClick={clearLogs}>
						<Trash2 className="size-3.5" />
					</PanelButton>
					<PanelButton label="Close log" onClick={() => setLogOpen(false)}>
						<X className="size-4" />
					</PanelButton>
				</div>
			</header>

			<div
				ref={bodyRef}
				onScroll={(e) => {
					const el = e.currentTarget
					stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40
				}}
				className="min-h-0 flex-1 overflow-y-auto px-4 py-2 font-mono text-[11px] leading-[1.55] select-text"
			>
				{shown.length === 0 ? (
					<p className="pt-2 text-white/30">
						Nothing logged yet. Downloads, game launches and cloud saves show up here.
					</p>
				) : (
					shown.map((l, i) => (
						<div key={i} className={cn("flex gap-3 whitespace-pre-wrap break-words", levelColor[l.level] ?? "text-white/70")}>
							<span className="shrink-0 text-white/30 tabular-nums">{clock.format(l.time)}</span>
							<span className="w-16 shrink-0 truncate text-white/40">{l.source}</span>
							{l.app && <span className="max-w-40 shrink-0 truncate text-white/45">{titles.get(l.app) ?? l.app}</span>}
							<span className="min-w-0 flex-1">{l.text}</span>
						</div>
					))
				)}
			</div>
		</section>
	)
}

function PanelButton({ label, onClick, children }: { label: string; onClick: () => void; children: React.ReactNode }) {
	return (
		<button
			onClick={onClick}
			title={label}
			aria-label={label}
			className="grid size-7 place-items-center rounded-full text-white/50 transition hover:bg-white/10 hover:text-white"
		>
			{children}
		</button>
	)
}
