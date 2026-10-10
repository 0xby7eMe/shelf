import { useMemo, useState } from "react"
import { Moon } from "lucide-react"

import { sysmon } from "../../../wailsjs/go/models"
import { AreaChart, Sparkline } from "@/components/monitor/charts"
import { COLORS, niceMax, series, type Hardware } from "@/lib/hardware"
import { formatBitrate, formatBytes, formatGHz, formatPercent, formatRate, formatUptime } from "@/lib/format"
import { cn } from "@/lib/utils"

// --- what can be selected ---

export type ResourceId = "cpu" | "memory" | `disk:${string}` | `net:${string}` | `gpu:${string}`

export interface Resource {
	id: ResourceId
	title: string
	subtitle: string
	value: string
	color: string
	values: (number | null)[]
	max: number
}

const last = <T,>(a: T[]): T | undefined => a[a.length - 1]

const peak = (...lists: (number | null)[][]) => Math.max(0, ...lists.flat().map((v) => v ?? 0))

/** The tiles down the left side, each with a summary and a small graph. */
export function useResources({ info, samples }: Hardware): Resource[] {
	return useMemo(() => {
		const now = last(samples)
		const out: Resource[] = []

		out.push({
			id: "cpu",
			title: "CPU",
			subtitle: info?.cpu.model ?? "",
			value: now ? `${formatPercent(now.cpu.usage)}  ${formatGHz(now.cpu.freqMHz)}` : "",
			color: COLORS.cpu,
			values: series(samples, (s) => s.cpu.usage),
			max: 100,
		})

		const memTotal = now?.memory.total ?? info?.memory.total ?? 0
		out.push({
			id: "memory",
			title: "Memory",
			subtitle: memTotal ? `${formatBytes(memTotal)} total` : "",
			value: now ? `${formatBytes(now.memory.used)}  (${formatPercent((now.memory.used / (memTotal || 1)) * 100)})` : "",
			color: COLORS.memory,
			values: series(samples, (s) => (s.memory.total ? (s.memory.used / s.memory.total) * 100 : 0)),
			max: 100,
		})

		for (const d of info?.disks ?? []) {
			const disk = now?.disks.find((x) => x.name === d.name)
			out.push({
				id: `disk:${d.name}`,
				title: `Disk (${d.name})`,
				subtitle: `${d.kind}${d.model ? `  ${d.model}` : ""}`,
				value: disk ? `${formatPercent(disk.activePct)}  ${formatRate(disk.readBps + disk.writeBps)}` : "",
				color: COLORS.disk,
				values: series(samples, (s) => s.disks.find((x) => x.name === d.name)?.activePct),
				max: 100,
			})
		}

		for (const n of info?.nets ?? []) {
			const net = now?.nets.find((x) => x.name === n.name)
			const vals = series(samples, (s) => {
				const x = s.nets.find((y) => y.name === n.name)
				return x ? x.rxBps + x.txBps : null
			})
			out.push({
				id: `net:${n.name}`,
				title: n.kind === "Wi-Fi" ? `Wi-Fi (${n.name})` : `Ethernet (${n.name})`,
				subtitle: n.addrs?.find((a) => !a.includes(":")) ?? "Not connected",
				value: net ? `↓ ${formatBitrate(net.rxBps)}  ↑ ${formatBitrate(net.txBps)}` : "",
				color: COLORS.net,
				values: vals,
				max: niceMax(peak(vals), 125_000),
			})
		}

		;(now?.gpus ?? []).forEach((g, i) => {
			const usage = g.usage ?? 0
			out.push({
				id: `gpu:${g.id}`,
				title: `GPU ${i}`,
				subtitle: g.name,
				value: g.asleep ? "Powered down" : `${formatPercent(usage)}${g.tempC != null ? `  ${Math.round(g.tempC)}°C` : ""}`,
				color: COLORS.gpu,
				values: series(samples, (s) => s.gpus.find((x) => x.id === g.id)?.usage ?? 0),
				max: 100,
			})
		})
		return out
	}, [info, samples])
}

