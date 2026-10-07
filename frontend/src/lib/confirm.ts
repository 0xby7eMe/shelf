import { useSyncExternalStore } from "react"

// A promise-based replacement for window.confirm, shown by <ConfirmDialog />.
//
//   if (await confirm({ title: "Uninstall?", confirmLabel: "Uninstall" })) ...

export interface ConfirmOptions {
	title: string
	description?: string
	confirmLabel?: string
	cancelLabel?: string
	/** Red confirm button, for things that can't be undone. */
	destructive?: boolean
	/** Shows the game's poster and name above the text. */
	game?: { name: string; cover?: string }
}

interface State {
	open: boolean
	options: ConfirmOptions | null
	resolve: ((ok: boolean) => void) | null
}

let state: State = { open: false, options: null, resolve: null }
const listeners = new Set<() => void>()

function set(next: State) {
	state = next
	listeners.forEach((l) => l())
}

export function confirm(options: ConfirmOptions): Promise<boolean> {
	// A second request while one is open cancels the first.
	state.resolve?.(false)
	return new Promise((resolve) => set({ open: true, options, resolve }))
}

/** Closes the dialog with an answer. The options stay so it can animate out. */
export function answer(ok: boolean) {
	state.resolve?.(ok)
	set({ ...state, open: false, resolve: null })
}

export function useConfirmState(): State {
	return useSyncExternalStore(
		(cb) => {
			listeners.add(cb)
			return () => listeners.delete(cb)
		},
		() => state
	)
}
