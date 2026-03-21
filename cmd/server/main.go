package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/giorgio/conference-go/pkg/data"
	"github.com/giorgio/conference-go/pkg/embedding"
	"github.com/giorgio/conference-go/pkg/indexing"
	"github.com/giorgio/conference-go/pkg/preprocessing"
)

type App struct {
	preproc  *preprocessing.Preprocessor
	embedder *embedding.Embedder
	index    *indexing.Index
}

type MatchRequest struct {
	Description string `json:"description"`
}

type MatchResponse struct {
	Matches []*indexing.SearchResult `json:"matches,omitempty"`
	Error   string                   `json:"error,omitempty"`
}

func main() {
	var webMode = flag.Bool("web", false, "Run in web server mode")
	var port = flag.String("port", "8080", "Port for web server")
	flag.Parse()

	ctx := context.Background()

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		log.Println("Warning: GOOGLE_API_KEY environment variable not set")
		if !*webMode {
			log.Fatal("GOOGLE_API_KEY environment variable must be set for CLI mode")
		}
		log.Println("Starting web server in limited mode...")
		app = &App{} // Start with empty app for web mode
		runWebServer(app, *port)
		return
	}

	app, err := initializeApp(ctx, apiKey)
	if err != nil {
		if *webMode {
			log.Printf("Warning: %v", err)
			log.Println("Starting web server in limited mode...")
			app = &App{} // Start with empty app
		} else {
			log.Fatalf("Failed to initialize app: %v", err)
		}
	}

	if *webMode {
		runWebServer(app, *port)
	} else {
		if app.index == nil || app.index.Size() == 0 {
			log.Fatal("No person profiles loaded. Cannot run in CLI mode.")
		}
		runCLI(app)
	}
}

func initializeApp(ctx context.Context, apiKey string) (*App, error) {
	preproc, err := preprocessing.NewPreprocessor(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create preprocessor: %w", err)
	}

	embedder, err := embedding.NewEmbedder(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	index := indexing.NewIndex()

	// Load and embed all fake descriptions
	fmt.Println("Loading and embedding fake descriptions...")
	loadedCount := 0
	failedCount := 0
	for i, desc := range data.FakeDescriptions {
		person, err := preproc.ProcessDescription(desc)
		if err != nil {
			log.Printf("Failed to process description %d: %v", i, err)
			failedCount++
			continue
		}

		emb, err := embedder.EmbedPerson(person)
		if err != nil {
			log.Printf("Failed to embed person %d: %v", i, err)
			failedCount++
			continue
		}

		index.Add(person, emb)
		fmt.Printf("Embedded person %d: %s\n", i+1, person.Name)
		loadedCount++
	}

	if loadedCount == 0 {
		return nil, fmt.Errorf("failed to load any person profiles - check your API key")
	}

	app := &App{
		preproc:  preproc,
		embedder: embedder,
		index:    index,
	}

	if app.index.Size() == 0 {
		return nil, fmt.Errorf("failed to load any person profiles - check your API key")
	}

	return app, nil
}

func runCLI(app *App) {
	// Get ideal connection description from command line or use default
	var idealDesc string
	if len(os.Args) > 1 {
		idealDesc = os.Args[1]
	} else {
		idealDesc = "Looking for a senior AI engineer interested in machine learning ethics and open-source contributions, who enjoys hiking and mentoring."
	}

	fmt.Printf("Processing ideal connection: %s\n", idealDesc)

	idealPerson, err := app.preproc.ProcessDescription(idealDesc)
	if err != nil {
		log.Fatalf("Failed to process ideal description: %v", err)
	}

	fmt.Printf("Ideal person: %+v\n", idealPerson)

	idealEmb, err := app.embedder.EmbedPerson(idealPerson)
	if err != nil {
		log.Fatalf("Failed to embed ideal person: %v", err)
	}

	// Search for top 5 matches
	results := app.index.Search(idealEmb, 5)
	fmt.Println("\nTop matches:")
	for i, res := range results {
		fmt.Printf("%d. %s (Similarity: %.4f)\n", i+1, res.Person.Name, res.Similarity)
		fmt.Printf("   Title: %s at %s\n", res.Person.Title, res.Person.Company)
		fmt.Printf("   Interests: %v\n", res.Person.Interests)
		fmt.Printf("   Skills: %v\n", res.Person.Skills)
		fmt.Println()
	}
}

func runWebServer(app *App, port string) {
	// Serve static files
	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// API endpoint for matching
	http.HandleFunc("/api/match", app.handleMatch)

	// Main page
	http.HandleFunc("/", app.handleIndex)

	fmt.Printf("Starting web server on port %s...\n", port)
	fmt.Printf("Open http://localhost:%s in your browser\n", port)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func (app *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("./web/templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, nil)
}

func (app *App) handleMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if app is properly initialized
	if app.preproc == nil || app.embedder == nil || app.index == nil {
		app.sendJSONError(w, "Server is not properly initialized. Please check your API key and restart.", http.StatusServiceUnavailable)
		return
	}

	if app.index.Size() == 0 {
		app.sendJSONError(w, "No person profiles are loaded. Please check your API key.", http.StatusServiceUnavailable)
		return
	}

	var req MatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.sendJSONError(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	if req.Description == "" {
		app.sendJSONError(w, "Description is required", http.StatusBadRequest)
		return
	}

	// Process the description
	idealPerson, err := app.preproc.ProcessDescription(req.Description)
	if err != nil {
		app.sendJSONError(w, fmt.Sprintf("Failed to process description: %v", err), http.StatusInternalServerError)
		return
	}

	idealEmb, err := app.embedder.EmbedPerson(idealPerson)
	if err != nil {
		app.sendJSONError(w, fmt.Sprintf("Failed to embed description: %v", err), http.StatusInternalServerError)
		return
	}

	// Search for matches
	results := app.index.Search(idealEmb, 5)

	response := MatchResponse{Matches: results}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (app *App) sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(MatchResponse{Error: message})
}