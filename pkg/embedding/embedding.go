package embedding

import (
	"context"
	"fmt"
	"strings"

	"github.com/giorgio/conference-go/pkg/types"
	"google.golang.org/genai"
)

type Embedder struct {
	client *genai.Client
	ctx    context.Context
}

func NewEmbedder(ctx context.Context, apiKey string) (*Embedder, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &Embedder{
		client: client,
		ctx:    ctx,
	}, nil
}

func (e *Embedder) Close() error {
	// The new SDK doesn't have a Close method
	return nil
}

func (e *Embedder) EmbedPerson(person *types.Person) ([]float32, error) {
	// Convert person to text for embedding
	text := e.personToText(person)

	resp, err := e.client.Models.EmbedContent(e.ctx, "text-embedding-004", []*genai.Content{
		{
			Parts: []*genai.Part{
				{Text: text},
			},
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to embed content: %w", err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return resp.Embeddings[0].Values, nil
}

func (e *Embedder) personToText(person *types.Person) string {
	var parts []string
	if person.Name != "" {
		parts = append(parts, "Name: "+person.Name)
	}
	if person.Title != "" {
		parts = append(parts, "Title: "+person.Title)
	}
	if person.Company != "" {
		parts = append(parts, "Company: "+person.Company)
	}
	if len(person.Interests) > 0 {
		parts = append(parts, "Interests: "+strings.Join(person.Interests, ", "))
	}
	if len(person.Skills) > 0 {
		parts = append(parts, "Skills: "+strings.Join(person.Skills, ", "))
	}
	if len(person.Goals) > 0 {
		parts = append(parts, "Goals: "+strings.Join(person.Goals, ", "))
	}
	return strings.Join(parts, ". ")
}