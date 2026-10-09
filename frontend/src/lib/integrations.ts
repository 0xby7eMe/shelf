import { useSyncExternalStore } from "react"

// The stores Shelf connects to, as tabs of the Integrations page. Anything that
// needs to send the user to one of them sets the tab before opening settings.
export type Integration = "steam" | "epic" | "gog" | "ubisoft"

export const INTEGRATIONS: { id: Integration; label: string }[] = [
	{ id: "steam", label: "Steam" },
	{ id: "epic", label: "Epic Games" },
	{ id: "gog", label: "GOG" },
	{ id: "ubisoft", label: "Ubisoft" },
]

// The tabs this system has: Ubisoft Connect runs through Proton, so it needs Linux.
export function integrationsFor(proton: boolean) {
	return INTEGRATIONS.filter((i) => i.id !== "ubisoft" || proton)
}

let tab: Integration = "epic"
const listeners = new Set<() => void>()

export function setIntegrationTab(next: Integration) {
	if (tab === next) return
	tab = next
	listeners.forEach((l) => l())
}

export function useIntegrationTab(): Integration {
	return useSyncExternalStore(
		(cb) => {
			listeners.add(cb)
			return () => listeners.delete(cb)
		},
		() => tab
	)
}
