package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"github.com/giorgio/conference-go/pkg/conversation"
	"github.com/giorgio/conference-go/pkg/data"
	"github.com/giorgio/conference-go/pkg/email"
	"github.com/giorgio/conference-go/pkg/embedding"
	"github.com/giorgio/conference-go/pkg/indexing"
	"github.com/giorgio/conference-go/pkg/preprocessing"
	"github.com/giorgio/conference-go/pkg/types"
)

type App struct {
	preproc          *preprocessing.Preprocessor
	embedder         *embedding.Embedder
	index            *indexing.Index
	writer           *email.Writer
	conversationalist *conversation.Conversationalist
}

type MatchRequest struct {
	Description string `json:"description"`
}

type MatchResponse struct {
	Matches []*indexing.SearchResult `json:"matches,omitempty"`
	Error   string                   `json:"error,omitempty"`
}

type ProfileResponse struct {
	Person *types.Person `json:"person,omitempty"`
	Error  string        `json:"error,omitempty"`
}

type EmailRequest struct {
	From *types.Person `json:"from"`
	To   *types.Person `json:"to"`
}

type EmailResponse struct {
	Email string `json:"email,omitempty"`
	Error string `json:"error,omitempty"`
}

type ChatRequest struct {
	Person  *types.Person          `json:"person"`
	History []conversation.Message `json:"history"`
	Message string                 `json:"message"`
}

type ChatResponse struct {
	Reply string `json:"reply,omitempty"`
	Error string `json:"error,omitempty"`
}

type TTSRequest struct {
	Text  string `json:"text"`
	Voice string `json:"voice,omitempty"`
}

func main() {
	var webMode = flag.Bool("web", false, "Run in web server mode")
	var port = flag.String("port", "8080", "Port for web server")
	var evalMode = flag.Bool("eval", false, "Run evaluation metrics")
	flag.Parse()

	ctx := context.Background()

	// Load environment variables from .env (if present). CLI env vars still override.
	_ = godotenv.Load(".env")

	apiKey := os.Getenv("GOOGLE_API_KEY")
	var app *App
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

	if *evalMode {
		if app.index == nil || app.index.Size() == 0 {
			log.Fatal("No person profiles loaded. Cannot run evaluation.")
		}
		runEvaluation(app)
		return
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

const cacheFile = "embeddings_cache.json"

type cacheEntry struct {
	DescHash  string        `json:"desc_hash"`
	Person    *types.Person `json:"person"`
	Embedding []float32     `json:"embedding"`
}

func descHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

func loadCache() (map[string]*cacheEntry, error) {
	b, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, err
	}
	var entries []*cacheEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, err
	}
	m := make(map[string]*cacheEntry, len(entries))
	for _, e := range entries {
		m[e.DescHash] = e
	}
	return m, nil
}

