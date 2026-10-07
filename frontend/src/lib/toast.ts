import { useSyncExternalStore } from "react"

export type ToastKind = "success" | "error" | "info"

export interface ToastAction {
	label: string
	onClick: () => void
}

export interface Toast {
	id: number
	kind: ToastKind
	title: string
	description?: string
	action?: ToastAction
}

interface Options {
	description?: string
	action?: ToastAction
	duration?: number
}

const DURATION: Record<ToastKind, number> = { success: 6000, info: 5000, error: 9000 }
const MAX_VISIBLE = 4

let toasts: Toast[] = []
let nextId = 1
const listeners = new Set<() => void>()

function emit() {
	listeners.forEach((l) => l())
}

export function dismiss(id: number) {
	toasts = toasts.filter((t) => t.id !== id)
	emit()
}

function push(kind: ToastKind, title: string, opts: Options = {}) {
	const id = nextId++
	toasts = [...toasts, { id, kind, title, description: opts.description, action: opts.action }].slice(
		-MAX_VISIBLE
	)
	emit()
	setTimeout(() => dismiss(id), opts.duration ?? DURATION[kind])
	return id
}

export const toast = {
	success: (title: string, opts?: Options) => push("success", title, opts),
	error: (title: string, opts?: Options) => push("error", title, opts),
	info: (title: string, opts?: Options) => push("info", title, opts),
}

export function useToasts() {
	return useSyncExternalStore(
		(cb) => {
			listeners.add(cb)
			return () => listeners.delete(cb)
		},
		() => toasts
	)
}
