# Conference-Go

An AI-enabled smart conference connection matching and email generation system using Google Gemini API via AI Studio.

## Features

- **Preprocessing Module**: Uses Gemini to parse natural language descriptions of people into structured JSON format optimized for embeddings.
- **Embedding Module**: Embeds structured person data into vector space using Gemini embedding models.
- **Indexing Module**: Brute-force indexing and similarity search for person embeddings.
- **Attendee Profiles**: 308 diverse fake person descriptions for testing and demonstration.
- **Connection Matching**: Input an ideal connection description and find the most similar people from the dataset.
- **Email Drafting**: AI-generated introduction emails between matched attendees.
- **Chat**: Converse with an AI persona of any attendee.
- **Text-to-Speech**: Listen to attendee responses via Gemini TTS.

## Quick Start (Docker)

```bash
./setup.sh
```

The script checks for Docker, prompts for your API key (saves to `.env`), builds the image, and starts the server at http://localhost:8080. The SQLite database is mounted as a bind volume so profiles persist across container restarts.

Or manually:

```bash
export GOOGLE_API_KEY=your_key_here
docker compose up --build
```

## Usage

### Web Interface (Recommended)

Run the application in web server mode:

```bash
go run cmd/server/main.go -web
```

Or specify a custom port:

```bash
go run cmd/server/main.go -web -port 3000
```

Then open http://localhost:8080 in your browser.

The web interface provides:
- Browse all conference attendees
- Search for your ideal connection by natural language description
- Add new attendees via `POST /api/attendees/add`
- AI-generated introduction emails between matched attendees
- Chat with an AI persona of any attendee
- Text-to-speech playback of attendee responses

### Command Line Interface

For command-line usage:

```bash
go run cmd/server/main.go "Your ideal connection description here"
```

If no description is provided, it uses a default example.

The system will:
1. Load and embed all 308 attendee profiles (database-cached after first run)
2. Process your ideal connection description
3. Find and display the top 5 most similar matches with details

## Startup & Database

Profiles (structured data + embeddings) are stored in `conference.db` (SQLite) after the first run. On subsequent starts, only descriptions not already in the database are re-processed.

| Scenario | Time (`go run`) |
|---|---|
| Cold start (empty DB) | ~86s — preprocesses and embeds all 308 profiles with 10 concurrent Gemini API calls |
| Warm start (DB populated) | ~24s — loads profiles from SQLite (most of this is Go compilation) |

Once the database is populated, no API calls are made at startup. New attendees added via `POST /api/attendees/add` are persisted immediately and survive restarts.

## Setup

1. Install Go 1.19+ if not already installed.

2. Get a Gemini API key:
   - Go to [Google AI Studio](https://aistudio.google.com/)
   - Sign in with your Google account
   - Create a new API key or use an existing one

3. Set environment variables:
   ```bash
   export GOOGLE_API_KEY=your-gemini-api-key
   ```

4. Install dependencies:
   ```bash
   go mod tidy
   ```

5. Run the application:
   ```bash
   go run cmd/server/main.go "Your ideal connection description here"
   ```

## Architecture

- `pkg/preprocessing/`: Natural language → structured JSON conversion using Gemini.
- `pkg/embedding/`: Generates embeddings for person profiles using Gemini embedding API.
- `pkg/indexing/`: Brute-force vector indexing and cosine similarity search.
- `pkg/database/`: SQLite-backed profile store (structured data + embeddings).
- `pkg/types/`: Common data structures.
- `pkg/email/`: AI-generated introduction email drafting.
- `pkg/conversation/`: Attendee persona chat and text-to-speech via Gemini.
- `pkg/data/`: 308 fake attendee descriptions and evaluation cases.
- `web/templates/`: HTML templates for the web interface.
- `web/static/`: CSS and JavaScript files for styling and interactivity.
- `cmd/server/`: Main application supporting both CLI and web server modes.

## .env support

Create a `.env` file in repository root:

```bash
GOOGLE_API_KEY=your_gemini_api_key
```

Install dependency:

```bash
go get github.com/joho/godotenv
```

Then run:

```bash
go run cmd/server/main.go -web
```

Your `.env` should be gitignored:

```
.env
```
