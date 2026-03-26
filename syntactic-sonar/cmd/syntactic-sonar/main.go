package main

import (
	"log"

	"github.com/syntacticsonar/daemon/internal/audio"
	"github.com/syntacticsonar/daemon/internal/parser"
	"github.com/syntacticsonar/daemon/internal/rpc"
)

func main() {
	log.Println("Starting Syntactic Sonar daemon...")

	// 1. Initialize Audio Engine
	log.Println("Initializing Audio Synthesis Engine...")
	synth, err := audio.NewSynth()
	if err != nil {
		log.Fatalf("Failed to initialize Oto audio sink: %v", err)
	}

	// 2. Initialize AST Parser Engine
	log.Println("Initializing Tree-sitter Parser Engine...")
	analyzer := parser.NewAnalyzer()

	// 3. Initialize and Start JSON-RPC WebSocket Server
	// Defaulting to port 4444 to avoid typical 8080 conflicts
	addr := "localhost:4444"
	server := rpc.NewServer(addr, analyzer, synth)

	log.Printf("Syntactic Sonar is listening for editor payloads on ws://%s/ws", addr)

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