func saveCache(entries []*cacheEntry) {
	b, err := json.Marshal(entries)
	if err != nil {
		log.Printf("Warning: failed to marshal cache: %v", err)
		return
	}
	if err := os.WriteFile(cacheFile, b, 0644); err != nil {
		log.Printf("Warning: failed to write cache: %v", err)
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

	writer, err := email.NewWriter(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create email writer: %w", err)
	}

	conv, err := conversation.NewConversationalist(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversationalist: %w", err)
	}

	index := indexing.NewIndex()

	// Load existing cache
	cache, err := loadCache()
	if err != nil {
		cache = make(map[string]*cacheEntry)
		if !os.IsNotExist(err) {
			log.Printf("Warning: could not load cache (%v), will re-embed", err)
		}
	} else {
		fmt.Printf("Loaded cache with %d entries\n", len(cache))
	}

	fmt.Println("Loading and embedding descriptions...")

	type initResult struct {
		person *types.Person
		emb    []float32
		desc   string
		err    error
	}

	results := make([]initResult, len(data.FakeDescriptions))
	const maxConcurrent = 10
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for i, desc := range data.FakeDescriptions {
		// Serve from cache if available
		if entry, ok := cache[descHash(desc)]; ok {
			results[i] = initResult{person: entry.Person, emb: entry.Embedding, desc: desc}
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(i int, desc string) {
			defer wg.Done()
			defer func() { <-sem }()

			person, err := preproc.ProcessDescription(desc)
			if err != nil {
				results[i] = initResult{err: fmt.Errorf("preprocess: %w", err)}
				return
			}
			emb, err := embedder.EmbedPerson(person)
			if err != nil {
				results[i] = initResult{err: fmt.Errorf("embed %s: %w", person.Name, err)}
				return
			}
			results[i] = initResult{person: person, emb: emb, desc: desc}
		}(i, desc)
	}
	wg.Wait()

	// Collect new entries to add to cache
	var newEntries []*cacheEntry
	loadedCount := 0
	failedCount := 0
	for i, r := range results {
		if r.err != nil {
			log.Printf("Failed description %d: %v", i, r.err)
			failedCount++
			continue
		}
		index.Add(r.person, r.emb)
		fmt.Printf("Loaded person %d: %s\n", i+1, r.person.Name)
		loadedCount++
		if _, cached := cache[descHash(r.desc)]; !cached {
			newEntries = append(newEntries, &cacheEntry{
				DescHash:  descHash(r.desc),
				Person:    r.person,
				Embedding: r.emb,
			})
		}
	}

	// Persist updated cache
	if len(newEntries) > 0 {
		allEntries := make([]*cacheEntry, 0, len(cache)+len(newEntries))
		for _, e := range cache {
			allEntries = append(allEntries, e)
		}
		allEntries = append(allEntries, newEntries...)
		saveCache(allEntries)
		fmt.Printf("Saved %d new entries to cache\n", len(newEntries))
	}

	if loadedCount == 0 {
		return nil, fmt.Errorf("failed to load any person profiles - check your API key")
	}

	app := &App{
		preproc:          preproc,
		embedder:         embedder,
		index:            index,
		writer:           writer,
		conversationalist: conv,
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

func runEvaluation(app *App) {
	fmt.Println("Running evaluation metrics on data.EvalCases...")
	correctTop1 := 0
	correctTop5 := 0
	total := len(data.EvalCases)
	accumSimTop1 := 0.0

	for _, tc := range data.EvalCases {
		idealPerson, err := app.preproc.ProcessDescription(tc.Query)
		if err != nil {
			log.Printf("Preprocess fail for query '%s': %v", tc.Query, err)
			idealPerson = &types.Person{Description: tc.Query}
		}

		idealEmb, err := app.embedder.EmbedPerson(idealPerson)
		if err != nil {
			log.Printf("Embed fail for query '%s': %v", tc.Query, err)
			idealEmb = fallbackEmbed(tc.Query)
		}

		results := app.index.Search(idealEmb, 5)
		if len(results) > 0 {
			accumSimTop1 += float64(results[0].Similarity)
		}

		foundTop1 := false
		foundTop5 := false
		for i, res := range results {
			for _, expected := range tc.Expected {
				if res.Person.Name == expected {
					if i == 0 {
						foundTop1 = true
					}
					foundTop5 = true
					break
				}
			}
			if foundTop5 && i >= 4 {
				break
			}
		}

		if foundTop1 {
			correctTop1++
		}
		if foundTop5 {
			correctTop5++
		}
	}

	fmt.Printf("Evaluation cases: %d\n", total)
	fmt.Printf("Top-1 accuracy: %.2f%%\n", 100*float64(correctTop1)/float64(total))
	fmt.Printf("Top-5 accuracy: %.2f%%\n", 100*float64(correctTop5)/float64(total))
	fmt.Printf("Avg Top-1 similarity: %.4f\n", accumSimTop1/float64(total))
}

func fallbackEmbed(text string) []float32 {
	const dim = 256
	vec := make([]float32, dim)
	for i := 0; i < dim; i++ {
		h := fnv.New32a()
		_, _ = h.Write([]byte(fmt.Sprintf("%d:%s", i, text)))
		vec[i] = float32(h.Sum32()) / float32(math.MaxUint32)
	}
	return vec
}

func runWebServer(app *App, port string) {
	// Serve static files
	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// API endpoints
	http.HandleFunc("/api/attendees", app.handleAttendees)
	http.HandleFunc("/api/profile", app.handleProfile)
	http.HandleFunc("/api/match", app.handleMatch)
	http.HandleFunc("/api/email", app.handleEmail)
	http.HandleFunc("/api/chat", app.handleChat)
	http.HandleFunc("/api/tts", app.handleTTS)

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
		log.Printf("Preprocessing failed, using fallback text-based person: %v", err)
		idealPerson = &types.Person{Description: req.Description}
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

func (app *App) handleAttendees(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	persons := []*types.Person{}
	if app.index != nil {
		persons = app.index.Persons()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"attendees": persons, "total": len(persons)})
}

func (app *App) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if app.preproc == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ProfileResponse{Error: "Server not initialized"})
		return
	}

	var req MatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Description == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ProfileResponse{Error: "Description is required"})
		return
	}

	person, err := app.preproc.ProcessDescription(req.Description)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ProfileResponse{Error: fmt.Sprintf("Failed to process description: %v", err)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ProfileResponse{Person: person})
}

func (app *App) handleEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if app.writer == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(EmailResponse{Error: "Server not initialized"})
		return
	}

	var req EmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.From == nil || req.To == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(EmailResponse{Error: "Both 'from' and 'to' person profiles are required"})
		return
	}

	text, err := app.writer.WriteEmail(req.From, req.To)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(EmailResponse{Error: fmt.Sprintf("Failed to write email: %v", err)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EmailResponse{Email: text})
}

func (app *App) sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(MatchResponse{Error: message})
}

func (app *App) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if app.conversationalist == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ChatResponse{Error: "Server not initialized"})
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Person == nil || req.Message == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{Error: "person and message are required"})
		return
	}

	reply, err := app.conversationalist.Chat(req.Person, req.History, req.Message)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{Error: fmt.Sprintf("Failed to generate reply: %v", err)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ChatResponse{Reply: reply})
}

func (app *App) handleTTS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if app.conversationalist == nil {
		http.Error(w, "Server not initialized", http.StatusServiceUnavailable)
		return
	}

	var req TTSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Text == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}

	audio, err := app.conversationalist.Speak(req.Text, req.Voice)
	if err != nil {
		log.Printf("TTS error: %v", err)
		http.Error(w, fmt.Sprintf("Failed to generate speech: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(audio)))
	w.Write(audio)
}