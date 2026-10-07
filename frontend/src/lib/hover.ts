import { useEffect } from "react"

// Card hover is driven by a data-hover attribute instead of CSS :hover.
//
// :hover is re-evaluated whenever content moves under a still cursor, so
// scrolling made every card that passed the pointer lift and drop. Here a card
// only counts as hovered when the mouse actually moves over it, and the
// hover is dropped as soon as anything scrolls. The gamepad uses the same
// attribute for the focused card, so both inputs look identical.

let current: Element | null = null

export function setHover(el: Element | null) {
	if (current === el) return
	current?.removeAttribute("data-hover")
	current = el
	current?.setAttribute("data-hover", "")
	// Without a pointer the highlight sits at the top centre, not where the last mouse hover left it.
	const poster = current?.querySelector<HTMLElement>("[data-poster]")
	poster?.style.removeProperty("--x")
	poster?.style.removeProperty("--y")
}

// The highlight follows the pointer. Positions are applied once per frame, and
// only to the hovered card, so moving the mouse never forces extra layout work.
let frame = 0
let pointer = { x: 0, y: 0 }

function moveHighlight() {
	frame = 0
	const poster = current?.querySelector<HTMLElement>("[data-poster]")
	if (!poster) return
	const r = poster.getBoundingClientRect()
	poster.style.setProperty("--x", `${pointer.x - r.left}px`)
	poster.style.setProperty("--y", `${pointer.y - r.top}px`)
}

export function useStableHover() {
	useEffect(() => {
		const onMove = (e: PointerEvent) => {
			if (e.pointerType === "touch") return
			setHover((e.target as Element | null)?.closest?.(".group") ?? null)
			if (current) {
				pointer = { x: e.clientX, y: e.clientY }
				if (!frame) frame = requestAnimationFrame(moveHighlight)
			}
		}
		const clear = () => setHover(null)
		// A controller scrolls the page itself to follow its focus, and the
		// focused card must keep its lift while that happens.
		const onScroll = () => {
			if (document.documentElement.dataset.input !== "pad") clear()
		}

		window.addEventListener("pointermove", onMove, { passive: true })
		// Scroll events don't bubble, so listen in the capture phase to catch every scroller.
		window.addEventListener("scroll", onScroll, { capture: true, passive: true })
		document.documentElement.addEventListener("pointerleave", clear)
		return () => {
			window.removeEventListener("pointermove", onMove)
			window.removeEventListener("scroll", onScroll, { capture: true })
			document.documentElement.removeEventListener("pointerleave", clear)
			cancelAnimationFrame(frame)
			frame = 0
			clear()
		}
	}, [])
}
