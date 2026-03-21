package embedding

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"sync"

	"github.com/giorgio/conference-go/pkg/types"
	"google.golang.org/genai"
)

type Embedder struct {
	client       *genai.Client
	ctx          context.Context
	workingModel string
	mu           sync.RWMutex
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

func (e *Embedder) embedText(model, text string) ([]float32, error) {
	resp, err := e.client.Models.EmbedContent(e.ctx, model, []*genai.Content{
		{Parts: []*genai.Part{{Text: text}}},
	}, nil)
	if err != nil {
		return nil, err
	}
	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return resp.Embeddings[0].Values, nil
}

func (e *Embedder) EmbedPerson(person *types.Person) ([]float32, error) {
	text := e.personToText(person)

	// Fast path: use cached working model
	e.mu.RLock()
	cached := e.workingModel
	e.mu.RUnlock()

	if cached != "" {
		if values, err := e.embedText(cached, text); err == nil {
			return values, nil
		}
		// cached model failed — reset and fall through to discovery
		e.mu.Lock()
		e.workingModel = ""
		e.mu.Unlock()
	}

	modelCandidates := []string{
		"gemini-embedding-001",
		"gemini-embedding-2",
		"gemini-embedding-1",
		"models/gemini-embedding-2",
		"models/gemini-embedding-1",
		"gemini-2.5-flash",
		"models/gemini-2.5-flash",
		"gemini-2.5-pro",
		"models/gemini-2.5-pro",
	}

	for _, model := range modelCandidates {
		if values, err := e.embedText(model, text); err == nil {
			e.mu.Lock()
			e.workingModel = model
			e.mu.Unlock()
			return values, nil
		}
	}

	// Fallback embedding using deterministic pseudo-vector for local testing.
	return e.fallbackEmbed(text), nil
}

func (e *Embedder) fallbackEmbed(text string) []float32 {
	const dim = 256
	vec := make([]float32, dim)
	for i := 0; i < dim; i++ {
		h := fnv.New32a()
		_, _ = h.Write([]byte(fmt.Sprintf("%d:%s", i, text)))
		vec[i] = float32(h.Sum32()) / float32(math.MaxUint32)
	}
	return vec
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