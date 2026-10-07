import { cn } from "@/lib/utils"

// Small building blocks shared by the settings sections.

export function SectionHeading({ title, hint }: { title: string; hint?: string }) {
	return (
		<div className="mb-5">
			<h2 className="text-lg font-semibold tracking-tight">{title}</h2>
			{hint && <p className="mt-1 text-sm text-white/45">{hint}</p>}
		</div>
	)
}

export function Panel({ children, className }: { children: React.ReactNode; className?: string }) {
	return (
		<div className={cn("space-y-5 rounded-2xl bg-white/[0.04] p-5 ring-1 ring-white/[0.06]", className)}>
			{children}
		</div>
	)
}

export function Label({ children }: { children: React.ReactNode }) {
	return <p className="text-[11px] tracking-wide text-white/45 uppercase">{children}</p>
}

const base =
	"inline-flex h-10 items-center justify-center gap-2 rounded-full px-5 text-xs font-medium ring-1 transition disabled:pointer-events-none disabled:opacity-40"

export function PillButton({
	variant = "soft",
	className,
	...props
}: React.ComponentProps<"button"> & { variant?: "soft" | "solid" | "danger" }) {
	return (
		<button
			{...props}
			className={cn(
				base,
				variant === "solid" &&
					"bg-white text-black ring-transparent hover:shadow-[0_0_40px_rgba(255,255,255,0.3)]",
				variant === "soft" && "bg-white/5 text-white/80 ring-white/10 hover:bg-white/10 hover:text-white",
				variant === "danger" && "bg-red-500/10 text-red-300 ring-red-400/20 hover:bg-red-500/20",
				className
			)}
		/>
	)
}

export function IconButton({
	label,
	className,
	...props
}: React.ComponentProps<"button"> & { label: string }) {
	return (
		<button
			{...props}
			title={label}
			aria-label={label}
			className={cn(
				"grid size-9 shrink-0 place-items-center rounded-full bg-white/5 text-white/70 ring-1 ring-white/10 transition hover:bg-white/10 hover:text-white disabled:pointer-events-none disabled:opacity-30",
				className
			)}
		/>
	)
}

export function Bar({ value, className }: { value: number; className?: string }) {
	return (
		<div className={cn("h-1.5 overflow-hidden rounded-full bg-white/10", className)}>
			<div
				className="h-full rounded-full bg-white/70 transition-[width] duration-500"
				style={{ width: `${Math.min(100, Math.max(0, value))}%` }}
			/>
		</div>
	)
}

export const inputClass =
	"h-10 rounded-full border-0 bg-white/5 px-4 text-sm shadow-none ring-1 ring-white/5"
export const triggerClass =
	"h-10 w-full rounded-full border-0 bg-white/5 px-4 text-sm shadow-none ring-1 ring-white/5"
export const selectContentClass = "border-white/10 bg-popover/80 backdrop-blur-xl"
