import { useRef } from "react"
import { Trash2 } from "lucide-react"

import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog"
import { answer, useConfirmState } from "@/lib/confirm"
import { cn } from "@/lib/utils"

// Mount once. Anything can open it with confirm() from lib/confirm.
export function ConfirmDialog() {
	const { open, options } = useConfirmState()
	const cancelRef = useRef<HTMLButtonElement>(null)

	return (
		<Dialog open={open} onOpenChange={(next) => !next && answer(false)}>
			<DialogContent
				showCloseButton={false}
				// Focus starts on Cancel, so a stray Enter or A press can't delete anything.
				initialFocus={cancelRef}
				style={{
					backdropFilter: "blur(30px)",
					WebkitBackdropFilter: "blur(30px)",
					backgroundColor: "rgba(10, 10, 10, 0.7)",
				}}
				className="gap-6 border-white/10 p-6 sm:max-w-sm"
			>
				{options && (
					<>
						<div className="flex items-start gap-4">
							{options.game?.cover ? (
								<img
									src={options.game.cover}
									alt=""
									className="h-20 w-14 shrink-0 rounded-lg object-cover ring-1 ring-white/15"
								/>
							) : (
								<span
									className={cn(
										"grid size-11 shrink-0 place-items-center rounded-full ring-1",
										options.destructive
											? "bg-red-500/10 text-red-300 ring-red-400/20"
											: "bg-white/5 text-white/70 ring-white/10"
									)}
								>
									<Trash2 className="size-4" />
								</span>
							)}
							<div className="min-w-0">
								<DialogTitle className="text-base leading-snug font-semibold text-balance">
									{options.title}
								</DialogTitle>
								{options.game && (
									<p className="mt-0.5 truncate text-xs text-white/45">{options.game.name}</p>
								)}
								<DialogDescription className="mt-2 text-sm leading-relaxed text-white/55">
									{options.description}
								</DialogDescription>
							</div>
						</div>

						<div className="flex justify-end gap-2">
							<button
								ref={cancelRef}
								onClick={() => answer(false)}
								className="h-10 rounded-full bg-white/5 px-5 text-sm text-white/80 ring-1 ring-white/10 transition hover:bg-white/10 hover:text-white"
							>
								{options.cancelLabel ?? "Cancel"}
							</button>
							<button
								onClick={() => answer(true)}
								className={cn(
									"h-10 rounded-full px-5 text-sm font-medium transition",
									options.destructive
										? "bg-red-500 text-white hover:bg-red-400 hover:shadow-[0_0_32px_rgba(239,68,68,0.45)]"
										: "bg-solid text-solid-foreground accent-glow"
								)}
							>
								{options.confirmLabel ?? "Confirm"}
							</button>
						</div>
					</>
				)}
			</DialogContent>
		</Dialog>
	)
}