/** One resource in the list on the left. */
export function Tile({ r, selected, onSelect }: { r: Resource; selected: boolean; onSelect: () => void }) {
	return (
		<button
			onClick={onSelect}
			aria-current={selected ? "true" : undefined}
			className={cn(
				"flex w-full items-center gap-3 rounded-xl p-3 text-left ring-1 transition",
				selected ? "bg-white/10 ring-white/20" : "bg-white/[0.03] ring-white/[0.06] hover:bg-white/[0.07]"
			)}
		>
			<Sparkline values={r.values} max={r.max} color={r.color} className="shrink-0" />
			<span className="min-w-0 flex-1">
				<span className="block truncate text-sm font-medium">{r.title}</span>
				<span className="block truncate text-[11px] text-white/45">{r.subtitle}</span>
				<span className="mt-0.5 block truncate text-[11px] text-white/70 tabular-nums">{r.value}</span>
			</span>
		</button>
	)
}

// --- shared pieces ---

function Heading({ title, right }: { title: string; right?: string }) {
	return (
		<div className="flex items-baseline justify-between gap-4">
			<h2 className="text-lg font-semibold tracking-tight">{title}</h2>
			{right && <p className="truncate text-xs text-white/50">{right}</p>}
		</div>
	)
}

function ChartLabel({ left, right, className }: { left: string; right?: string; className?: string }) {
	return (
		<div className={cn("mb-1.5 flex items-center justify-between text-[11px] text-white/45", className)}>
			<span>{left}</span>
			<span className="tabular-nums">{right}</span>
		</div>
	)
}

function Stat({ label, value, wide }: { label: string; value: React.ReactNode; wide?: boolean }) {
	return (
		<div className={cn("min-w-0", wide && "col-span-2")}>
			<p className="text-[11px] text-white/45">{label}</p>
			<p className="mt-0.5 truncate text-lg tabular-nums">{value}</p>
		</div>
	)
}

function Facts({ rows }: { rows: [string, React.ReactNode][] }) {
	return (
		<dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1 text-xs">
			{rows
				.filter(([, v]) => v !== "" && v != null)
				.map(([k, v]) => (
					<div key={k} className="contents">
						<dt className="text-white/45">{k}</dt>
						<dd className="min-w-0 truncate text-white/80">{v}</dd>
					</div>
				))}
		</dl>
	)
}

function Card({ children, className }: { children: React.ReactNode; className?: string }) {
	return <div className={cn("space-y-4 rounded-2xl bg-white/[0.04] p-5 ring-1 ring-white/[0.06]", className)}>{children}</div>
}

const temp = (t: number | null | undefined) => (t == null ? "—" : `${Math.round(t)}°C`)

// --- CPU ---

export function CpuPanel({ info, samples, ticks }: Hardware) {
	const [mode, setMode] = useState<"overall" | "cores">("overall")
	const now = last(samples)
	const cores = now?.cpu.cores ?? []

	return (
		<div className="space-y-5">
			<Heading title="CPU" right={info?.cpu.model} />
			<Card>
				<div className="flex items-start justify-between gap-4">
					<ChartLabel className="flex-1" left="% Utilization over 60 seconds" right="100%" />
					<div className="mb-1.5 flex rounded-full bg-white/5 p-0.5 text-[11px] ring-1 ring-white/5">
						{(["overall", "cores"] as const).map((m) => (
							<button
								key={m}
								onClick={() => setMode(m)}
								className={cn("rounded-full px-3 py-1 transition", mode === m ? "bg-white/10 text-white" : "text-white/50 hover:text-white")}
							>
								{m === "overall" ? "Overall" : "Logical processors"}
							</button>
						))}
					</div>
				</div>
				{mode === "overall" ? (
					<AreaChart
						series={[{ values: series(samples, (s) => s.cpu.usage), color: COLORS.cpu }]}
						max={100}
						phase={ticks}
						className="h-64"
						label="CPU utilization"
					/>
				) : (
					<div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
						{cores.map((_, i) => (
							<div key={i}>
								<AreaChart
									series={[{ values: series(samples, (s) => s.cpu.cores[i]), color: COLORS.cpu }]}
									max={100}
									phase={ticks}
									grid={false}
									className="h-16"
									label={`Logical processor ${i}`}
								/>
								<p className="mt-1 flex justify-between text-[10px] text-white/45 tabular-nums">
									<span>CPU {i}</span>
									<span>{formatPercent(cores[i] ?? 0)}</span>
								</p>
							</div>
						))}
					</div>
				)}
				<div className="grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-4">
					<Stat label="Utilization" value={now ? formatPercent(now.cpu.usage) : "—"} />
					<Stat label="Speed" value={now?.cpu.freqMHz ? formatGHz(now.cpu.freqMHz) : "—"} />
					<Stat label="Temperature" value={temp(now?.cpu.tempC)} />
					<Stat label="Up time" value={now ? formatUptime(now.cpu.uptime) : "—"} />
					<Stat label="Processes" value={now?.cpu.processes ?? "—"} />
					<Stat label="Threads" value={now?.cpu.threads ?? "—"} />
					{/* Windows keeps no load average. */}
					{!info?.host.os.startsWith("Windows") && (
						<Stat label="Load average" value={now ? now.cpu.load.map((l) => l.toFixed(2)).join("  ") : "—"} wide />
					)}
				</div>
			</Card>
			<Card>
				<Facts
					rows={[
						["Model", info?.cpu.model],
						["Cores", info?.cpu.cores],
						["Logical processors", info?.cpu.threads],
						["Maximum speed", info?.cpu.maxMHz ? formatGHz(info.cpu.maxMHz) : ""],
						["Architecture", info?.cpu.arch],
						[info?.host.os.startsWith("Windows") ? "Version" : "Kernel", info?.host.kernel],
					]}
				/>
			</Card>
		</div>
	)
}

