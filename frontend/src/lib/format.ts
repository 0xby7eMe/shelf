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