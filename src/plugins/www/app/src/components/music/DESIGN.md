# Music Visualization Component: Frequency Circle

## Goal
Create a Three.js-based visualization that represents audio frequencies on a "Frequency Circle" inspired by the Circle of Fifths.

## Features
- **Three.js Scene**: A 3D environment with a central circular visualization.
- **Circle of Fifths Layout**: 12 nodes representing the 12 chromatic notes (C, G, D, A, E, B, Gb, Db, Ab, Eb, Bb, F).
- **Audio Analysis**: Use Web Audio API `AnalyserNode` to capture microphone input.
- **Chromagram Calculation**: 
    - Convert FFT frequency bins to MIDI notes: `n = 12 * log2(f / 440) + 69`.
    - Aggregate energy into 12 pitch class bins (`n % 12`).
    - Normalize energy for visualization.
- **Live Visualization**:
    - The 12 nodes will glow or scale based on the energy in their corresponding pitch class.
    - Particles or lines connecting active notes to show harmonic relationships.
- **Menu Controls**:
    - "Enable Microphone" button to start audio context and listener.
    - Sensitivity slider.
    - Visualization style toggles.

## Technical Details

### Musical Mapping
The Circle of Fifths order:
`C, G, D, A, E, B, F#, Db, Ab, Eb, Bb, F`
Indices in the 12-semitone chromatic scale (C=0, C#=1, ...):
`0, 7, 2, 9, 4, 11, 6, 1, 8, 3, 10, 5`

### Component Structure
- `mountMusic(container: HTMLElement)`: Entry point.
- `MusicScene`: Class to manage Three.js scene, camera, and render loop.
- `AudioAnalyzer`: Class to handle Web Audio API, mic input, and FFT-to-Chroma conversion.
- `FrequencyCircle`: Three.js object group representing the 12 notes.

### Visual Style
- Neon/Cyberpunk aesthetic matching Dialtone.
- Soft glowing spheres for notes.
- Dynamic lines between nodes if multiple notes are detected (chords).
- Background stars or grid.
