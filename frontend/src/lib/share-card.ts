import { library } from "../../wailsjs/go/models"
import { GetCardCover } from "../../wailsjs/go/main/App"

export const CARD_WIDTH = 1200
export const CARD_HEIGHT = 720
const TOP_COUNT = 5

const STORES: { id: string; label: string; color: string }[] = [
	{ id: "steam", label: "Steam", color: "#66c0f4" },
	{ id: "epic", label: "Epic", color: "#e5e5e5" },
	{ id: "gog", label: "GOG", color: "#b06ce8" },
	{ id: "ubisoft", label: "Ubisoft", color: "#4a90ff" },
]

export interface CardData {
	total: number
	installed: number
	favorites: number
	minutes: number
	stores: { label: string; color: string; count: number }[]
	top: library.Game[]
}

// Everything on the card is a count or a title: no paths, accounts or dates.
export function buildCardData(games: library.Game[], favorites: Set<string>): CardData {
	return {
		total: games.length,
		installed: games.filter((g) => g.installed).length,
		favorites: games.filter((g) => favorites.has(g.externalId)).length,
		minutes: games.reduce((sum, g) => sum + g.playtimeMinutes, 0),
		stores: STORES.map((s) => ({
			label: s.label,
			color: s.color,
			count: games.filter((g) => g.source === s.id).length,
		})).filter((s) => s.count > 0),
		top: games
			.filter((g) => g.playtimeMinutes > 0)
			.sort((a, b) => b.playtimeMinutes - a.playtimeMinutes)
			.slice(0, TOP_COUNT),
	}
}

function loadImage(src: string): Promise<HTMLImageElement | null> {
	return new Promise((resolve) => {
		if (!src) return resolve(null)
		const img = new Image()
		img.onload = () => resolve(img)
		img.onerror = () => resolve(null)
		img.src = src
	})
}

// Covers arrive as data URLs through the backend, so the canvas stays exportable.
export function loadCovers(games: library.Game[]): Promise<(HTMLImageElement | null)[]> {
	return Promise.all(
		games.map((g) =>
			GetCardCover(g.id)
				.then(loadImage)
				.catch(() => null)
		)
	)
}

const FONT = "'Geist Variable', system-ui, sans-serif"

function hours(minutes: number): string {
	const h = minutes / 60
	return h >= 10 ? `${Math.round(h).toLocaleString()} h` : `${h.toFixed(1)} h`
}

function roundRect(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number) {
	ctx.beginPath()
	ctx.roundRect(x, y, w, h, r)
}

// Draws img to fill the box, like object-fit: cover.
function drawCover(ctx: CanvasRenderingContext2D, img: HTMLImageElement, x: number, y: number, w: number, h: number) {
	const scale = Math.max(w / img.width, h / img.height)
	const sw = w / scale
	const sh = h / scale
	ctx.drawImage(img, (img.width - sw) / 2, (img.height - sh) / 2, sw, sh, x, y, w, h)
}

function fit(ctx: CanvasRenderingContext2D, text: string, maxWidth: number): string {
	if (ctx.measureText(text).width <= maxWidth) return text
	let s = text
	while (s.length > 1 && ctx.measureText(s + "…").width > maxWidth) s = s.slice(0, -1)
	return s.trimEnd() + "…"
}

function label(ctx: CanvasRenderingContext2D, text: string, x: number, y: number) {
	ctx.font = `600 13px ${FONT}`
	ctx.letterSpacing = "3px"
	ctx.fillStyle = "rgba(255,255,255,0.45)"
	ctx.fillText(text.toUpperCase(), x, y)
	ctx.letterSpacing = "0px"
}

