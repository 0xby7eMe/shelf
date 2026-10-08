import { useEffect, useMemo, useRef, useState } from "react"
import { Download } from "lucide-react"

import { SaveShareCard } from "../../wailsjs/go/main/App"
import { library } from "../../wailsjs/go/models"
import { Button } from "@/components/ui/button"
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogTitle,
} from "@/components/ui/dialog"
import { buildCardData, drawCard, loadCovers } from "@/lib/share-card"
import { toast } from "@/lib/toast"

interface Props {
	open: boolean
	games: library.Game[]
	favorites: Set<string>
	onOpenChange: (open: boolean) => void
}

export function ShareCardDialog({ open, games, favorites, onOpenChange }: Props) {
	const canvasRef = useRef<HTMLCanvasElement>(null)
	const [ready, setReady] = useState(false)
	const [saving, setSaving] = useState(false)
	const data = useMemo(() => buildCardData(games, favorites), [games, favorites])

	useEffect(() => {
		if (!open) return
		let alive = true
		setReady(false)
		// The dialog mounts its content a moment after opening.
		const frame = requestAnimationFrame(async () => {
			const canvas = canvasRef.current
			if (!canvas) return
			const covers = await loadCovers(data.top)
			if (!alive) return
			await drawCard(canvas, data, covers)
			if (alive) setReady(true)
		})
		return () => {
			alive = false
			cancelAnimationFrame(frame)
		}
	}, [open, data])

	async function save() {
		const canvas = canvasRef.current
		if (!canvas) return
		setSaving(true)
		try {
			const path = await SaveShareCard(canvas.toDataURL("image/png"))
			if (path) toast.success("Card saved", { description: path })
		} catch (e) {
			toast.error("Couldn't save the card", { description: String(e) })
		} finally {
			setSaving(false)
		}
	}

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent
				style={{
					backdropFilter: "blur(30px)",
					WebkitBackdropFilter: "blur(30px)",
					backgroundColor: "rgba(10, 10, 10, 0.6)",
				}}
				className="gap-5 border-white/10 p-6 sm:max-w-3xl"
			>
				<div>
					<DialogTitle className="text-[11px] font-medium tracking-[0.25em] text-muted-foreground uppercase">
						Share card
					</DialogTitle>
					<DialogDescription className="mt-2 text-sm text-muted-foreground">
						An image of your library to post anywhere. It holds only game titles, play time and counts.
					</DialogDescription>
				</div>

				<canvas
					ref={canvasRef}
					className="aspect-[5/3] w-full rounded-2xl ring-1 ring-white/10"
					style={{ opacity: ready ? 1 : 0.4, transition: "opacity 150ms" }}
				/>

				<div className="flex justify-end">
					<Button onClick={save} disabled={!ready || saving}>
						<Download />
						Save as PNG
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	)
}
