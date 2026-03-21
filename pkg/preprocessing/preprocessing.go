package preprocessing

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/giorgio/conference-go/pkg/types"
	"google.golang.org/genai"
)

type Preprocessor struct {
	client *genai.Client
	ctx    context.Context
}

func NewPreprocessor(ctx context.Context, apiKey string) (*Preprocessor, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &Preprocessor{
		client: client,
		ctx:    ctx,
	}, nil
}

func (p *Preprocessor) Close() error {
	// The new SDK doesn't have a Close method
	return nil
}

func (p *Preprocessor) ProcessDescription(description string) (*types.Person, error) {
	prompt := fmt.Sprintf(`Parse the following natural language description of a person into a JSON object with the following structure:
{
  "name": "string",
  "title": "string",
  "company": "string",
  "interests": ["string"],
  "skills": ["string"],
  "goals": ["string"]
}

Description: %s

Return only the JSON object, no additional text.`, description)

	req := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	resp, err := p.client.Models.GenerateContent(p.ctx, "gemini-2.5-flash", []*genai.Content{
		{
			Parts: []*genai.Part{
				{Text: prompt},
			},
		},
	}, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	jsonStr := resp.Candidates[0].Content.Parts[0].Text

	var person types.Person
	if err := json.Unmarshal([]byte(jsonStr), &person); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	person.Description = description
	return &person, nil
}