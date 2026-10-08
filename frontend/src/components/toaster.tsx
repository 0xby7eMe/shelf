import { AlertCircle, CheckCircle2, Info, X } from "lucide-react"

import { dismiss, useToasts, type ToastKind } from "@/lib/toast"
import { cn } from "@/lib/utils"

const icons: Record<ToastKind, React.ComponentType<{ className?: string }>> = {
	success: CheckCircle2,
	error: AlertCircle,
	info: Info,
}

const tint: Record<ToastKind, string> = {
	success: "text-emerald-400",
	error: "text-red-400",
	info: "text-white/60",
}

export function Toaster() {
	const toasts = useToasts()

	return (
		<div
			aria-live="polite"
			className="pointer-events-none fixed right-6 bottom-6 z-[100] flex w-[22rem] max-w-[calc(100vw-3rem)] flex-col gap-2"
		>
			{toasts.map((t) => {
				const Icon = icons[t.kind]
				return (
					<div
						key={t.id}
						role={t.kind === "error" ? "alert" : "status"}
						style={{ backdropFilter: "blur(20px) saturate(140%)", WebkitBackdropFilter: "blur(20px) saturate(140%)" }}
						className="pointer-events-auto flex animate-in items-start gap-3 rounded-2xl bg-[rgba(10,10,10,0.7)] p-4 shadow-[0_16px_40px_-12px_rgba(0,0,0,0.8)] ring-1 ring-white/10 duration-300 fade-in slide-in-from-right-4 motion-reduce:animate-none"
					>
						<Icon className={cn("mt-0.5 size-4 shrink-0", tint[t.kind])} />
						<div className="min-w-0 flex-1">
							<p className="text-sm leading-snug font-medium text-white">{t.title}</p>
							{t.description && (
								<p className="mt-0.5 text-xs leading-snug break-words text-white/50">
									{t.description}
								</p>
							)}
							{t.action && (
								<button
									onClick={() => {
										t.action?.onClick()
										dismiss(t.id)
									}}
									className="mt-2.5 rounded-full bg-white px-3.5 py-1 text-xs font-medium text-black transition hover:shadow-[0_0_24px_rgba(255,255,255,0.3)]"
								>
									{t.action.label}
								</button>
							)}
						</div>
						<button
							onClick={() => dismiss(t.id)}
							aria-label="Dismiss"
							className="shrink-0 text-white/40 transition hover:text-white"
						>
							<X className="size-3.5" />
						</button>
					</div>
				)
			})}
		</div>
	)
}
