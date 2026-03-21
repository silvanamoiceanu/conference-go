# Conference-Go

An AI-enabled smart conference connection matching and email generation system using Google Gemini API via AI Studio.

## Features

- **Preprocessing Module**: Uses Gemini to parse natural language descriptions of people into structured JSON format optimized for embeddings.
- **Embedding Module**: Embeds structured person data into vector space using Gemini embedding models.
- **Indexing Module**: Brute-force indexing and similarity search for person embeddings.
- **Fake Data**: Includes 50 diverse fake person descriptions for testing and demonstration.
- **Connection Matching**: Input an ideal connection description and find the most similar people from the dataset.

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
- A clean form to input your ideal connection description
- Real-time progress updates during processing
- Beautiful display of match results with similarity scores
- Responsive design that works on mobile and desktop

### Command Line Interface

For command-line usage:

```bash
go run cmd/server/main.go "Your ideal connection description here"
```

If no description is provided, it uses a default example.

The system will:
1. Load and embed all 50 fake person profiles (cached in memory)
2. Process your ideal connection description
3. Find and display the top 5 most similar matches with details

## Caching

Embeddings are computed once at startup and cached in memory for fast subsequent searches. No redundant API calls are made during runtime.

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

- `pkg/preprocessing/`: Handles natural language to structured data conversion using Gemini.
- `pkg/embedding/`: Generates embeddings for person profiles using Gemini embedding API.
- `pkg/indexing/`: Manages vector indexing and similarity search.
- `pkg/types/`: Common data structures.
- `web/templates/`: HTML templates for the web interface.
- `web/static/`: CSS and JavaScript files for styling and interactivity.
- `cmd/server/`: Main application supporting both CLI and web server modes. 
