// Interface sounds.
//
// When Steam is installed, these are Steam's own Deck UI sounds, played from
// its install folder through /sfx/<name> (see library.Sounds); Shelf doesn't
// ship them. Without Steam, or if a file is missing, a small synthesizer
// stands in: dry, rounded pops in the same spirit. Either way a compressor on
// the way out keeps louder volumes from distorting.

import { useSyncExternalStore } from "react"

import { getPrefs } from "@/lib/prefs"

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
			void loadSamples(bus)
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

function synth(notes: Note[]) {
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

// --- Steam's own sounds ---

type Key = "move" | "confirm" | "back" | "tab" | "blocked"

const FILES: Record<Key, string> = {
	move: "navigation",
	confirm: "confirm",
	back: "back",
	tab: "tab",
	blocked: "blocked",
}

// The files are mastered quietly (some peak below 0.1), so each is scaled to a common peak.
const TARGET_PEAK = 0.6
const MAX_BOOST = 8

interface Sample {
	buffer: AudioBuffer
	gain: number
}

const samples = new Map<Key, Sample>()
type Source = "loading" | "steam" | "builtin"
let source: Source = "loading"
const sourceListeners = new Set<() => void>()

function setSource(next: Source) {
	source = next
	sourceListeners.forEach((l) => l())
}

/** Where sounds come from right now: Steam's files, the built-in synth, or still loading. */
export function useSoundSource(): Source {
	return useSyncExternalStore(
		(cb) => {
			sourceListeners.add(cb)
			return () => sourceListeners.delete(cb)
		},
		() => source
	)
}

function peakOf(buf: AudioBuffer): number {
	let peak = 0
	for (let c = 0; c < buf.numberOfChannels; c++) {
		for (const v of buf.getChannelData(c)) peak = Math.max(peak, Math.abs(v))
	}
	return peak
}

async function loadSamples(b: Bus) {
	const keys = Object.keys(FILES) as Key[]
	await Promise.all(
		keys.map(async (key) => {
			try {
				const res = await fetch(`/sfx/${FILES[key]}`)
				if (!res.ok) return
				const data = await res.arrayBuffer()
				// The callback form works in every WebKit; the promise form doesn't.
				const buffer = await new Promise<AudioBuffer>((resolve, reject) =>
					b.ac.decodeAudioData(data, resolve, reject)
				)
				const peak = peakOf(buffer)
				if (peak > 0) samples.set(key, { buffer, gain: Math.min(MAX_BOOST, TARGET_PEAK / peak) })
			} catch {
				// this one falls back to the synth
			}
		})
	)
	setSource(samples.size === keys.length ? "steam" : "builtin")
}

// The previous navigation tick is faded out when the next one starts, so
// holding a direction doesn't stack their long tails into a wash.
let lastMove: { src: AudioBufferSourceNode; amp: GainNode } | null = null

function playSample(key: Key): boolean {
	const { sounds, volume } = getPrefs()
	if (!sounds || volume <= 0) return true // muted counts as handled
	const b = getBus()
	const sample = samples.get(key)
	if (!b || !sample) return false

	const amp = b.ac.createGain()
	amp.gain.value = (volume / 100) ** 1.6 * sample.gain
	const src = b.ac.createBufferSource()
	src.buffer = sample.buffer
	src.connect(amp).connect(b.input)

	if (key === "move") {
		if (lastMove) {
			const t = b.ac.currentTime
			lastMove.amp.gain.cancelScheduledValues(t)
			lastMove.amp.gain.setValueAtTime(lastMove.amp.gain.value, t)
			lastMove.amp.gain.linearRampToValueAtTime(0, t + 0.04)
			lastMove.src.stop(t + 0.05)
		}
		lastMove = { src, amp }
	}
	src.start()
	return true
}

type Dir = "up" | "down" | "left" | "right"

// The synth's version of a navigation tick, nudged a little so rapid
// movement doesn't machine-gun. Going up or left sits slightly lower.
function moveSynth(dir: Dir) {
	const base = dir === "up" || dir === "left" ? 430 : 470
	const f = base * (0.97 + Math.random() * 0.06)
	synth([
		{ freq: 3000, len: 0.012, gain: 0.22, noise: true }, // the tap
		{ freq: f * 1.25, to: f * 0.85, len: 0.075, gain: 1, tone: 1900 }, // the round "pop"
	])
}

export const sfx = {
	move: (dir: Dir = "right") => playSample("move") || moveSynth(dir),
	confirm: () =>
		playSample("confirm") ||
		synth([
			{ freq: 3000, len: 0.012, gain: 0.25, noise: true },
			{ freq: 520, to: 600, len: 0.07, gain: 0.85, tone: 2300 },
			{ freq: 780, to: 920, at: 0.07, len: 0.13, gain: 0.95, tone: 2600 },
		]),
	back: () =>
		playSample("back") ||
		synth([
			{ freq: 620, to: 520, len: 0.065, gain: 0.7, tone: 2000 },
			{ freq: 440, to: 340, at: 0.065, len: 0.12, gain: 0.75, tone: 1800 },
		]),
	tab: () =>
		playSample("tab") ||
		synth([
			{ freq: 2200, len: 0.05, gain: 0.12, noise: true },
			{ freq: 360, to: 700, len: 0.1, gain: 0.7, tone: 2300 },
		]),
	blocked: () => playSample("blocked") || synth([{ freq: 170, to: 120, len: 0.09, gain: 0.8, tone: 700 }]),
}

// Create the audio context right away, so Steam's sounds are decoded and ready
// by the time the first button is pressed. This stays at the end of the file
// because it needs everything above it to be initialized.
if (typeof window !== "undefined") getBus()
