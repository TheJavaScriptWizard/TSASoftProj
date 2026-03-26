package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/syntacticsonar/daemon/internal/audio"
	"github.com/syntacticsonar/daemon/internal/parser"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all for local daemon
	},
}

// Request defines the JSON-RPC 2.0 request structure
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      *int            `json:"id"`
}

// UpdateSonarParams defines the payload for the update_sonar method
type UpdateSonarParams struct {
	File     string `json:"file"`
	Language string `json:"language"`
	Line     uint32 `json:"line"` // 0-indexed
	Col      uint32 `json:"col"`  // 0-indexed
	Content  string `json:"content"`
}

// Response defines the JSON-RPC 2.0 response structure
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *ErrorData  `json:"error,omitempty"`
	ID      *int        `json:"id"`
}

type ErrorData struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server handles incoming JSON-RPC connections
type Server struct {
	addr      string
	analyzer  *parser.Analyzer
	synth     *audio.Synth
	mu        sync.Mutex
	lastDepth int
	lastLine  int64
}

// NewServer initializes a new JSON-RPC WebSocket and TCP server
func NewServer(addr string, analyzer *parser.Analyzer, synth *audio.Synth) *Server {
	return &Server{
		addr:      addr,
		analyzer:  analyzer,
		synth:     synth,
		lastLine:  -1,
	}
}

// Start begins listening for requests on HTTP/WS and raw TCP
func (s *Server) Start() error {
	// Start Raw TCP server on port 4445 for Neovim (Native Lua TCP)
	go s.startTCP(":4445")

	http.HandleFunc("/ws", s.handleWS)
	log.Printf("Listening for JSON-RPC WebSockets on ws://%s/ws\n", s.addr)
	return http.ListenAndServe(s.addr, nil)
}

func (s *Server) startTCP(addr string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("TCP Listen error: %v\n", err)
		return
	}
	log.Printf("Listening for JSON-RPC raw TCP on %s (for Neovim Lua)\n", addr)
	
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go s.handleTCP(conn)
	}
}

func (s *Server) handleTCP(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	// Up to 10MB buffer for large files
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		if req.Method == "update_sonar" {
			var params UpdateSonarParams
			if err := json.Unmarshal(req.Params, &params); err == nil {
				go s.processUpdate(params)
			}
			
			if req.ID != nil {
				resp := Response{
					JSONRPC: "2.0",
					Result:  "ok",
					ID:      req.ID,
				}
				respBytes, _ := json.Marshal(resp)
				conn.Write(append(respBytes, '\n'))
			}
		}
	}
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	for {
		var req Request
		err := conn.ReadJSON(&req)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ReadJSON error: %v\n", err)
			}
			break
		}

		if req.Method == "update_sonar" {
			var params UpdateSonarParams
			if err := json.Unmarshal(req.Params, &params); err == nil {
				go s.processUpdate(params)
			} else {
				log.Printf("Failed to unmarshal params: %v\n", err)
			}
			
			if req.ID != nil {
				resp := Response{
					JSONRPC: "2.0",
					Result:  "ok",
					ID:      req.ID,
				}
				conn.WriteJSON(resp)
			}
		}
	}
}

func (s *Server) processUpdate(params UpdateSonarParams) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Never trigger sounds if we solely moved horizontally on the exact same line.
	if int64(params.Line) == s.lastLine {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 2. Find the structural starting point of the line (skip pure whitespace)
	firstCharCol := uint32(0)
	lines := strings.Split(params.Content, "\n")
	if int(params.Line) < len(lines) {
		lineText := lines[params.Line]
		if strings.TrimSpace(lineText) == "" {
			return // Ignore pure empty lines to prevent chaotic depth jumps
		}
		for i, c := range lineText {
			if c != ' ' && c != '\t' {
				firstCharCol = uint32(i)
				break
			}
		}
	}

	// 3. Analyze the AST depth of the block struct (the first non-whitespace char)
	result, err := s.analyzer.Analyze(ctx, []byte(params.Content), params.Line, firstCharCol, params.Language)
	if err != nil {
		log.Printf("Analysis error: %v\n", err)
		return
	}

	s.lastLine = int64(params.Line)

	log.Printf("Line %d - Depth is: %d (Type:%s)\n", params.Line, result.Depth, result.NodeType)
	s.synth.PlaySonar(result.Depth, int(params.Col))
	s.lastDepth = result.Depth
}
