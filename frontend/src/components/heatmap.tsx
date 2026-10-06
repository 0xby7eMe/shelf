import { library } from "../../wailsjs/go/models"
import { formatDuration } from "@/lib/format"
import { cn } from "@/lib/utils"

const shades = ["bg-white/[0.05]", "bg-white/20", "bg-white/35", "bg-white/55", "bg-white/85"]

function level(minutes: number) {
	if (minutes <= 0) return 0
	if (minutes < 30) return 1
	if (minutes < 90) return 2
	if (minutes < 180) return 3
	return 4
}

function parseDay(s: string) {
	const [y, m, d] = s.split("-").map(Number)
	return new Date(y, m - 1, d)
}

const dayFmt = new Intl.DateTimeFormat(undefined, {
	weekday: "short",
	month: "short",
	day: "numeric",
})
const monthFmt = new Intl.DateTimeFormat(undefined, { month: "short" })
const weekdayFmt = new Intl.DateTimeFormat(undefined, { weekday: "short" })

const weekday = (i: number) => weekdayFmt.format(new Date(2026, 0, 5 + i))

export function Heatmap({ days }: { days: library.DayStat[] }) {
	const weeks: library.DayStat[][] = []
	for (let i = 0; i < days.length; i += 7) weeks.push(days.slice(i, i + 7))

	const monthLabel = (i: number) => {
		const month = parseDay(weeks[i][0].date).getMonth()
		if (i > 0 && month === parseDay(weeks[i - 1][0].date).getMonth()) return ""
		return monthFmt.format(parseDay(weeks[i][0].date))
	}

	const today = days[days.length - 1]?.date

	return (
		<div>
			<div className="flex gap-2">
				<div className="mt-5 flex flex-col gap-1 text-[10px] leading-[14px] text-white/30">
					{Array.from({ length: 7 }, (_, i) => (
						<span key={i} className="h-3.5">
							{i % 2 === 0 ? weekday(i) : ""}
						</span>
					))}
				</div>

				<div>
					<div className="mb-1.5 flex h-3.5 gap-1 text-[10px] leading-[14px] text-white/30">
						{weeks.map((_, i) => (
							<div key={i} className="w-3.5 shrink-0">
								<span className="whitespace-nowrap">{monthLabel(i)}</span>
							</div>
						))}
					</div>

					<div className="flex gap-1">
						{weeks.map((w, i) => (
							<div key={i} className="flex flex-col gap-1">
								{w.map((d) => (
									<div
										key={d.date}
										title={`${dayFmt.format(parseDay(d.date))} · ${
											d.minutes > 0 ? formatDuration(d.minutes) : "No play"
										}`}
										className={cn(
											"size-3.5 rounded-[4px]",
											shades[level(d.minutes)],
											d.date === today && "ring-1 ring-white/40"
										)}
									/>
								))}
							</div>
						))}
					</div>
				</div>
			</div>

			<div className="mt-3 flex items-center justify-end gap-1.5 text-[10px] text-white/30">
				Less
				{shades.map((s) => (
					<span key={s} className={cn("size-2.5 rounded-[3px]", s)} />
				))}
				More
			</div>
		</div>
	)
}