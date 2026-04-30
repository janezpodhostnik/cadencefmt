package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/janezpodhostnik/cadencefmt/internal/format"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Server implements a minimal LSP server that only handles textDocument/formatting.
type Server struct {
	mu   sync.Mutex
	docs map[protocol.DocumentURI]string
	conn jsonrpc2.Conn
}

// NewServer creates a new LSP server.
func NewServer() *Server {
	return &Server{
		docs: make(map[protocol.DocumentURI]string),
	}
}

// Run starts the LSP server on stdio.
func (s *Server) Run(ctx context.Context) error {
	stream := jsonrpc2.NewStream(stdrwc{})
	conn := jsonrpc2.NewConn(stream)
	s.conn = conn

	conn.Go(ctx, s.handle)
	<-conn.Done()
	return conn.Err()
}

func (s *Server) handle(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	switch req.Method() {
	case "initialize":
		return s.handleInitialize(ctx, reply, req)
	case "initialized":
		return reply(ctx, nil, nil)
	case "shutdown":
		return reply(ctx, nil, nil)
	case "exit":
		os.Exit(0)
		return nil
	case "textDocument/didOpen":
		return s.handleDidOpen(ctx, reply, req)
	case "textDocument/didChange":
		return s.handleDidChange(ctx, reply, req)
	case "textDocument/didClose":
		return s.handleDidClose(ctx, reply, req)
	case "textDocument/formatting":
		return s.handleFormatting(ctx, reply, req)
	default:
		return reply(ctx, nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, req.Method()))
	}
}

func (s *Server) handleInitialize(_ context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.InitializeParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(context.Background(), nil, err)
	}

	result := protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync:           protocol.TextDocumentSyncKindFull,
			DocumentFormattingProvider: true,
		},
	}
	return reply(context.Background(), result, nil)
}

func (s *Server) handleDidOpen(_ context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(context.Background(), nil, err)
	}

	s.mu.Lock()
	s.docs[params.TextDocument.URI] = params.TextDocument.Text
	s.mu.Unlock()

	return reply(context.Background(), nil, nil)
}

func (s *Server) handleDidChange(_ context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(context.Background(), nil, err)
	}

	s.mu.Lock()
	// Full document sync: the last content change contains the entire document
	if len(params.ContentChanges) > 0 {
		s.docs[params.TextDocument.URI] = params.ContentChanges[len(params.ContentChanges)-1].Text
	}
	s.mu.Unlock()

	return reply(context.Background(), nil, nil)
}

func (s *Server) handleDidClose(_ context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(context.Background(), nil, err)
	}

	s.mu.Lock()
	delete(s.docs, params.TextDocument.URI)
	s.mu.Unlock()

	return reply(context.Background(), nil, nil)
}

func (s *Server) handleFormatting(_ context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DocumentFormattingParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(context.Background(), nil, err)
	}

	s.mu.Lock()
	text, ok := s.docs[params.TextDocument.URI]
	s.mu.Unlock()

	if !ok {
		return reply(context.Background(), nil, fmt.Errorf("document not found: %s", params.TextDocument.URI))
	}

	filename := string(uri.URI(params.TextDocument.URI).Filename())
	formatted, err := format.Format([]byte(text), filename, format.Default())
	if err != nil {
		// Return empty edits on error — don't disrupt the editor
		return reply(context.Background(), []protocol.TextEdit{}, nil)
	}

	if bytes.Equal(formatted, []byte(text)) {
		return reply(context.Background(), []protocol.TextEdit{}, nil)
	}

	// Single edit replacing the entire document
	lines := countLines(text)
	edit := protocol.TextEdit{
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: uint32(lines), Character: 0},
		},
		NewText: string(formatted),
	}

	return reply(context.Background(), []protocol.TextEdit{edit}, nil)
}

func countLines(s string) int {
	n := 0
	for _, c := range s {
		if c == '\n' {
			n++
		}
	}
	return n
}

// stdrwc wraps stdin/stdout as an io.ReadWriteCloser for jsonrpc2.
type stdrwc struct{}

func (stdrwc) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdrwc) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdrwc) Close() error                { return nil }
