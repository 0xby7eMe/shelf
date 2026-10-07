import { useSyncExternalStore } from "react"

// The stores Shelf connects to, as tabs of the Integrations page. Anything that
// needs to send the user to one of them sets the tab before opening settings.
export type Integration = "epic" | "ubisoft"

export const INTEGRATIONS: { id: Integration; label: string }[] = [
	{ id: "epic", label: "Epic Games" },
	{ id: "ubisoft", label: "Ubisoft" },
]

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
