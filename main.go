package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Configuration
var ollamaAPIURL = "http://localhost:11434/api" // Base URL for Ollama API
var listenAddr = ":8085"                        // Address the Go server listens on

// --- Structs for Ollama API Interaction ---

// Model represents a single available model from Ollama /api/tags
type Model struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
	Digest     string    `json:"digest"`
}

// ModelList represents the response from Ollama /api/tags
type ModelList struct {
	Models []Model `json:"models"`
}

// Message represents a single message in the chat history (used for request and response chunks)
type Message struct {
	Role    string   `json:"role"` // "user" or "assistant"
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"` // List of base64 encoded images
}

// OllamaChatRequest represents the payload sent to Ollama /api/chat
type OllamaChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"` // Will be set to true for streaming
	Format   string    `json:"format,omitempty"` // e.g., "json"
	// Add other options like 'options' if needed
}

// OllamaStreamChunk represents one chunk of a streaming response from Ollama /api/chat
type OllamaStreamChunk struct {
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	Message   Message   `json:"message"` // Contains content fragment
	Done      bool      `json:"done"`
	// Fields available in the final 'done' chunk
	TotalDuration    int64 `json:"total_duration,omitempty"`
	LoadDuration     int64 `json:"load_duration,omitempty"`
	PromptEvalCount  int   `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64 `json:"prompt_eval_duration,omitempty"`
	EvalCount        int   `json:"eval_count,omitempty"`
	EvalDuration     int64 `json:"eval_duration,omitempty"`
	Context          []int `json:"context,omitempty"`
}


// --- Global Variables ---
var templates *template.Template
// Increased timeout for potentially long-running streams
var client = &http.Client{Timeout: 30 * time.Minute}

// --- Utility Functions ---

// getOllamaModels fetches the list of available models from the Ollama API
func getOllamaModels() (*ModelList, error) {
	req, err := http.NewRequest("GET", ollamaAPIURL+"/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request to Ollama /api/tags: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			log.Printf("Warning: Could not connect to Ollama API at %s. Is Ollama running?", ollamaAPIURL)
			return nil, fmt.Errorf("cannot connect to Ollama API at %s. Please ensure Ollama is running", ollamaAPIURL)
		}
		return nil, fmt.Errorf("error fetching models from Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API returned non-OK status (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var models ModelList
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("error decoding Ollama model list: %w", err)
	}
	return &models, nil
}


// --- HTTP Handlers ---

// handleIndex serves the main HTML page
func handleIndex(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"History": []Message{}, // Let JS manage history display
	}
	err := templates.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleGetModels serves the list of available Ollama models
func handleGetModels(w http.ResponseWriter, r *http.Request) {
	models, err := getOllamaModels()
	if err != nil {
		log.Printf("Error getting Ollama models: %v", err)
		if strings.Contains(err.Error(), "cannot connect") {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		} else {
			http.Error(w, "Failed to fetch models from Ollama", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(models); err != nil {
		log.Printf("Error encoding models to JSON: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleChat handles incoming chat messages and streams the response back
func handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form data for prompt, model, history, and images
	err := r.ParseMultipartForm(32 << 20) // 32MB max memory
	if err != nil {
		log.Printf("Error parsing multipart form: %v", err)
		http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return
	}

	model := r.FormValue("model")
	prompt := r.FormValue("prompt")
	historyJSON := r.FormValue("history") // Get history from client

	if model == "" {
		if prompt == "" && len(r.MultipartForm.File["images"]) == 0 {
			http.Error(w, "Missing 'model' or 'prompt'/'images' in form data", http.StatusBadRequest)
			return
		}
	}

	// Decode history from client
	var currentHistory []Message
	if historyJSON != "" {
		if err := json.Unmarshal([]byte(historyJSON), &currentHistory); err != nil {
			log.Printf("Error unmarshalling history JSON: %v", err)
			http.Error(w, "Invalid history format", http.StatusBadRequest)
			return
		}
	}

	// --- Handle Image Uploads ---
	var base64Images []string
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		files := r.MultipartForm.File["images"]
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("Error opening uploaded file %s: %v", fileHeader.Filename, err)
				http.Error(w, "Error processing uploaded file", http.StatusInternalServerError)
				return
			}
			// Use defer inside closure to ensure file is closed correctly in loop
			processFile := func(f io.ReadCloser) error {
				defer f.Close()
				fileBytes, err := io.ReadAll(f)
				if err != nil {
					return fmt.Errorf("reading file %s: %w", fileHeader.Filename, err)
				}
				base64Image := base64.StdEncoding.EncodeToString(fileBytes)
				base64Images = append(base64Images, base64Image)
				log.Printf("Processed image: %s (%d bytes)", fileHeader.Filename, len(fileBytes))
				return nil
			}
			if err := processFile(file); err != nil {
				log.Printf("Error processing file: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	// Create the user message
	userMessage := Message{
		Role:    "user",
		Content: prompt,
		Images:  base64Images,
	}

	// Append user message to the history for the Ollama request
	messagesForOllama := append(currentHistory, userMessage)

	// Create the request for Ollama, explicitly enabling streaming
	ollamaReqPayload := OllamaChatRequest{
		Model:    model,
		Messages: messagesForOllama,
		Stream:   true, // IMPORTANT: Enable streaming
	}

	// Marshal the request payload
	jsonData, err := json.Marshal(ollamaReqPayload)
	if err != nil {
		log.Printf("Error marshalling Ollama request: %v", err)
		http.Error(w, "Error preparing request", http.StatusInternalServerError)
		return
	}

	// Create the HTTP request to Ollama
	req, err := http.NewRequest("POST", ollamaAPIURL+"/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating request to Ollama /api/chat: %v", err)
		http.Error(w, "Error creating request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// --- Execute Request and Stream Response ---
	log.Printf("Sending request to Ollama model %s (streaming)", model)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending request to Ollama: %v", err)
		http.Error(w, "Error communicating with Ollama", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	// Check Ollama's response status before starting stream
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Ollama API returned non-OK status (%d): %s", resp.StatusCode, string(bodyBytes))
		http.Error(w, fmt.Sprintf("Ollama error (%d): %s", resp.StatusCode, string(bodyBytes)), http.StatusBadGateway) // Use BadGateway to indicate upstream error
		return
	}

	// Set headers for Server-Sent Events (SSE)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Optional: Allow CORS if frontend is served from a different origin
	// w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get the ResponseWriter Flusher interface
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Println("Error: Streaming unsupported - Flusher interface not available")
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Use a scanner to read the response body line by line
	scanner := bufio.NewScanner(resp.Body)
	var chunk OllamaStreamChunk // To parse each JSON line

	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		if len(lineBytes) == 0 {
			continue // Skip empty lines
		}

		// Attempt to parse the line as a stream chunk
		err := json.Unmarshal(lineBytes, &chunk)
		if err != nil {
			log.Printf("Error unmarshalling Ollama stream chunk: %v. Line: %s", err, string(lineBytes))
			// Decide how to handle parse errors - maybe send an error event?
			// For now, log and continue, hoping the next line is okay.
			continue
		}

		// Format as SSE message: "data: {json}\n\n"
		_, err = fmt.Fprintf(w, "data: %s\n\n", string(lineBytes))
		if err != nil {
			log.Printf("Error writing SSE data to client: %v", err)
			// Client likely disconnected, stop streaming
			return
		}

		// Flush the buffer to send the chunk immediately
		flusher.Flush()

		// If Ollama signals it's done, we can stop reading
		if chunk.Done {
			log.Printf("Ollama stream finished for model %s.", model)
			break
		}
	}

	// Check for scanner errors (e.g., connection closed by Ollama)
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading Ollama stream: %v", err)
		// Don't write to http.ResponseWriter here, connection might be broken
		return
	}

	log.Println("Finished streaming response to client.")
}


// --- Main Function ---

func main() {
	// Find and parse templates
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("Warning: Could not get executable path: %v. Assuming templates are relative to CWD.", err)
		exePath = "."
	}
	templateDir := filepath.Join(filepath.Dir(exePath), "templates")
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		log.Printf("Warning: Template directory '%s' not found relative to executable. Trying relative to CWD.", templateDir)
		templateDir = "templates"
		if _, err := os.Stat(templateDir); os.IsNotExist(err) {
			log.Fatalf("FATAL: Template directory '%s' not found. Please ensure it exists.", templateDir)
		}
	}
	templatePattern := filepath.Join(templateDir, "*.html")
	log.Printf("Loading templates from: %s", templatePattern)
	templates = template.Must(template.ParseGlob(templatePattern))

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/models", handleGetModels)
	mux.HandleFunc("/api/chat", handleChat) // Now handles streaming

	log.Printf("Starting Ollama Go Frontend Server on %s", listenAddr)
	log.Printf("Access the UI at http://localhost%s", listenAddr)
	log.Printf("Make sure Ollama server is running at %s", ollamaAPIURL)

	// Start the server
	server := &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Minute, // Increased write timeout for long streams
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}

	log.Println("Server stopped.")
}

