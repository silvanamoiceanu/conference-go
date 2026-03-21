package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		log.Fatal("GOOGLE_API_KEY not set")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatal(err)
	}

	page, err := client.Models.List(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Page created successfully\n")
	fmt.Println("Available models:")
	count := 0
	for {
		model, err := page.Next(ctx)
		if err != nil {
			fmt.Printf("Error getting next model: %v\n", err)
			if err == genai.ErrPageDone {
				fmt.Println("Reached end of page")
				break
			}
			log.Fatal(err)
		}
		fmt.Printf("- %s\n", model.Name)
		count++
		if count > 20 {
			fmt.Println("Stopping after 20 models")
			break
		}
	}
	fmt.Printf("Total models found: %d\n", count)

	// Check specific embedding models by name
	modelNames := []string{
		"gemini-embedding-2",
		"gemini-embedding-1",
		"models/gemini-embedding-2",
		"models/gemini-embedding-1",
		"embedding-2",
		"models/embedding-2",
		"embedding-1",
		"models/embedding-1",
		"gemini-2.5-flash",
		"models/gemini-2.5-flash",
		"gemini-2.5-pro",
		"models/gemini-2.5-pro",
	}

	// Quick embedding check for valid model names
	for _, mname := range []string{"gemini-2.5-flash", "gemini-2.5-pro"} {
		fmt.Printf("\nTrying EmbedContent on model: %s\n", mname)
		resp, err := client.Models.EmbedContent(ctx, mname, []*genai.Content{{Parts: []*genai.Part{{Text: "test"}}}}, nil)
		if err != nil {
			fmt.Printf("EmbedContent failed for %s: %v\n", mname, err)
			continue
		}
		fmt.Printf("Embed content success: %#v\n", resp)
	}
	for _, mname := range modelNames {
		m, err := client.Models.Get(ctx, mname, nil)
		if err != nil {
			fmt.Printf("could not get model %q: %v\n", mname, err)
			continue
		}
		fmt.Printf("model %q exists (display=%q, version=%q)\n", mname, m.DisplayName, m.Version)
	}

}