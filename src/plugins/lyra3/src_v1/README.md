# Lyra3 src_v1 (Google Lyria)

Version 1 focuses on Google's Lyria AI music model, providing tools to generate music from text prompts and images.

## Google Lyria Prompt Guide

To get the best results, structure your prompts using these four pillars:

### 1. Core Pillars
- **Genre & Era:** Be specific about style and time (e.g., "early 90s hip-hop", "2000s pop"). Blend genres for unique results (e.g., "K-pop with a Motown edge").
- **Tempo:** Use descriptive terms like "fast", "slow", "slow ballad", or "drum and bass" to set the pace.
- **Instruments:** List specific instruments (e.g., "acoustic guitar", "80s synth"). If blank, Lyria uses typical genre instruments.
- **Dynamics:** Describe the song's flow (e.g., "a quiet piano that builds into an explosive chorus").

### 2. Vocals and Lyrics
- **Vocal Profiles:** Define singer gender, range (baritone, soprano), and texture (soulful, gravelly, breathy).
- **Custom Lyrics:** Use the prefix `Lyrics:` followed by your text.
- **Backing Vocals:** Use round brackets for echoes or backing singers: `Lyrics: Let's go (go)`.
- **Thematic Lyrics:** If you want Lyria to write lyrics, provide a theme like "a song about success".

### 3. Image-to-Music (Vision)
When using images to set the mood, Lyria analyzes:
- **Subject:** Emotional state, clothing, pose.
- **Location:** Background setting (city, wilderness, etc.).
- **Action:** What is happening in the scene.

### 4. Advanced Tips
- **Musicality:** Try prompting for "harmonies", "counterpoints", or "dense instrumental layers".
- **Vocal Patterns:** Describe the "groove" (e.g., "fast-paced" or "laid-back").
- **Visual Variety:** Try historical paintings, cartoons, or scientific diagrams.

## Example Prompts

Here are some high-quality prompts structured according to the Lyria pillars:

### 1. SimCity-Style Urban Jazz (Ambient Game Music)
> **Prompt:** "Urban Jazz city-builder soundtrack, late 90s style. Smooth and sophisticated. Featuring a melodic electric piano (Rhodes), a walking upright bass line, and a subtle jazz flute. Light acoustic drum kit with brushed snare and crisp hi-hats. Steady moderate tempo, 115 BPM. The dynamics are even and soothing, perfect for a construction and management simulation background loop."

### 2. Cyberpunk / Synthwave
> **Prompt:** "Aggressive 80s cyberpunk synth-wave, driving tempo. Featuring heavy analog bass synths, gated reverb drums, and a soaring lead synthesizer melody. Intense and cinematic dynamics with a feeling of high-speed motion. No vocals."

### 3. Lo-Fi Hip-Hop (Chill Study)
> **Prompt:** "Early 2020s lo-fi hip-hop beat, laid-back and chill. Soft sampled acoustic guitar loops, crackly vinyl textures, and a dusty boom-bap drum pattern. Slow tempo, 85 BPM. Mellow and consistent dynamics for focused work."

### 4. Epic Orchestral
> **Prompt:** "Modern cinematic orchestral theme, epic and heroic. High-energy tempo with staccato strings, powerful brass fanfares, and thunderous taiko drums. Starts with a quiet, mysterious woodwind solo and builds into a massive, explosive orchestral climax."

## CLI Usage

```sh
# Generate music from a prompt
./dialtone.sh lyra3 src_v1 generate --prompt "90s synth-wave with a driving tempo"

# List your generated tracks
./dialtone.sh lyra3 src_v1 list

# Show track details
./dialtone.sh lyra3 src_v1 info --id <track_id>

# Run smoke tests
./dialtone.sh lyra3 src_v1 test
```

## Google Gen AI SDK Example (Go)

To use Lyria programmatically in Go, use the `google.golang.org/genai` SDK:

```go
import (
	"context"
	"encoding/base64"
	"os"
	"google.golang.org/genai"
)

func generate(ctx context.Context, projectID, prompt string) error {
	client, _ := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: "us-central1",
		Backend:  genai.BackendVertexAI,
	})

	model := "lyria-002"
	resp, err := client.Models.GenerateContent(ctx, model, genai.Text(prompt), nil)
	if err != nil {
		return err
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		part := resp.Candidates[0].Content.Parts[0]
		if blob := part.InlineData; blob != nil {
			audioData, _ := base64.StdEncoding.DecodeString(blob.Data)
			return os.WriteFile("output.wav", audioData, 0644)
		}
	}
	return nil
}
```
