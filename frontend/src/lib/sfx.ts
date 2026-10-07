import { getPrefs } from "@/lib/prefs"

// Interface sounds, synthesized so the app ships no audio files. They are soft
// sine blips: a tick for moving, a rising pair for select, a falling pair for back.

let ctx: AudioContext | null = null

function audio(): AudioContext | null {
	try {
		if (!ctx) {
			const Ctor = window.AudioContext ?? (window as any).webkitAudioContext
			if (!Ctor) return null
			ctx = new Ctor()
		}
		if (ctx.state === "suspended") void ctx.resume()
		return ctx
	} catch {
		return null
	}
}

// Browsers only start audio after a user gesture; any of these unlocks it.
if (typeof window !== "undefined") {
	for (const ev of ["pointerdown", "keydown", "gamepadconnected"]) {
		window.addEventListener(ev, () => audio(), { once: true, passive: true })
	}
}

interface Tone {
	from: number
	to?: number
	at?: number // seconds from now
	len: number
	gain?: number
	type?: OscillatorType
}

function play(tones: Tone[]) {
	const { sounds, volume } = getPrefs()
	if (!sounds || volume <= 0) return
	const ac = audio()
	if (!ac) return

	const master = (volume / 100) * 0.22
	for (const t of tones) {
		const start = ac.currentTime + (t.at ?? 0)
		const osc = ac.createOscillator()
		const amp = ac.createGain()
		osc.type = t.type ?? "sine"
		osc.frequency.setValueAtTime(t.from, start)
		if (t.to) osc.frequency.exponentialRampToValueAtTime(t.to, start + t.len)

		// Quick attack and an exponential tail avoid clicks.
		const peak = master * (t.gain ?? 1)
		amp.gain.setValueAtTime(0.0001, start)
		amp.gain.linearRampToValueAtTime(peak, start + 0.006)
		amp.gain.exponentialRampToValueAtTime(0.0001, start + t.len)

		osc.connect(amp).connect(ac.destination)
		osc.start(start)
		osc.stop(start + t.len + 0.02)
	}
}

export const sfx = {
	move: () => play([{ from: 620, to: 700, len: 0.05, gain: 0.55 }]),
	confirm: () =>
		play([
			{ from: 523, len: 0.09 },
			{ from: 784, len: 0.14, at: 0.07 },
		]),
	back: () =>
		play([
			{ from: 659, len: 0.08, gain: 0.8 },
			{ from: 440, len: 0.13, at: 0.06, gain: 0.8 },
		]),
	tab: () => play([{ from: 440, to: 880, len: 0.1, gain: 0.7 }]),
	blocked: () => play([{ from: 170, to: 130, len: 0.07, gain: 0.7, type: "triangle" }]),
}
