import { cn } from "@/lib/utils"

export function LiveDot({ className }: { className?: string }) {
	return (
		<span className={cn("relative flex size-1.5 shrink-0", className)}>
			<span className="absolute inline-flex size-full animate-ping rounded-full bg-emerald-400/70 motion-reduce:animate-none" />
			<span className="relative inline-flex size-full rounded-full bg-emerald-400" />
		</span>
	)
}