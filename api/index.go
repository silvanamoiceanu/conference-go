package handler

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"github.com/giorgio/conference-go/pkg/conversation"
	"github.com/giorgio/conference-go/pkg/database"
	"github.com/giorgio/conference-go/pkg/email"
	"github.com/giorgio/conference-go/pkg/embedding"
	"github.com/giorgio/conference-go/pkg/indexing"
	"github.com/giorgio/conference-go/pkg/preprocessing"
	"github.com/giorgio/conference-go/pkg/types"
)

//go:embed public
var publicFS embed.FS

type app struct {
	preproc           *preprocessing.Preprocessor
	embedder          *embedding.Embedder
	writer            *email.Writer
	conversationalist *conversation.Conversationalist
	db                *database.DB
}

var (
	once      sync.Once
	globalApp *app
	initErr   error
	mux       *http.ServeMux
)

func initApp() {
	_ = godotenv.Load()
	ctx := context.Background()

	apiKey := os.Getenv("GOOGLE_API_KEY")
	dbURL := os.Getenv("DATABASE_URL")
	if apiKey == "" || dbURL == "" {
		initErr = fmt.Errorf("GOOGLE_API_KEY and DATABASE_URL must be set")
		return
	}

	preproc, err := preprocessing.NewPreprocessor(ctx, apiKey)
	if err != nil {
		initErr = fmt.Errorf("preprocessor: %w", err)
		return
	}

	embedder, err := embedding.NewEmbedder(ctx, apiKey)
	if err != nil {
		initErr = fmt.Errorf("embedder: %w", err)
		return
	}

	writer, err := email.NewWriter(ctx, apiKey)
	if err != nil {
		initErr = fmt.Errorf("email writer: %w", err)
		return
	}

	conv, err := conversation.NewConversationalist(ctx, apiKey)
	if err != nil {
		initErr = fmt.Errorf("conversationalist: %w", err)
		return
	}

	db, err := database.Open(ctx, dbURL)
	if err != nil {
		initErr = fmt.Errorf("database: %w", err)
		return
	}

	globalApp = &app{
		preproc:           preproc,
		embedder:          embedder,
		writer:            writer,
		conversationalist: conv,
		db:                db,
	}

	// Static file serving from embedded FS
	subFS, err := fs.Sub(publicFS, "public")
	if err != nil {
		initErr = fmt.Errorf("embed fs: %w", err)
		return
	}
	staticServer := http.FileServer(http.FS(subFS))

	mux = http.NewServeMux()

	// Static files and root
	mux.Handle("/static/", staticServer)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := publicFS.ReadFile("public/index.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})

	// API routes
	mux.HandleFunc("/api/match", globalApp.handleMatch)
	mux.HandleFunc("/api/attendees/add", globalApp.handleAddAttendee)
	mux.HandleFunc("/api/attendees", globalApp.handleAttendees)
	mux.HandleFunc("/api/profile", globalApp.handleProfile)
	mux.HandleFunc("/api/email", globalApp.handleEmail)
	mux.HandleFunc("/api/chat", globalApp.handleChat)
	mux.HandleFunc("/api/tts", globalApp.handleTTS)
}

// Handler is the Vercel serverless entry point.
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(initApp)

	if initErr != nil {
		http.Error(w, "Server initialization failed: "+initErr.Error(), http.StatusInternalServerError)
		return
	}

	mux.ServeHTTP(w, r)
}

// --- request / response types ---

type matchRequest struct {
	Description string `json:"description"`
}

type matchResponse struct {
	Matches []*indexing.SearchResult `json:"matches,omitempty"`
	Error   string                   `json:"error,omitempty"`
}

type profileResponse struct {
	Person *types.Person `json:"person,omitempty"`
	Error  string        `json:"error,omitempty"`
}

type emailRequest struct {
	From *types.Person `json:"from"`
	To   *types.Person `json:"to"`
}

type emailResponse struct {
	Email string `json:"email,omitempty"`
	Error string `json:"error,omitempty"`
}

type chatRequest struct {
	Person  *types.Person          `json:"person"`
	History []conversation.Message `json:"history"`
	Message string                 `json:"message"`
}

type chatResponse struct {
	Reply string `json:"reply,omitempty"`
	Error string `json:"error,omitempty"`
}

