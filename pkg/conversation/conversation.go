package conversation

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/giorgio/conference-go/pkg/types"
	"google.golang.org/genai"
)

type Message struct {
	Role    string `json:"role"`    // "user" or "model"
	Content string `json:"content"`
}

type Conversationalist struct {
	client *genai.Client
	ctx    context.Context
}

func NewConversationalist(ctx context.Context, apiKey string) (*Conversationalist, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}
	return &Conversationalist{client: client, ctx: ctx}, nil
}

func buildSystemPrompt(person *types.Person) string {
	return fmt.Sprintf(`You are roleplaying as %s, a real conference attendee at Global Tech Summit 2026.

Your profile:
- Name: %s
- Title: %s at %s
- Skills: %s
- Interests: %s
- Goals: %s

The user is practicing their networking pitch or introduction with you. Respond naturally and authentically as this person would in a real conference networking conversation. Be genuinely engaged but professionally discerning — ask follow-up questions, show enthusiasm for relevant topics, gently push back on vague pitches. Keep responses concise (2-4 sentences) since this is a live spoken conversation. Do not break character. Do not acknowledge you are an AI.`,
		person.Name,
		person.Name, person.Title, person.Company,
		strings.Join(person.Skills, ", "),
		strings.Join(person.Interests, ", "),
		strings.Join(person.Goals, ", "),
	)
}

// Chat generates a reply as the simulated person given the conversation history and new user message.
func (c *Conversationalist) Chat(person *types.Person, history []Message, userMessage string) (string, error) {
	var contents []*genai.Content

	for _, msg := range history {
		role := "user"
		if msg.Role == "model" {
			role = "model"
		}
		contents = append(contents, &genai.Content{
			Role:  role,
			Parts: []*genai.Part{{Text: msg.Content}},
		})
	}
	contents = append(contents, &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: userMessage}},
	})

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: buildSystemPrompt(person)}},
		},
	}

	resp, err := c.client.Models.GenerateContent(c.ctx, "gemini-2.5-flash", contents, config)
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	return resp.Candidates[0].Content.Parts[0].Text, nil
}

// Speak converts text to speech using Gemini TTS and returns WAV audio bytes.
func (c *Conversationalist) Speak(text, voiceName string) ([]byte, error) {
	if voiceName == "" {
		voiceName = "Kore"
	}

	config := &genai.GenerateContentConfig{
		ResponseModalities: []string{"AUDIO"},
		SpeechConfig: &genai.SpeechConfig{
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{
					VoiceName: voiceName,
				},
			},
		},
	}

	resp, err := c.client.Models.GenerateContent(
		c.ctx,
		"gemini-2.5-flash-preview-tts",
		[]*genai.Content{{Parts: []*genai.Part{{Text: text}}}},
		config,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate speech: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no audio in response")
	}

	part := resp.Candidates[0].Content.Parts[0]
	if part.InlineData == nil {
		return nil, fmt.Errorf("no inline data in audio response")
	}

	// SDK returns decoded PCM bytes; wrap in a WAV container for browser playback.
	// Gemini TTS spec: 24 kHz, mono, 16-bit PCM.
	return addWAVHeader(part.InlineData.Data, 24000, 1, 16), nil
}

// addWAVHeader wraps raw PCM bytes in a standard WAV (RIFF) container.
func addWAVHeader(pcm []byte, sampleRate, channels, bitsPerSample int) []byte {
	dataLen := len(pcm)
	buf := make([]byte, 44+dataLen)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+dataLen))
	copy(buf[8:12], "WAVE")

	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16) // chunk size
	binary.LittleEndian.PutUint16(buf[20:22], 1)  // PCM format
	binary.LittleEndian.PutUint16(buf[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sampleRate*channels*bitsPerSample/8))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(channels*bitsPerSample/8))
	binary.LittleEndian.PutUint16(buf[34:36], uint16(bitsPerSample))

	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataLen))
	copy(buf[44:], pcm)

	return buf
}
