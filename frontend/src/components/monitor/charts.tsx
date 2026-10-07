import { useId } from "react"

import { WINDOW } from "@/lib/hardware"
import { cn } from "@/lib/utils"

export interface Series {
	values: (number | null)[]
	color: string
	label?: string
}

const W = 600
const H = 200

function xOf(offset: number, i: number, size: number) {
	return ((offset + i) / (size - 1)) * W
}

function yOf(v: number, max: number) {
	return H - Math.min(Math.max(v / max, 0), 1) * H
}

/** The line through a series, newest at the right edge, and the same line closed along the bottom. */
function paths(values: (number | null)[], max: number, size: number) {
	const visible = values.slice(-size)
	const offset = size - visible.length
	let line = ""
	let first = -1
	let last = -1
	visible.forEach((v, i) => {
		if (v == null) return
		const x = xOf(offset, i, size)
		line += `${line ? "L" : "M"}${x.toFixed(1)},${yOf(v, max).toFixed(1)}`
		if (first < 0) first = i
		last = i
	})
	if (!line) return { line: "", area: "" }
	const area = `${line}L${xOf(offset, last, size).toFixed(1)},${H}L${xOf(offset, first, size).toFixed(1)},${H}Z`
	return { line, area }
}

interface ChartProps {
	series: Series[]
	/** The value at the top of the graph. */
	max: number
	/** Counts every sample so far; moves the vertical grid with time. */
	phase?: number
	size?: number
	grid?: boolean
	className?: string
	label?: string
}

/** A filled line graph of the last minute, newest on the right. */
export function AreaChart({ series: all, max, phase = 0, size = WINDOW, grid = true, className, label }: ChartProps) {
	const id = useId()

	// A gridline sits where a sample multiple of ten was taken, so the grid scrolls as time passes.
	const verticals: number[] = []
	if (grid) {
		for (let m = Math.floor((phase - 1) / 10); m >= 0; m--) {
			const ago = phase - 1 - m * 10
			if (ago > size - 1) break
			verticals.push(W - (ago / (size - 1)) * W)
		}
	}

	return (
		<svg
			viewBox={`0 0 ${W} ${H}`}
			preserveAspectRatio="none"
			role="img"
			aria-label={label}
			className={cn("block w-full overflow-hidden rounded-lg bg-black/25 ring-1 ring-white/10", className)}
		>
			<defs>
				{all.map((s, i) => (
					<linearGradient key={i} id={`${id}-${i}`} x1="0" y1="0" x2="0" y2="1">
						<stop offset="0%" stopColor={s.color} stopOpacity="0.35" />
						<stop offset="100%" stopColor={s.color} stopOpacity="0.02" />
					</linearGradient>
				))}
			</defs>
			{grid && (
				<g stroke="white" strokeOpacity="0.07" strokeWidth="1" vectorEffect="non-scaling-stroke">
					{[0.25, 0.5, 0.75].map((f) => (
						<line key={f} x1="0" x2={W} y1={H * f} y2={H * f} vectorEffect="non-scaling-stroke" />
					))}
					{verticals.map((x, i) => (
						<line key={i} x1={x} x2={x} y1="0" y2={H} vectorEffect="non-scaling-stroke" />
					))}
				</g>
			)}
			{all.map((s, i) => {
				const p = paths(s.values, max, size)
				return (
					<g key={i}>
						{p.area && <path d={p.area} fill={`url(#${id}-${i})`} />}
						{p.line && (
							<path
								d={p.line}
								fill="none"
								stroke={s.color}
								strokeWidth="1.6"
								strokeLinejoin="round"
								vectorEffect="non-scaling-stroke"
							/>
						)}
					</g>
				)
			})}
		</svg>
	)
}

/** A small graph for a list tile: no grid, one series. */
export function Sparkline({ values, max, color, className }: { values: (number | null)[]; max: number; color: string; className?: string }) {
	return <AreaChart series={[{ values, color }]} max={max} grid={false} className={cn("h-10 w-24 rounded-md", className)} />
}
