# Ollama Go Frontend

![Alt text](/screenshot.png?raw=true "Optional Title")

A simple, self-hosted web frontend for the [Ollama](https://ollama.com/) API, built with Go and vanilla JavaScript. Allows you to chat with your local Ollama models through a web browser interface.

## Features

* **Model Selection:** Fetches and lists available Ollama models from your local instance.
* **Model Tagging:** Automatically attempts to tag models (e.g., `[vision]`, `[text]`, `[code]`) in the dropdown for easier identification.
* **Text Chat:** Send text prompts to the selected model.
* **Image Upload:** Supports uploading images for multimodal models (like LLaVA).
* **Streaming Responses:** Displays model responses token-by-token in real-time.
* **Markdown Rendering:** Renders assistant responses as Markdown (includes code blocks, lists, etc.).
* **Dark Theme:** Features a VS Code-like dark theme for comfortable viewing.
* **Conversation History:** Maintains chat history within the current browser session (cleared on reload or manually).
* **Simple Go Backend:** Lightweight backend server written in Go using standard libraries.
* **Vanilla JS Frontend:** No heavy frontend frameworks, uses Tailwind CSS (via CDN) for styling.

## Prerequisites

1.  **Go:** Version 1.18 or higher installed. ([Installation Guide](https://go.dev/doc/install))
2.  **Ollama:** Ollama must be installed and running locally. The application assumes Ollama is accessible at `http://localhost:11434`. ([Ollama Website](https://ollama.com/))
3.  **Ollama Models:** You need to have pulled the models you want to use into Ollama (e.g., `ollama pull llama3`, `ollama pull llava`).

## How to Run

1.  **Clone the Repository (or download the files):**
    ```bash
    git clone <your-repo-url>
    cd ollama-go-frontend
    ```
    *(Replace `<your-repo-url>` with the actual URL if you host it)*
    *Alternatively, ensure you have `main.go` and the `templates/index.html` file in the correct structure.*

2.  **Initialize Go Module (if needed):**
    If you haven't already, initialize the Go module:
    ```bash
    go mod init ollama-go-frontend
    # or go mod tidy
    ```

3.  **Build the Application (Optional but Recommended):**
    ```bash
    go build
    ```
    This creates an executable (e.g., `ollama-go-frontend` or `ollama-go-frontend.exe`).

4.  **Run the Application:**
    * If built:
        ```bash
        ./ollama-go-frontend
        ```
        (or `.\ollama-go-frontend.exe` on Windows)
    * Alternatively, run directly:
        ```bash
        go run main.go
        ```

5.  **Access the Frontend:**
    Open your web browser and navigate to `http://localhost:8085` (or the address shown in the terminal output).

6.  **Ensure Ollama is Running:** Make sure your Ollama application/server is running in the background.

## Technology Stack

* **Backend:** Go (using standard libraries: `net/http`, `encoding/json`, etc.)
* **Frontend:** HTML, Vanilla JavaScript, Tailwind CSS (via CDN)
* **Markdown Parsing:** [marked.js](https://marked.js.org/) (via CDN)
* **HTML Sanitization:** [DOMPurify](https://github.com/cure53/DOMPurify) (via CDN)
* **API Interaction:** Ollama HTTP API

## Future Improvements / Ideas

* [ ] More robust server-side session management for persistent history.
* [ ] Syntax highlighting for code blocks in Markdown.
* [ ] Ability to configure Ollama API endpoint via environment variable or flag.
* [ ] Option to save/load conversations.
* [ ] More sophisticated model tagging (perhaps using model metadata if available).
* [ ] Add stop generation button.
* [ ] Implement light theme toggle.
* [ ] Support for Ollama API options (temperature, top_k, etc.).

## Contributing

Contributions are welcome! Please feel free to open an issue or submit a pull request.
