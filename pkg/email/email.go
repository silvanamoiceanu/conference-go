package email

import (
	"context"
	"fmt"
	"strings"

	"github.com/giorgio/conference-go/pkg/types"
	"google.golang.org/genai"
)

type Writer struct {
	client *genai.Client
	ctx    context.Context
}

func NewWriter(ctx context.Context, apiKey string) (*Writer, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}
	return &Writer{client: client, ctx: ctx}, nil
}

func (w *Writer) WriteEmail(from, to *types.Person) (string, error) {
	prompt := fmt.Sprintf(`Write a short, warm, professional networking email from %s to %s they met at a conference.

Sender profile:
- Name: %s
- Title: %s at %s
- Skills: %s
- Interests: %s
- Goals: %s

Recipient profile:
- Name: %s
- Title: %s at %s
- Skills: %s
- Interests: %s
- Goals: %s

Guidelines:
- 3-4 short paragraphs
- Reference specific shared interests or complementary skills between them
- Mention a concrete reason to connect (collaboration, advice, shared goal)
- End with a specific call to action (coffee chat, 30-min call, etc.)
- Warm but professional tone
- Do not invent facts beyond the profiles provided
- Return only the email text (no subject line, no markdown)`,
		from.Name, to.Name,
		from.Name, from.Title, from.Company,
		strings.Join(from.Skills, ", "),
		strings.Join(from.Interests, ", "),
		strings.Join(from.Goals, ", "),
		to.Name, to.Title, to.Company,
		strings.Join(to.Skills, ", "),
		strings.Join(to.Interests, ", "),
		strings.Join(to.Goals, ", "),
	)

	resp, err := w.client.Models.GenerateContent(w.ctx, "gemini-2.5-flash", []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate email: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	return resp.Candidates[0].Content.Parts[0].Text, nil
}
