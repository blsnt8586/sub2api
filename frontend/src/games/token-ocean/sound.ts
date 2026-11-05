/** Small synthesized water/chomp cues; no downloads, audio starts only after a user gesture. */
export function createOceanSound() {
  let audio: AudioContext | null = null,
    active = false,
    lastBite = -Infinity
  const tone = (
    hz: number,
    end: number,
    duration: number,
    delay = 0,
    type: OscillatorType = 'sine',
  ) => {
    if (!active || !audio || audio.state !== 'running') return
    const oscillator = audio.createOscillator(),
      gain = audio.createGain(),
      at = audio.currentTime + delay
    oscillator.type = type
    oscillator.frequency.setValueAtTime(hz, at)
    oscillator.frequency.exponentialRampToValueAtTime(end, at + duration)
    gain.gain.setValueAtTime(0.0001, at)
    gain.gain.exponentialRampToValueAtTime(0.045, at + 0.008)
    gain.gain.exponentialRampToValueAtTime(0.0001, at + duration)
    oscillator.connect(gain)
    gain.connect(audio.destination)
    oscillator.start(at)
    oscillator.stop(at + duration + 0.01)
    oscillator.onended = () => {
      oscillator.disconnect()
      gain.disconnect()
    }
  }
  return {
    async enable() {
      try {
        audio ??= new AudioContext()
        await audio.resume()
        active = true
        return true
      } catch {
        active = false
        return false
      }
    },
    mute() {
      active = false
    },
    bite(combo: number) {
      if (!audio || audio.currentTime - lastBite < 0.055) return
      lastBite = audio.currentTime
      const base = 230 + Math.min(12, combo) * 27
      tone(base, base * 0.55, 0.085)
      tone(base * 1.8, base * 1.4, 0.07, 0.02)
    },
    dash() {
      tone(260, 65, 0.18)
    },
    evolve() {
      for (let i = 0; i < 3; i++)
        tone(392 * Math.pow(1.26, i), 440 * Math.pow(1.26, i), 0.16, i * 0.065)
    },
    hurt() {
      tone(160, 55, 0.23, 0, 'triangle')
    },
    close() {
      active = false
      if (audio) void audio.close().catch(() => {})
      audio = null
    },
  }
}
