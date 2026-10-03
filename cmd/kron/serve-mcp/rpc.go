package servemcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/xxx/kron/internal/model"
)

// JSON-RPC 2.0 error codes. The full spec also allows server-defined
// codes in the range -32000 to -32099; we don't use them in v1.1.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

// request is the wire envelope for a single JSON-RPC 2.0 call.
//
// We use json.RawMessage for Params so each tool can decode its own
// argument shape. ID is json.RawMessage too because the spec allows
// string, number, or null IDs and we must echo back whatever the
// client sent.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

// response is the success envelope. Error responses use responseError
// instead; the two are mutually exclusive on the wire.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  any             `json:"result"`
	ID      json.RawMessage `json:"id"`
}

// responseError is the error envelope.
type responseError struct {
	JSONRPC string          `json:"jsonrpc"`
	Error   rpcError        `json:"error"`
	ID      json.RawMessage `json:"id"`
}

// rpcError matches the JSON-RPC 2.0 error object shape.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	// Data is intentionally omitted from v1.1: tools should put
	// actionable detail into Message so MCP clients can show it
	// without having to handle two error fields. If a future tool
	// needs structured data, it can be added without breaking the
	// existing envelope.
}

// server is the JSON-RPC dispatcher. It owns the tool registry and
// the in/out streams; it does NOT own the root path (passed into each
// tool via toolContext at dispatch time).
type server struct {
	root   string
	out    io.Writer
	errOut io.Writer
	tools  map[string]toolHandler
}

// serve reads one request per line from in, dispatches it, and writes
// one response per line to out. Notifications (requests without an
// "id") produce no response. The loop terminates when EOF is reached
// or when a single line fails to be read.
func (s *server) serve(in io.Reader) error {
	scanner := bufio.NewScanner(in)
	// A pathological JSON-RPC line should not be allowed to balloon
	// memory. 1 MiB matches the limit used in internal/parser/anchors.go.
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	enc := json.NewEncoder(s.out)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // tolerate blank lines; spec is silent on them
		}
		s.handleLine(line, enc)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(s.errOut, "kron serve-mcp: read stdin:", err)
		return err
	}
	return nil
}

// handleLine parses one JSON-RPC envelope, dispatches it, and writes
// the response. A line that fails to parse as JSON yields a parse-error
// response (if the client sent an id we can recover) or is otherwise
// dropped silently — per the spec, we cannot respond to an unparseable
// notification.
func (s *server) handleLine(line []byte, enc *json.Encoder) {
	// Strip a UTF-8 BOM if present. PowerShell (and some Windows
	// tools) emit one at the start of stdin; treating it as part of
	// the JSON envelope would yield a parse error on the first
	// request of every session. UTF-8 BOM is the byte sequence
	// EF BB BF.
	if len(line) >= 3 && line[0] == 0xEF && line[1] == 0xBB && line[2] == 0xBF {
		line = line[3:]
	}
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		s.writeError(enc, nil, codeParseError, "parse error: "+err.Error())
		return
	}
	if req.JSONRPC != "2.0" {
		s.writeError(enc, req.ID, codeInvalidRequest, "jsonrpc field must be exactly \"2.0\"")
		return
	}
	if req.Method == "" {
		s.writeError(enc, req.ID, codeInvalidRequest, "method field is required")
		return
	}

	handler, ok := s.tools[req.Method]
	if !ok {
		s.writeError(enc, req.ID, codeMethodNotFound, "method not found: "+req.Method)
		return
	}

	// Notifications (no id) never produce a response, even on handler
	// error. This matches the JSON-RPC 2.0 spec.
	isNotification := len(req.ID) == 0

	result, err := handler(s.toolContext(), req.Params)
	if err != nil {
		// Map sentinel errors to JSON-RPC error codes. Anything else
		// is an internal error from the server's perspective; the
		// human-readable message is preserved so MCP clients can show
		// it in their UI.
		if isNotification {
			fmt.Fprintln(s.errOut, "kron serve-mcp: notification handler error:", err)
			return
		}
		s.writeError(enc, req.ID, classifyError(err), err.Error())
		return
	}

	if isNotification {
		return
	}
	if err := enc.Encode(response{
		JSONRPC: "2.0",
		Result:  result,
		ID:      req.ID,
	}); err != nil {
		fmt.Fprintln(s.errOut, "kron serve-mcp: write response:", err)
	}
}

// toolContext is the ctx every tool handler receives. It carries the
// MCP caller identity and is the v1.1 plumbing point for future
// cancellation hooks (currently no-op).
func (s *server) toolContext() context.Context {
	return withMCPCaller(context.Background(), "")
}

// writeError writes a JSON-RPC error response. id is passed through
// as-is (it may be nil for unparseable lines; the response will carry
// id:null per the spec). For notifications (no id), the spec says no
// response should be produced; the calling handleLine already
// short-circuits before reaching here for that case, so this function
// always writes.
func (s *server) writeError(enc *json.Encoder, id json.RawMessage, code int, msg string) {
	if err := enc.Encode(responseError{
		JSONRPC: "2.0",
		Error:   rpcError{Code: code, Message: msg},
		ID:      id,
	}); err != nil {
		fmt.Fprintln(s.errOut, "kron serve-mcp: write error response:", err)
	}
}

// classifyError maps a Go error from a tool handler to a JSON-RPC
// error code. Domain sentinels get domain-specific codes; anything
// else is treated as an internal failure (-32603).
//
// The mapping is documented in docs/implementation/error-catalog.md §3
// and must stay aligned with that table.
func classifyError(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, model.ErrIntentNotFound),
		errors.Is(err, model.ErrIntentExists),
		errors.Is(err, model.ErrSlugInvalid):
		return codeInvalidParams
	case errors.Is(err, model.ErrAnchorDangling),
		errors.Is(err, model.ErrFrontmatterInvalid),
		errors.Is(err, model.ErrConfigInvalid):
		return codeInternalError
	default:
		return codeInternalError
	}
}