export async function drawCard(
	canvas: HTMLCanvasElement,
	data: CardData,
	covers: (HTMLImageElement | null)[]
) {
	await document.fonts.load(`600 20px ${FONT}`).catch(() => {})

	canvas.width = CARD_WIDTH
	canvas.height = CARD_HEIGHT
	const ctx = canvas.getContext("2d")
	if (!ctx) return
	const pad = 64

	ctx.fillStyle = "#0a0a0a"
	ctx.fillRect(0, 0, CARD_WIDTH, CARD_HEIGHT)

	// A soft glow from the most played game: its cover shrunk to a few pixels
	// and scaled back up, which blurs without needing canvas filters.
	const lead = covers.find((c) => c)
	if (lead) {
		const tiny = document.createElement("canvas")
		tiny.width = 24
		tiny.height = 14
		const tctx = tiny.getContext("2d")
		if (tctx) {
			drawCover(tctx, lead, 0, 0, tiny.width, tiny.height)
			ctx.imageSmoothingQuality = "high"
			ctx.globalAlpha = 0.5
			ctx.drawImage(tiny, 0, 0, CARD_WIDTH, CARD_HEIGHT)
			ctx.globalAlpha = 1
		}
	}
	const shade = ctx.createLinearGradient(0, 0, 0, CARD_HEIGHT)
	shade.addColorStop(0, "rgba(10,10,10,0.55)")
	shade.addColorStop(1, "rgba(10,10,10,0.92)")
	ctx.fillStyle = shade
	ctx.fillRect(0, 0, CARD_WIDTH, CARD_HEIGHT)

	ctx.textBaseline = "alphabetic"

	label(ctx, "Shelf  ·  my library", pad, pad + 10)

	// Hours played, the headline.
	ctx.fillStyle = "#fff"
	ctx.font = `700 104px ${FONT}`
	const big = data.minutes > 0 ? hours(data.minutes) : "0 h"
	ctx.fillText(big, pad - 4, pad + 140)
	ctx.font = `400 22px ${FONT}`
	ctx.fillStyle = "rgba(255,255,255,0.6)"
	ctx.fillText(`played across ${data.total.toLocaleString()} games`, pad, pad + 182)

	// Store split.
	let sx = pad
	ctx.font = `500 17px ${FONT}`
	for (const s of data.stores) {
		ctx.fillStyle = s.color
		ctx.beginPath()
		ctx.arc(sx + 5, pad + 223, 5, 0, Math.PI * 2)
		ctx.fill()
		ctx.fillStyle = "rgba(255,255,255,0.8)"
		const text = `${s.label} ${s.count}`
		ctx.fillText(text, sx + 18, pad + 229)
		sx += 18 + ctx.measureText(text).width + 28
	}

	// Stat tiles on the right.
	const tiles = [
		{ label: "Games", value: data.total.toLocaleString() },
		{ label: "Installed", value: data.installed.toLocaleString() },
		{ label: "Favorites", value: data.favorites.toLocaleString() },
	]
	const tileW = 168
	const tileGap = 16
	const tilesX = CARD_WIDTH - pad - (tileW * tiles.length + tileGap * (tiles.length - 1))
	tiles.forEach((t, i) => {
		const x = tilesX + i * (tileW + tileGap)
		const y = pad + 36
		roundRect(ctx, x, y, tileW, 132, 20)
		ctx.fillStyle = "rgba(255,255,255,0.06)"
		ctx.fill()
		ctx.strokeStyle = "rgba(255,255,255,0.08)"
		ctx.lineWidth = 1
		ctx.stroke()
		ctx.fillStyle = "#fff"
		ctx.font = `700 46px ${FONT}`
		ctx.fillText(t.value, x + 22, y + 70)
		ctx.fillStyle = "rgba(255,255,255,0.5)"
		ctx.font = `500 15px ${FONT}`
		ctx.fillText(t.label, x + 22, y + 102)
	})

	// Most played.
	const rowY = 340
	label(ctx, "Most played", pad, rowY)

	if (data.top.length === 0) {
		ctx.fillStyle = "rgba(255,255,255,0.5)"
		ctx.font = `400 20px ${FONT}`
		ctx.fillText("Play something and it shows up here.", pad, rowY + 50)
		return
	}

	const gap = 24
	const posterW = Math.floor((CARD_WIDTH - pad * 2 - gap * (TOP_COUNT - 1)) / TOP_COUNT)
	const posterH = Math.round(posterW * 1.4)
	const posterY = rowY + 24
	data.top.forEach((g, i) => {
		const x = pad + i * (posterW + gap)
		ctx.save()
		roundRect(ctx, x, posterY, posterW, posterH, 14)
		ctx.clip()
		const img = covers[i]
		if (img) {
			drawCover(ctx, img, x, posterY, posterW, posterH)
		} else {
			ctx.fillStyle = "#1c1c20"
			ctx.fillRect(x, posterY, posterW, posterH)
		}
		ctx.restore()
		roundRect(ctx, x + 0.5, posterY + 0.5, posterW - 1, posterH - 1, 14)
		ctx.strokeStyle = "rgba(255,255,255,0.14)"
		ctx.lineWidth = 1
		ctx.stroke()

		// Rank badge.
		ctx.fillStyle = "rgba(10,10,10,0.75)"
		ctx.beginPath()
		ctx.arc(x + 22, posterY + 22, 14, 0, Math.PI * 2)
		ctx.fill()
		ctx.fillStyle = "#fff"
		ctx.font = `700 15px ${FONT}`
		ctx.textAlign = "center"
		ctx.fillText(String(i + 1), x + 22, posterY + 27)
		ctx.textAlign = "left"

		ctx.fillStyle = "#fff"
		ctx.font = `600 16px ${FONT}`
		ctx.fillText(fit(ctx, g.name, posterW), x, posterY + posterH + 26)
		ctx.fillStyle = "rgba(255,255,255,0.5)"
		ctx.font = `400 14px ${FONT}`
		ctx.fillText(hours(g.playtimeMinutes), x, posterY + posterH + 46)
	})
}
