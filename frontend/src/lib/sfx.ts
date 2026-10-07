import { getPrefs } from "@/lib/prefs"

// Interface sounds, synthesized so the app ships no audio files.
//
// The character follows console-style UI sounds such as Steam Deck's: dry,
// rounded "pop" blips with a quick pitch glide and a small tap at the front,
// not bells. Everything is low-passed so nothing is harsh, and a compressor on
// the way out keeps louder volumes from distorting.

interface Bus {
	ac: AudioContext
	input: GainNode // dry signal
	send: GainNode // small room, mixed in very lightly
	noise: AudioBuffer // the raw material for the tap that opens each sound
}

let bus: Bus | null = null

// A tiny, dark room: decaying noise, generated once.
function roomImpulse(ac: AudioContext): AudioBuffer {
	const len = Math.floor(ac.sampleRate * 0.22)
	const buf = ac.createBuffer(2, len, ac.sampleRate)
	for (let ch = 0; ch < 2; ch++) {
		const data = buf.getChannelData(ch)
		for (let i = 0; i < len; i++) data[i] = (Math.random() * 2 - 1) * (1 - i / len) ** 3
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
			send.gain.value = 0.07
			send.connect(reverb).connect(comp)

			const noise = ac.createBuffer(1, Math.floor(ac.sampleRate * 0.1), ac.sampleRate)
			const nd = noise.getChannelData(0)
			for (let i = 0; i < nd.length; i++) nd[i] = Math.random() * 2 - 1

			bus = { ac, input, send, noise }
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
	/** Low-pass cutoff in Hz. */
	tone?: number
	/** A burst of band-passed noise centred on `freq` instead of a pitched note: the tap. */
	noise?: boolean
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

		const amp = ac.createGain()
		amp.gain.setValueAtTime(0.0001, t0)
		amp.gain.linearRampToValueAtTime(master * (n.gain ?? 1), t0 + 0.003)
		amp.gain.exponentialRampToValueAtTime(0.0001, t0 + n.len)
		amp.connect(b.input)
		amp.connect(b.send)

		if (n.noise) {
			const src = ac.createBufferSource()
			src.buffer = b.noise
			const bp = ac.createBiquadFilter()
			bp.type = "bandpass"
			bp.frequency.value = n.freq
			bp.Q.value = 1.1
			src.connect(bp).connect(amp)
			src.start(t0)
			src.stop(t0 + n.len + 0.02)
			continue
		}

		const lp = ac.createBiquadFilter()
		lp.type = "lowpass"
		lp.frequency.value = n.tone ?? 2400
		lp.Q.value = 0.5
		lp.connect(amp)

		const osc = ac.createOscillator()
		osc.type = "sine"
		osc.frequency.setValueAtTime(n.freq, t0)
		if (n.to) osc.frequency.exponentialRampToValueAtTime(n.to, t0 + n.len)
		osc.connect(lp)
		osc.start(t0)
		osc.stop(t0 + n.len + 0.02)
	}
}

type Dir = "up" | "down" | "left" | "right"

// Every step sounds like the same soft tick, nudged a little so rapid
// movement doesn't machine-gun. Going up or left sits slightly lower.
function moveSound(dir: Dir = "right") {
	const base = dir === "up" || dir === "left" ? 430 : 470
	const f = base * (0.97 + Math.random() * 0.06)
	play([
		{ freq: 3000, len: 0.012, gain: 0.22, noise: true }, // the tap
		{ freq: f * 1.25, to: f * 0.85, len: 0.075, gain: 1, tone: 1900 }, // the round "pop"
	])
}

export const sfx = {
	move: moveSound,
	// Two quick pops, the second higher and a touch longer: a clear "yes".
	confirm: () =>
		play([
			{ freq: 3000, len: 0.012, gain: 0.25, noise: true },
			{ freq: 520, to: 600, len: 0.07, gain: 0.85, tone: 2300 },
			{ freq: 780, to: 920, at: 0.07, len: 0.13, gain: 0.95, tone: 2600 },
		]),
	// The same two pops falling, softer.
	back: () =>
		play([
			{ freq: 620, to: 520, len: 0.065, gain: 0.7, tone: 2000 },
			{ freq: 440, to: 340, at: 0.065, len: 0.12, gain: 0.75, tone: 1800 },
		]),
	// A quick rising swipe.
	tab: () =>
		play([
			{ freq: 2200, len: 0.05, gain: 0.12, noise: true },
			{ freq: 360, to: 700, len: 0.1, gain: 0.7, tone: 2300 },
		]),
	// Nothing further that way: a dull low bump.
	blocked: () => play([{ freq: 170, to: 120, len: 0.09, gain: 0.8, tone: 700 }]),
}
