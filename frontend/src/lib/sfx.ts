import { getPrefs } from "@/lib/prefs"

// Interface sounds, synthesized so the app ships no audio files.
//
// The character is soft and glassy, in the spirit of a handheld console UI:
// short sine notes with a faint octave overtone, rounded off by a low-pass
// filter and placed in a small room. A compressor on the way out keeps louder
// volumes from distorting.

interface Bus {
	ac: AudioContext
	input: GainNode // dry signal and reverb send meet the compressor via these
	send: GainNode
}

let bus: Bus | null = null

// A short, dark room: decaying noise, generated once.
function roomImpulse(ac: AudioContext): AudioBuffer {
	const len = Math.floor(ac.sampleRate * 0.45)
	const buf = ac.createBuffer(2, len, ac.sampleRate)
	for (let ch = 0; ch < 2; ch++) {
		const data = buf.getChannelData(ch)
		for (let i = 0; i < len; i++) {
			const t = i / len
			data[i] = (Math.random() * 2 - 1) * (1 - t) ** 3.2
		}
	}
	return buf
}

function getBus(): Bus | null {
	try {
		if (!bus) {
			const Ctor = window.AudioContext ?? (window as any).webkitAudioContext
			if (!Ctor) return null
			const ac: AudioContext = new Ctor()

			const comp = ac.createDynamicsCompressor()
			comp.threshold.value = -14
			comp.knee.value = 18
			comp.ratio.value = 4
			comp.attack.value = 0.003
			comp.release.value = 0.12
			comp.connect(ac.destination)

			const input = ac.createGain()
			input.connect(comp)

			const reverb = ac.createConvolver()
			reverb.buffer = roomImpulse(ac)
			const send = ac.createGain()
			send.gain.value = 0.22
			send.connect(reverb).connect(comp)

			bus = { ac, input, send }
		}
		if (bus.ac.state === "suspended") void bus.ac.resume()
		return bus
	} catch {
		return null
	}
}

// Browsers only start audio after a user gesture; any of these unlocks it.
if (typeof window !== "undefined") {
	for (const ev of ["pointerdown", "keydown", "gamepadconnected"]) {
		window.addEventListener(ev, () => getBus(), { once: true, passive: true })
	}
}

interface Note {
	freq: number
	to?: number // glide target
	at?: number // seconds from now
	len: number
	gain?: number
	/** Level of the octave-up overtone relative to the note. 0 is a pure sine. */
	shimmer?: number
	/** Low-pass cutoff in Hz. */
	tone?: number
}

function play(notes: Note[]) {
	const { sounds, volume } = getPrefs()
	if (!sounds || volume <= 0) return
	const b = getBus()
	if (!b) return
	const { ac } = b

	// Perceived loudness is roughly logarithmic, so the slider follows a curve.
	const master = (volume / 100) ** 1.6 * 0.5

	for (const n of notes) {
		const t0 = ac.currentTime + (n.at ?? 0)
		const peak = master * (n.gain ?? 1)

		const amp = ac.createGain()
		amp.gain.setValueAtTime(0.0001, t0)
		amp.gain.linearRampToValueAtTime(peak, t0 + 0.004)
		amp.gain.exponentialRampToValueAtTime(0.0001, t0 + n.len)

		const lp = ac.createBiquadFilter()
		lp.type = "lowpass"
		lp.frequency.value = n.tone ?? 4200
		lp.Q.value = 0.4

		lp.connect(amp)
		amp.connect(b.input)
		amp.connect(b.send)

		const voices: [number, number][] = [[1, 1]]
		if (n.shimmer) voices.push([2, n.shimmer])
		for (const [mult, level] of voices) {
			const osc = ac.createOscillator()
			osc.type = "sine"
			osc.frequency.setValueAtTime(n.freq * mult, t0)
			if (n.to) osc.frequency.exponentialRampToValueAtTime(n.to * mult, t0 + n.len)

			const g = ac.createGain()
			g.gain.value = level
			osc.connect(g).connect(lp)
			osc.start(t0)
			osc.stop(t0 + n.len + 0.05)
		}
	}
}

// Moving walks along a pentatonic scale: right and down climb, left and up
// fall. Every note sounds good next to the others, so browsing plays a soft
// melody rather than the same blip over and over.
const SCALE = [523, 587, 659, 784, 880, 1047, 1175, 1319]
let step = 3

type Dir = "up" | "down" | "left" | "right"

function moveSound(dir: Dir = "right") {
	step = Math.min(SCALE.length - 1, Math.max(0, step + (dir === "right" || dir === "down" ? 1 : -1)))
	const f = SCALE[step]
	play([
		// A tiny bright click gives the note a clear front edge...
		{ freq: 2600, to: 1900, len: 0.014, gain: 0.3, tone: 7000 },
		// ...and the body is a round, bell-like note that settles into its pitch.
		{ freq: f * 1.03, to: f, len: 0.13, gain: 1, shimmer: 0.35, tone: 5200 },
	])
}

export const sfx = {
	move: moveSound,
	// A warm rising pair, like a bell being touched.
	confirm: () =>
		play([
			{ freq: 784, len: 0.16, gain: 0.8, shimmer: 0.3 },
			{ freq: 1175, at: 0.075, len: 0.28, gain: 0.85, shimmer: 0.3 },
		]),
	// The same pair falling and quieter.
	back: () =>
		play([
			{ freq: 1047, len: 0.1, gain: 0.6, shimmer: 0.22, tone: 3600 },
			{ freq: 698, at: 0.065, len: 0.2, gain: 0.65, shimmer: 0.22, tone: 3600 },
		]),
	// A quick upward sweep when switching tabs.
	tab: () =>
		play([
			{ freq: 659, len: 0.09, gain: 0.55, shimmer: 0.2, tone: 3600 },
			{ freq: 988, at: 0.06, len: 0.14, gain: 0.6, shimmer: 0.2, tone: 3600 },
		]),
	// Nothing further that way: a dull low bump.
	blocked: () => play([{ freq: 210, to: 160, len: 0.09, gain: 0.7, tone: 900 }]),
}
