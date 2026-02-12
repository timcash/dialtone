export class AudioAnalyzer {
  private audioContext: AudioContext | null = null;
  private analyzer: AnalyserNode | null = null;
  private dataArray: Float32Array | null = null;
  private oscillator: OscillatorNode | null = null;
  private gainNode: GainNode | null = null;
  private isEnabled = false;
  public isSoundOn = false;

  private async ensureContext() {
    if (!this.audioContext) {
      try {
          this.audioContext = new (window.AudioContext || (window as any).webkitAudioContext)();
          this.analyzer = this.audioContext.createAnalyser();
          this.analyzer.fftSize = 4096;
          this.dataArray = new Float32Array(this.analyzer.frequencyBinCount);
      } catch (e) {
          return;
      }
    }
    if (this.audioContext.state === 'suspended') {
      try {
          await this.audioContext.resume();
      } catch (e) {}
    }
  }

  async resume() {
      await this.ensureContext();
  }

  async enable() {
    if (this.isEnabled) return;
    if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
      return;
    }
    try {
      await this.ensureContext();
      this.stopDemo();
      
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const source = this.audioContext!.createMediaStreamSource(stream);
      source.connect(this.analyzer!);
      this.isEnabled = true;
    } catch (err) {
      console.error("[AudioAnalyzer] Error accessing microphone:", err);
    }
  }

  async startDemo(frequency: number) {
    if (!this.audioContext || this.audioContext.state === 'suspended') return;
    
    this.stopDemo();

    this.oscillator = this.audioContext!.createOscillator();
    this.gainNode = this.audioContext!.createGain();

    this.oscillator.type = 'sine';
    this.oscillator.frequency.setValueAtTime(frequency, this.audioContext!.currentTime);
    
    this.gainNode.gain.setValueAtTime(0, this.audioContext!.currentTime);
    this.gainNode.gain.linearRampToValueAtTime(0.1, this.audioContext!.currentTime + 0.1);

    this.oscillator.connect(this.gainNode);
    this.gainNode.connect(this.analyzer!);
    
    if (this.isSoundOn) {
        this.gainNode.connect(this.audioContext!.destination);
    }

    this.oscillator.start();
  }

  stopDemo() {
    if (this.oscillator) {
      try {
        this.oscillator.stop();
        this.oscillator.disconnect();
      } catch (e) {}
      this.oscillator = null;
    }
    if (this.gainNode) {
      try {
          this.gainNode.disconnect();
      } catch(e) {}
      this.gainNode = null;
    }
  }

  getChromagram(sensitivity: number): Float32Array {
    const chroma = new Float32Array(12).fill(0);
    if (!this.analyzer || !this.dataArray || !this.audioContext || this.audioContext.state === 'suspended') return chroma;

    const data = this.dataArray as any;
    this.analyzer.getFloatFrequencyData(data);
    const sampleRate = this.audioContext.sampleRate;
    const fftSize = this.analyzer.fftSize;
    const binWidth = sampleRate / fftSize;

    for (let i = 0; i < data.length; i++) {
      const frequency = i * binWidth;
      if (frequency < 20) continue;

      const midiNote = 12 * Math.log2(frequency / 440) + 69;
      if (midiNote < 0 || midiNote > 127) continue;

      const noteIndex = Math.round(midiNote) % 12;
      const db = data[i];
      if (db > -100) {
        const magnitude = Math.pow(10, db / 20) * sensitivity;
        chroma[noteIndex] += magnitude;
      }
    }

    let max = 0;
    for (let i = 0; i < 12; i++) if (chroma[i] > max) max = chroma[i];
    if (max > 0) {
      for (let i = 0; i < 12; i++) chroma[i] /= max;
    }

    return chroma;
  }

  get isActive() { return this.isEnabled; }
  get isSuspended() { return !this.audioContext || this.audioContext.state === 'suspended'; }
}