type ttsRequest struct {
	Text  string `json:"text"`
	Voice string `json:"voice,omitempty"`
}

// --- helpers ---

func descHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

func sendJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// --- handlers ---

func (a *app) handleMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req matchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Description == "" {
		sendJSON(w, http.StatusBadRequest, matchResponse{Error: "description is required"})
		return
	}

	ctx := r.Context()

	person, err := a.preproc.ProcessDescription(req.Description)
	if err != nil {
		log.Printf("preprocess failed, using fallback: %v", err)
		person = &types.Person{Description: req.Description}
	}

	emb, err := a.embedder.EmbedPerson(person)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, matchResponse{Error: fmt.Sprintf("embed failed: %v", err)})
		return
	}

	persons, sims, err := a.db.SearchByEmbedding(ctx, emb, 5)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, matchResponse{Error: fmt.Sprintf("search failed: %v", err)})
		return
	}

	results := make([]*indexing.SearchResult, len(persons))
	for i := range persons {
		results[i] = &indexing.SearchResult{Person: persons[i], Similarity: sims[i]}
	}

	sendJSON(w, http.StatusOK, matchResponse{Matches: results})
}

func (a *app) handleAttendees(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	entries, err := a.db.GetAll(r.Context())
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	persons := make([]*types.Person, len(entries))
	for i, e := range entries {
		persons[i] = e.Person
	}

	sendJSON(w, http.StatusOK, map[string]any{"attendees": persons, "total": len(persons)})
}

func (a *app) handleAddAttendee(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req matchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Description == "" {
		sendJSON(w, http.StatusBadRequest, profileResponse{Error: "description is required"})
		return
	}

	ctx := r.Context()
	hash := descHash(req.Description)

	if a.db.Has(ctx, hash) {
		sendJSON(w, http.StatusConflict, profileResponse{Error: "profile already exists"})
		return
	}

	person, err := a.preproc.ProcessDescription(req.Description)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, profileResponse{Error: fmt.Sprintf("failed to process description: %v", err)})
		return
	}

	emb, err := a.embedder.EmbedPerson(person)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, profileResponse{Error: fmt.Sprintf("failed to embed profile: %v", err)})
		return
	}

	if err := a.db.Save(ctx, &database.Entry{DescHash: hash, Person: person, Embedding: emb}); err != nil {
		sendJSON(w, http.StatusInternalServerError, profileResponse{Error: fmt.Sprintf("failed to save profile: %v", err)})
		return
	}

	log.Printf("Added new attendee: %s (%s at %s)", person.Name, person.Title, person.Company)
	sendJSON(w, http.StatusOK, profileResponse{Person: person})
}

func (a *app) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req matchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Description == "" {
		sendJSON(w, http.StatusBadRequest, profileResponse{Error: "description is required"})
		return
	}

	person, err := a.preproc.ProcessDescription(req.Description)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, profileResponse{Error: fmt.Sprintf("failed to process: %v", err)})
		return
	}

	sendJSON(w, http.StatusOK, profileResponse{Person: person})
}

func (a *app) handleEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req emailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.From == nil || req.To == nil {
		sendJSON(w, http.StatusBadRequest, emailResponse{Error: "both 'from' and 'to' profiles are required"})
		return
	}

	text, err := a.writer.WriteEmail(req.From, req.To)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, emailResponse{Error: fmt.Sprintf("failed to write email: %v", err)})
		return
	}

	sendJSON(w, http.StatusOK, emailResponse{Email: text})
}

func (a *app) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Person == nil || req.Message == "" {
		sendJSON(w, http.StatusBadRequest, chatResponse{Error: "person and message are required"})
		return
	}

	reply, err := a.conversationalist.Chat(req.Person, req.History, req.Message)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, chatResponse{Error: fmt.Sprintf("failed to generate reply: %v", err)})
		return
	}

	sendJSON(w, http.StatusOK, chatResponse{Reply: reply})
}

func (a *app) handleTTS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ttsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Text == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}

	audio, err := a.conversationalist.Speak(req.Text, req.Voice)
	if err != nil {
		log.Printf("TTS error: %v", err)
		http.Error(w, fmt.Sprintf("failed to generate speech: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(audio)))
	w.Write(audio)
}
