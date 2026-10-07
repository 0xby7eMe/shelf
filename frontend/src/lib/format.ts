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

/** A transfer rate in bytes per second, e.g. "3.2 MB/s". */
export function formatRate(bytesPerSec: number): string {
	return `${formatBytes(bytesPerSec)}/s`
}

/** A network speed in bits per second, as network tools show it, e.g. "94.1 Mbps". */
export function formatBitrate(bytesPerSec: number): string {
	const bits = bytesPerSec * 8
	const units = ["bps", "Kbps", "Mbps", "Gbps"]
	let v = bits
	let i = 0
	while (v >= 1000 && i < units.length - 1) {
		v /= 1000
		i++
	}
	return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

/** Uptime as days:hours:minutes:seconds, e.g. "0:02:14:09". */
export function formatUptime(totalSeconds: number): string {
	const s = Math.max(0, Math.floor(totalSeconds))
	const d = Math.floor(s / 86400)
	const h = Math.floor((s % 86400) / 3600)
	const m = Math.floor((s % 3600) / 60)
	const pad = (n: number) => String(n).padStart(2, "0")
	return `${d}:${pad(h)}:${pad(m)}:${pad(s % 60)}`
}

export function formatGHz(mhz: number): string {
	return `${(mhz / 1000).toFixed(2)} GHz`
}

export function formatPercent(v: number): string {
	return `${v >= 10 ? Math.round(v) : v.toFixed(1)}%`
}
