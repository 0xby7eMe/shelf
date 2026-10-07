export function formatPlaytime(minutes: number): string {
	if (minutes <= 0) return "Not played"
	if (minutes < 60) return `${minutes} min`
	const h = minutes / 60
	return `${h >= 100 ? Math.round(h) : h.toFixed(1)} h`
}

const dateFmt = new Intl.DateTimeFormat(undefined, {
	year: "numeric",
	month: "short",
	day: "numeric",
})

export function formatLastPlayed(unix: number): string {
	return unix > 0 ? dateFmt.format(new Date(unix * 1000)) : "Never"
}

const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" })

export function relativeTime(unix: number): string {
	if (unix <= 0) return "Never"
	const days = Math.round((unix * 1000 - Date.now()) / 86_400_000)
	if (Math.abs(days) < 1) return "Today"
	if (Math.abs(days) < 30) return rtf.format(days, "day")
	if (Math.abs(days) < 365) return rtf.format(Math.round(days / 30), "month")
	return rtf.format(Math.round(days / 365), "year")
}

export function formatElapsed(totalSeconds: number): string {
	const h = Math.floor(totalSeconds / 3600)
	const m = Math.floor((totalSeconds % 3600) / 60)
	const s = totalSeconds % 60
	const mm = h > 0 ? String(m).padStart(2, "0") : String(m)
	return `${h > 0 ? `${h}:` : ""}${mm}:${String(s).padStart(2, "0")}`
}

export function formatDuration(minutes: number): string {
	if (minutes < 1) return "0 min"
	if (minutes < 60) return `${minutes} min`
	const h = Math.floor(minutes / 60)
	const m = minutes % 60
	return m ? `${h} h ${m} min` : `${h} h`
}

export function formatBytes(bytes: number): string {
	if (bytes <= 0) return "0 B"
	const units = ["B", "KB", "MB", "GB", "TB"]
	const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1000)))
	const v = bytes / 1000 ** i
	return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}
