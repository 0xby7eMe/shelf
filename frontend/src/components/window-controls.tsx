import { Minus, Plus, X } from "lucide-react"

import { Quit, WindowMinimise, WindowToggleMaximise } from "../../wailsjs/runtime/runtime"
import { cn } from "@/lib/utils"

function Dot({
	label,
	onClick,
	hover,
	children,
}: {
	label: string
	onClick: () => void
	hover: string
	children: React.ReactNode
}) {
	return (
		<button
			data-nav-skip
			onClick={onClick}
			aria-label={label}
			className={cn(
				"grid size-3 place-items-center rounded-full bg-white/15 text-black/0 outline-none transition duration-200",
				"hover:text-black/70 focus-visible:ring-2 focus-visible:ring-white/50",
				hover
			)}
		>
			{children}
		</button>
	)
}

export function WindowControls() {
	return (
		<div className="ml-auto flex items-center gap-2 pl-3">
			<Dot label="Minimize" onClick={WindowMinimise} hover="hover:bg-amber-400">
				<Minus className="size-2" strokeWidth={3.5} />
			</Dot>
			<Dot label="Maximize" onClick={WindowToggleMaximise} hover="hover:bg-emerald-400">
				<Plus className="size-2" strokeWidth={3.5} />
			</Dot>
			<Dot label="Close" onClick={Quit} hover="hover:bg-red-500">
				<X className="size-2" strokeWidth={3.5} />
			</Dot>
		</div>
	)
}