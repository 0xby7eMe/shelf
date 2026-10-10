import { useCallback, useEffect, useState } from "react"

import { GetGogAccount, GetSteamAPI, GetUbisoftStatus } from "../../../wailsjs/go/main/App"
import { epic, main } from "../../../wailsjs/go/models"
import { EventsOn } from "../../../wailsjs/runtime/runtime"
import { EpicSection } from "@/components/settings/epic-section"
import { GogSection } from "@/components/settings/gog-section"
import { SectionHeading } from "@/components/settings/ui"
import { SteamSection } from "@/components/settings/steam-section"
import { UbisoftSection } from "@/components/settings/ubisoft-section"
import { integrationsFor, setIntegrationTab, useIntegrationTab, type Integration } from "@/lib/integrations"
import { usePlatform } from "@/lib/platform"
import { cn } from "@/lib/utils"

interface Props {
	account: epic.Account | null
	onAccountChange: () => void
}

// One page for every store Shelf connects to. Each store is a tab.
export function IntegrationsSection({ account, onAccountChange }: Props) {
	const tab = useIntegrationTab()
	const { windowsGames } = usePlatform()
	const [ubisoft, setUbisoft] = useState<epic.UbisoftStatus | null>(null)
	const [gog, setGog] = useState<epic.GogAccount | null>(null)

	// Asking Steam is a few web requests, so it happens once when the page opens
	// and after the key changes, not with every library change.
	const [steam, setSteam] = useState<main.SteamAPIStatus | null>(null)
	useEffect(() => {
		GetSteamAPI().then(setSteam).catch(() => {})
	}, [])

	const reloadGog = useCallback(() => GetGogAccount().then(setGog).catch(() => {}), [])
	const reload = useCallback(() => {
		GetUbisoftStatus().then(setUbisoft).catch(() => {})
		reloadGog()
	}, [reloadGog])
	useEffect(() => {
		reload()
		const off = EventsOn("library:changed", reload)
		const t = setInterval(reload, 5000)
		return () => {
			off()
			clearInterval(t)
		}
	}, [reload])

	// Whether a store has an account connected, shown as a dot on its tab.
	const connected: Record<Integration, boolean> = {
		steam: !!steam?.overview,
		epic: !!account?.loggedIn,
		gog: !!gog?.loggedIn,
		ubisoft: !!ubisoft?.signedIn,
	}

	return (
		<div className="space-y-6">
			<SectionHeading title="Integrations" hint="Connect the stores your games come from." />

			<div role="tablist" aria-label="Stores" className="flex flex-wrap gap-1 rounded-full bg-white/5 p-1 ring-1 ring-white/5">
				{integrationsFor(windowsGames).map(({ id, label }) => (
					<button
						key={id}
						role="tab"
						aria-selected={tab === id}
						onClick={() => setIntegrationTab(id)}
						className={cn(
							"flex h-9 items-center gap-2 rounded-full px-4 text-xs transition",
							tab === id ? "bg-white/10 text-white" : "text-white/55 hover:text-white"
						)}
					>
						{label}
						<span
							aria-label={connected[id] ? "Connected" : "Not connected"}
							title={connected[id] ? "Connected" : "Not connected"}
							className={cn("size-1.5 rounded-full", connected[id] ? "bg-emerald-400" : "bg-white/20")}
						/>
					</button>
				))}
			</div>

			<div key={tab} className="animate-in duration-200 fade-in">
				{tab === "steam" && <SteamSection status={steam} onChange={setSteam} />}
				{tab === "epic" && <EpicSection account={account} onAccountChange={onAccountChange} />}
				{tab === "gog" && <GogSection account={gog} onAccountChange={reloadGog} />}
				{tab === "ubisoft" && windowsGames && <UbisoftSection />}
			</div>
		</div>
	)
}