// --- memory ---

export function MemoryPanel({ info, samples, ticks }: Hardware) {
	const now = last(samples)
	const m = now?.memory
	const total = m?.total ?? info?.memory.total ?? 0
	// Used and available add up to the total; of the available part, the free part is
	// untouched and the rest is cache that programs could have back at once.
	const used = m?.used ?? 0
	const free = m?.free ?? 0
	const cache = Math.max(0, (m?.available ?? 0) - free)
	const pct = (v: number) => (total ? `${(v / total) * 100}%` : "0%")

	return (
		<div className="space-y-5">
			<Heading title="Memory" right={total ? `${formatBytes(total)}` : undefined} />
			<Card>
				<ChartLabel left="Memory usage over 60 seconds" right={total ? formatBytes(total) : ""} />
				<AreaChart
					series={[{ values: series(samples, (s) => (s.memory.total ? (s.memory.used / s.memory.total) * 100 : 0)), color: COLORS.memory }]}
					max={100}
					phase={ticks}
					className="h-64"
					label="Memory usage"
				/>
				<div>
					<ChartLabel left="Memory composition" />
					<div className="flex h-5 overflow-hidden rounded-md bg-black/25 ring-1 ring-white/10">
						<div style={{ width: pct(used), background: COLORS.memory }} title={`In use ${formatBytes(used)}`} />
						<div style={{ width: pct(cache), background: `${COLORS.memory}66` }} title={`Cached ${formatBytes(cache)}`} />
					</div>
					<div className="mt-2 flex gap-5 text-[11px] text-white/50">
						<Legend color={COLORS.memory} label="In use" />
						<Legend color={`${COLORS.memory}66`} label="Cached, can be reclaimed" />
						<Legend color="transparent" label="Free" outline />
					</div>
				</div>
				<div className="grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-4">
					<Stat label="In use" value={m ? formatBytes(m.used) : "—"} />
					<Stat label="Available" value={m ? formatBytes(m.available) : "—"} />
					<Stat label="Cached" value={m ? formatBytes(m.cached) : "—"} />
					<Stat label="Free" value={m ? formatBytes(m.free) : "—"} />
					<Stat label="Buffers" value={m ? formatBytes(m.buffers) : "—"} />
					<Stat
						label="Swap"
						value={m && m.swapTotal ? `${formatBytes(m.swapUsed)} of ${formatBytes(m.swapTotal)}` : "None"}
						wide
					/>
				</div>
			</Card>
		</div>
	)
}

function Legend({ color, label, outline }: { color: string; label: string; outline?: boolean }) {
	return (
		<span className="flex items-center gap-1.5">
			<span className={cn("size-2.5 rounded-sm", outline && "ring-1 ring-white/30")} style={{ background: color }} />
			{label}
		</span>
	)
}

