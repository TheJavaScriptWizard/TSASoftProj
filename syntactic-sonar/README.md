# Syntactic Sonar

Syntactic Sonar is a real-time, audio-spatial accessibility daemon designed to help blind and visually impaired developers navigate source code structure. Rather than relying solely on a screen reader dictating characters, Syntactic Sonar maps a file's underlying Abstract Syntax Tree (AST) to raw, procedurally generated audio cues.

## Architecture Overview

The system operates in a decoupled Client-Server architecture, allowing any text editor to connect to a centralized audio synthesis daemon over JSON-RPC.

### 1. The Editor Client (VS Code Extension)
The client is responsible for observing user interactions and efficiently forwarding state to the daemon.
- **Event Listeners**: Hooks into text editor selection and cursor movement events.
- **Transport**: Maintains a raw, persistent TCP connection via Node's `net.Socket` to `127.0.0.1:4445`.
- **Payload**: On every cursor movement, it streams a lightweight JSON-RPC 2.0 request containing the absolute file path, 0-indexed row and column coordinates, and the current text contents of the document.

### 2. The JSON-RPC Daemon (`server.go`)
The core server handles concurrency, payload unpacking, and orchestrates the AST calculations.
- **Port Management**: Listens on TCP port 4445.
- **Parsing**: Deserializes incoming `update_sonar` JSON payloads using the standard `encoding/json` package.
- **Concurrency**: Spawns independent goroutines for AST analysis to ensure the editor never experiences blocking or network backpressure, utilizing mutex locks to guard internal state such as the most recently calculated depth.

### 3. The AST Analyzer (`treesitter.go`)
Converts the raw text payload into a deeply queryable syntax tree using the `go-tree-sitter` C-bindings.
- **Tokenization**: Parses the raw byte stream into an active Go-language AST.
- **Spatial Querying**: Locates the specific AST leaf node (e.g., `identifier`, `if_statement`) that strictly intersects with the user's current line and column coordinates.
- **Depth Calculation**: Iterates recursively upward through the node's parent chain until it reaches the root `source_file`, counting the exact structural depth of the statement to determine its scope level.

### 4. The Audio Synthesizer (`synth.go`)
Translates the calculated numerical depth and column position into audible spatial feedback.
- **Hardware Sink**: Interfaces with the operating system's native audio drivers using `ebitengine/oto/v3` to stream continuous PCM data.
- **Frequency Mapping**: Applies an exponentially scaled algorithm based on a Fully Diminished 7th Arpeggio. Each increase in AST depth drastically shifts the generated sine wave frequency by 3 semitones (a minor third).
- **Stereo Panning**: Transforms the editor column coordinate into an audio panning percentage, weighting the generated PCM pulse-code arrays to the left or right channels to provide physical, spatial horizontal awareness.
- **Envelope Generation**: Smoothly modulates the raw sine wave using programmatic Attack, Decay, Sustain, and Release (ADSR) to prevent audio clipping and polyphony crackling during rapid cursor traversal.

## Installation and Execution

1. Build and run the central Go daemon:
   ```bash
   cd cmd/syntactic-sonar
   go run main.go
   ```
2. Navigate to the companion VS Code Extension directory.
3. Install extension dependencies and compile:
   ```bash
   npm install
   npm run compile
   ```
4. Press F5 within the extension workspace to launch the Extension Development Host.
5. Open any Go source file in the newly opened window and navigate through the code with the arrow keys to trigger spatial sonar feedback.
