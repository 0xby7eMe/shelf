import { ArrowLeft, Download, HardDrive, Gamepad2, Shield, SlidersHorizontal, Store } from "lucide-react"

import { epic, library } from "../../../wailsjs/go/models"
import { WindowControls } from "@/components/window-controls"
import { AdvancedSection } from "@/components/settings/advanced-section"
import { ControllerSection } from "@/components/settings/controller-section"
import { DownloadsSection } from "@/components/settings/downloads-section"
import { EpicSection } from "@/components/settings/epic-section"
import { UbisoftSection } from "@/components/settings/ubisoft-section"
import { StorageSection } from "@/components/settings/storage-section"
import { cn } from "@/lib/utils"

export type SettingsSection = "epic" | "ubisoft" | "downloads" | "storage" | "controller" | "advanced"

export const SECTIONS: { id: SettingsSection; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
	{ id: "epic", label: "Epic Games", icon: Store },
	{ id: "ubisoft", label: "Ubisoft", icon: Shield },
	{ id: "downloads", label: "Downloads", icon: Download },
	{ id: "storage", label: "Storage", icon: HardDrive },
	{ id: "controller", label: "Controller", icon: Gamepad2 },
	{ id: "advanced", label: "Advanced", icon: SlidersHorizontal },
]

interface Props {
	section: SettingsSection
	onSection: (section: SettingsSection) => void
	onBack: () => void
	account: epic.Account | null
	onAccountChange: () => void
	queue: epic.QueueState
	games: library.Game[]
	onSelectGame: (game: library.Game) => void
}

export function SettingsPage({
	section,
	onSection,
	onBack,
	account,
	onAccountChange,
	queue,
	games,
	onSelectGame,
}: Props) {
	return (
		<div
			data-scroll-root
			className="fixed inset-0 z-40 animate-in overflow-y-auto bg-background text-foreground duration-200 fade-in"
		>
			<header className="titlebar sticky top-0 z-30 flex h-16 items-center gap-3 border-b border-white/5 bg-[rgba(10,10,10,0.55)] px-8 backdrop-blur-xl">
				<button
					onClick={onBack}
					className="flex h-9 items-center gap-2 rounded-full bg-white/5 pr-4 pl-3 text-xs text-white/80 ring-1 ring-white/10 transition hover:bg-white/10 hover:text-white"
				>
					<ArrowLeft className="size-3.5" />
					Library
				</button>
				<h1 className="ml-2 text-sm font-medium tracking-[0.25em] uppercase">Settings</h1>
				<div className="ml-auto">
					<WindowControls />
				</div>
			</header>

			<div className="mx-auto flex max-w-4xl flex-col gap-8 px-8 py-10 md:flex-row md:gap-12">
				<nav aria-label="Settings sections" className="flex shrink-0 gap-1 md:sticky md:top-28 md:h-fit md:w-48 md:flex-col">
					{SECTIONS.map(({ id, label, icon: Icon }) => {
						const count = id === "downloads" ? queue.jobs.length : 0
						return (
							<button
								key={id}
								onClick={() => onSection(id)}
								aria-current={section === id ? "page" : undefined}
								className={cn(
									"flex h-10 items-center gap-3 rounded-full px-4 text-sm transition",
									section === id
										? "bg-white/10 text-white"
										: "text-white/55 hover:bg-white/5 hover:text-white"
								)}
							>
								<Icon className="size-4" />
								<span className="flex-1 text-left">{label}</span>
								{count > 0 && (
									<span className="rounded-full bg-white/15 px-1.5 text-[10px] tabular-nums">{count}</span>
								)}
							</button>
						)
					})}
				</nav>

				<main key={section} className="min-w-0 max-w-2xl flex-1 animate-in duration-300 fade-in slide-in-from-bottom-2">
					{section === "epic" && <EpicSection account={account} onAccountChange={onAccountChange} />}
					{section === "ubisoft" && <UbisoftSection />}
					{section === "downloads" && <DownloadsSection queue={queue} games={games} />}
					{section === "storage" && <StorageSection games={games} onSelect={onSelectGame} />}
					{section === "controller" && <ControllerSection />}
					{section === "advanced" && <AdvancedSection />}
				</main>
			</div>
		</div>
	)
}