// --- disks ---

export function DiskPanel({ name, info, samples, ticks }: Hardware & { name: string }) {
	const disk = info?.disks.find((d) => d.name === name)
	const now = last(samples)?.disks.find((d) => d.name === name)
	const read = series(samples, (s) => s.disks.find((d) => d.name === name)?.readBps)
	const write = series(samples, (s) => s.disks.find((d) => d.name === name)?.writeBps)
	const scale = niceMax(peak(read, write), 1_000_000)

	return (
		<div className="space-y-5">
			<Heading title={`Disk (${name})`} right={disk?.model} />
			<Card>
				<div>
					<ChartLabel left="Active time" right="100%" />
					<AreaChart
						series={[{ values: series(samples, (s) => s.disks.find((d) => d.name === name)?.activePct), color: COLORS.disk }]}
						max={100}
						phase={ticks}
						className="h-40"
						label="Disk active time"
					/>
				</div>
				<div>
					<ChartLabel left="Disk transfer rate" right={formatRate(scale)} />
					<AreaChart
						series={[
							{ values: read, color: COLORS.disk, label: "Read" },
							{ values: write, color: "#8ab4ff", label: "Write" },
						]}
						max={scale}
						phase={ticks}
						className="h-40"
						label="Disk transfer rate"
					/>
					<div className="mt-2 flex gap-5 text-[11px] text-white/50">
						<Legend color={COLORS.disk} label="Read" />
						<Legend color="#8ab4ff" label="Write" />
					</div>
				</div>
				<div className="grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-4">
					<Stat label="Active time" value={now ? formatPercent(now.activePct) : "—"} />
					<Stat label="Read speed" value={now ? formatRate(now.readBps) : "—"} />
					<Stat label="Write speed" value={now ? formatRate(now.writeBps) : "—"} />
					<Stat label="Temperature" value={temp(now?.tempC)} />
				</div>
			</Card>
			<Card>
				<Facts
					rows={[
						["Model", disk?.model],
						["Type", disk?.kind],
						["Capacity", disk?.size ? formatBytes(disk.size) : ""],
					]}
				/>
				{(disk?.mounts?.length ?? 0) > 0 && (
					<div className="space-y-3 pt-1">
						{disk!.mounts.map((m) => {
							const usedPct = m.total ? ((m.total - m.free) / m.total) * 100 : 0
							return (
								<div key={m.path}>
									<div className="mb-1 flex justify-between gap-4 text-xs">
										<span className="truncate text-white/75">
											{m.path} <span className="text-white/35">{m.fs}</span>
										</span>
										<span className="shrink-0 tabular-nums text-white/45">
											{formatBytes(m.free)} free of {formatBytes(m.total)}
										</span>
									</div>
									<div className="h-1.5 overflow-hidden rounded-full bg-white/10">
										<div
											className={cn("h-full rounded-full", usedPct > 90 ? "bg-red-400/80" : "bg-white/60")}
											style={{ width: `${usedPct}%` }}
										/>
									</div>
								</div>
							)
						})}
					</div>
				)}
			</Card>
		</div>
	)
}

// --- network ---

export function NetworkPanel({ name, info, samples, ticks }: Hardware & { name: string }) {
	const nic = info?.nets.find((n) => n.name === name)
	const now = last(samples)?.nets.find((n) => n.name === name)
	const rx = series(samples, (s) => s.nets.find((n) => n.name === name)?.rxBps)
	const tx = series(samples, (s) => s.nets.find((n) => n.name === name)?.txBps)
	const scale = niceMax(peak(rx, tx), 125_000)

	return (
		<div className="space-y-5">
			<Heading title={nic?.kind === "Wi-Fi" ? "Wi-Fi" : "Ethernet"} right={name} />
			<Card>
				<ChartLabel left="Throughput over 60 seconds" right={formatBitrate(scale)} />
				<AreaChart
					series={[
						{ values: rx, color: COLORS.net, label: "Receive" },
						{ values: tx, color: "#ff6f91", label: "Send" },
					]}
					max={scale}
					phase={ticks}
					className="h-64"
					label="Network throughput"
				/>
				<div className="flex gap-5 text-[11px] text-white/50">
					<Legend color={COLORS.net} label="Receive" />
					<Legend color="#ff6f91" label="Send" />
				</div>
				<div className="grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-4">
					<Stat label="Receive" value={now ? formatBitrate(now.rxBps) : "—"} />
					<Stat label="Send" value={now ? formatBitrate(now.txBps) : "—"} />
					<Stat label="Link speed" value={nic?.speedMbps ? `${nic.speedMbps} Mbps` : "—"} />
				</div>
			</Card>
			<Card>
				<Facts
					rows={[
						["Adapter", name],
						["Type", nic?.kind],
						["Addresses", nic?.addrs?.length ? nic.addrs.join(", ") : "Not connected"],
					]}
				/>
			</Card>
		</div>
	)
}

