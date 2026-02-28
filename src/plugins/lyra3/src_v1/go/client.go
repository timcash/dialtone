package lyra3

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"google.golang.org/genai"
)

type Client struct {
	genaiClient *genai.Client
}

func NewClient(ctx context.Context, projectID, location string) (*Client, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, err
	}
	return &Client{genaiClient: client}, nil
}

func (c *Client) GenerateMusic(ctx context.Context, prompt string, outputPath string) error {
	// Lyria 2 model ID
	model := "lyria-002"

	// Call the model to generate content
	resp, err := c.genaiClient.Models.GenerateContent(ctx, model, genai.Text(prompt), nil)
	if err != nil {
		return fmt.Errorf("generate content: %w", err)
	}

	// Extract audio from the response
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("no audio content returned in response")
	}

	part := resp.Candidates[0].Content.Parts[0]
	if blob := part.InlineData; blob != nil {
		audioData, err := base64.StdEncoding.DecodeString(blob.Data)
		if err != nil {
			return fmt.Errorf("decode audio: %w", err)
		}

		if err := os.WriteFile(outputPath, audioData, 0644); err != nil {
			return fmt.Errorf("save file: %w", err)
		}
		return nil
	}

	return fmt.Errorf("response part did not contain audio data (InlineData)")
}
