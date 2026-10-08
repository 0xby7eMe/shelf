import { cn } from "@/lib/utils"

interface Props {
	label: string
	hint?: string
	checked: boolean
	disabled?: boolean
	onChange: (checked: boolean) => void
}

// A labelled on/off switch row.
export function ToggleRow({ label, hint, checked, disabled, onChange }: Props) {
	return (
		<div className={cn("flex items-center justify-between gap-4", disabled && "opacity-40")}>
			<div className="min-w-0">
				<p className="text-sm text-white/90">{label}</p>
				{hint && <p className="text-xs text-white/40">{hint}</p>}
			</div>
			<button
				type="button"
				role="switch"
				aria-checked={checked}
				aria-label={label}
				disabled={disabled}
				onClick={() => onChange(!checked)}
				className={cn(
					"relative h-6 w-10 shrink-0 rounded-full ring-1 ring-white/10 transition-colors duration-200 disabled:pointer-events-none",
					checked ? "bg-solid" : "bg-white/10"
				)}
			>
				<span
					className={cn(
						"absolute top-0.5 left-0.5 size-5 rounded-full transition-transform duration-200",
						checked ? "translate-x-4 bg-solid-foreground" : "bg-white/60"
					)}
				/>
			</button>
		</div>
	)
}