// --- graphics cards ---

export function GpuPanel({ id, info, samples, ticks }: Hardware & { id: string }) {
	const now = last(samples)?.gpus.find((g) => g.id === id)
	const meta = info?.gpus.find((g) => g.id === id)
	const total = now?.vramTotal || meta?.vramTotal || 0
	const nvidia = (now?.name ?? meta?.name ?? "").includes("NVIDIA")

	return (
		<div className="space-y-5">
			<Heading title="GPU" right={now?.name ?? meta?.name} />
			{now?.asleep && (
				<div className="flex items-start gap-3 rounded-xl bg-white/5 px-4 py-3 text-xs leading-relaxed text-white/65 ring-1 ring-white/10">
					<Moon className="mt-0.5 size-4 shrink-0 text-white/50" />
					This graphics card is powered down to save energy. Shelf doesn't wake it to take a reading, so its
					numbers appear once something starts using it.
				</div>
			)}
			<Card>
				<div>
					<ChartLabel left="Utilization" right="100%" />
					<AreaChart
						series={[{ values: series(samples, (s) => s.gpus.find((g) => g.id === id)?.usage ?? 0), color: COLORS.gpu }]}
						max={100}
						phase={ticks}
						className="h-40"
						label="GPU utilization"
					/>
				</div>
				{total > 0 && (
					<div>
						<ChartLabel left="Dedicated GPU memory" right={formatBytes(total)} />
						<AreaChart
							series={[{ values: series(samples, (s) => s.gpus.find((g) => g.id === id)?.vramUsed ?? 0), color: "#ff9aa6" }]}
							max={total}
							phase={ticks}
							className="h-32"
							label="GPU memory"
						/>
					</div>
				)}
				<div className="grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-4">
					<Stat label="Utilization" value={now?.usage != null ? formatPercent(now.usage) : "—"} />
					<Stat label="GPU memory" value={now && total ? `${formatBytes(now.vramUsed)} / ${formatBytes(total)}` : "—"} />
					<Stat label="Temperature" value={temp(now?.tempC)} />
					<Stat label="Power" value={now?.powerW != null ? `${now.powerW.toFixed(1)} W` : "—"} />
					<Stat label="Clock" value={now?.clockMHz != null ? `${Math.round(now.clockMHz)} MHz` : "—"} />
				</div>
			</Card>
			<Card>
				<Facts rows={[["Name", now?.name ?? meta?.name], ["Driver", meta?.driver], ["Card", id]]} />
				{nvidia && !now?.asleep && (
					<p className="text-[11px] leading-relaxed text-white/35">
						Reading an NVIDIA card's numbers keeps it awake for as long as this page is open. It powers down
						again shortly after you leave.
					</p>
				)}
			</Card>
		</div>
	)
}

/** The panel for whatever is selected. */
export function ResourcePanel({ id, hw }: { id: ResourceId; hw: Hardware }) {
	if (id === "cpu") return <CpuPanel {...hw} />
	if (id === "memory") return <MemoryPanel {...hw} />
	const [kind, name] = [id.slice(0, id.indexOf(":")), id.slice(id.indexOf(":") + 1)]
	if (kind === "disk") return <DiskPanel {...hw} name={name} />
	if (kind === "net") return <NetworkPanel {...hw} name={name} />
	return <GpuPanel {...hw} id={name} />
}

export type { sysmon }
