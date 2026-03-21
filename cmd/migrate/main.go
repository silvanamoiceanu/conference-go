// migrate pre-populates the Neon PostgreSQL database with all attendee profiles.
// Run once before deploying to Vercel:
//
//	DATABASE_URL=<neon-url> GOOGLE_API_KEY=<key> go run ./cmd/migrate
package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"github.com/giorgio/conference-go/pkg/data"
	"github.com/giorgio/conference-go/pkg/database"
	"github.com/giorgio/conference-go/pkg/embedding"
	"github.com/giorgio/conference-go/pkg/preprocessing"
)

func descHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

func main() {
	_ = godotenv.Load()
	ctx := context.Background()

	apiKey := os.Getenv("GOOGLE_API_KEY")
	dbURL := os.Getenv("DATABASE_URL")
	if apiKey == "" || dbURL == "" {
		log.Fatal("GOOGLE_API_KEY and DATABASE_URL must be set")
	}

	db, err := database.Open(ctx, dbURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	preproc, err := preprocessing.NewPreprocessor(ctx, apiKey)
	if err != nil {
		log.Fatalf("preprocessor: %v", err)
	}

	embedder, err := embedding.NewEmbedder(ctx, apiKey)
	if err != nil {
		log.Fatalf("embedder: %v", err)
	}

	var pending []string
	for _, desc := range data.FakeDescriptions {
		if !db.Has(ctx, descHash(desc)) {
			pending = append(pending, desc)
		}
	}

	if len(pending) == 0 {
		fmt.Println("All profiles already in database. Nothing to do.")
		return
	}

	fmt.Printf("Processing %d new descriptions...\n", len(pending))

	type result struct {
		desc string
		err  error
	}

	results := make([]result, len(pending))
	const maxConcurrent = 10
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for i, desc := range pending {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, desc string) {
			defer wg.Done()
			defer func() { <-sem }()

			person, err := preproc.ProcessDescription(desc)
			if err != nil {
				results[i] = result{desc: desc, err: fmt.Errorf("preprocess: %w", err)}
				return
			}

			emb, err := embedder.EmbedPerson(person)
			if err != nil {
				results[i] = result{desc: desc, err: fmt.Errorf("embed %s: %w", person.Name, err)}
				return
			}

			if err := db.Save(ctx, &database.Entry{
				DescHash:  descHash(desc),
				Person:    person,
				Embedding: emb,
			}); err != nil {
				results[i] = result{desc: desc, err: fmt.Errorf("save %s: %w", person.Name, err)}
				return
			}

			fmt.Printf("  saved: %s\n", person.Name)
		}(i, desc)
	}
	wg.Wait()

	var failed int
	for _, r := range results {
		if r.err != nil {
			log.Printf("FAILED: %v", r.err)
			failed++
		}
	}

	fmt.Printf("\nDone. %d saved, %d failed.\n", len(pending)-failed, failed)
}
