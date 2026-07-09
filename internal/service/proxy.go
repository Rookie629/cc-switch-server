package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ProxyService runs a local HTTP proxy that translates Anthropic API requests
// to OpenAI-compatible format and forwards them to the target provider.
type ProxyService struct {
	mu         sync.Mutex
	port       int
	providerID string
	running    bool
	server     *http.Server
	apiKey     string
	baseURL    string
	modelMap   map[string]string // anthropic model → target model
}

// NewProxyService creates a new proxy service.
func NewProxyService() *ProxyService {
	return &ProxyService{}
}

// Start launches the translation proxy on a random port in the given range.
func (ps *ProxyService) Start(apiKey, baseURL string, models []string, defaultModel string) (int, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.running {
		return ps.port, nil
	}

	port, err := findAvailablePort(19876, 19999)
	if err != nil {
		return 0, fmt.Errorf("find available port: %w", err)
	}

	ps.port = port
	ps.apiKey = apiKey
	ps.baseURL = strings.TrimRight(baseURL, "/")
	ps.running = true

	// Build model mapping
	ps.modelMap = make(map[string]string)
	for _, m := range models {
		ps.modelMap[m] = m
	}
	if defaultModel != "" && len(models) > 0 {
		// Map any unlisted model to default
		ps.modelMap["*"] = defaultModel
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", ps.handleMessages)
	mux.HandleFunc("/v1/messages/", ps.handleMessages)
	mux.HandleFunc("/health", ps.handleHealth)

	ps.server = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	go func() {
		log.Printf("[proxy] starting on port %d, forwarding to %s", port, baseURL)
		if err := ps.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[proxy] server error: %v", err)
			ps.mu.Lock()
			ps.running = false
			ps.mu.Unlock()
		}
	}()

	return port, nil
}

// Stop gracefully shuts down the in-process proxy. For the background daemon,
// use KillDaemon() instead.
func (ps *ProxyService) Stop() error {
	// First try killing any daemon process (background mode)
	if err := KillDaemon(); err != nil {
		log.Printf("[proxy] kill daemon: %v", err)
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if !ps.running || ps.server == nil {
		return nil
	}

	ps.running = false
	return ps.server.Close()
}

// Port returns the current proxy port, or 0 if not running. Also checks daemon PID file.
func (ps *ProxyService) Port() int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.running && ps.port > 0 {
		return ps.port
	}
	// Check daemon PID file
	if port := daemonPort(); port > 0 {
		return port
	}
	return 0
}

// IsRunning returns whether the proxy is running (in-process or daemon).
func (ps *ProxyService) IsRunning() bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.running {
		return true
	}
	return daemonRunning()
}

// ListenAndServe runs the proxy server in the foreground (blocking).
// Used by proxy-daemon subcommand. Writes PID file with real port.
func (ps *ProxyService) ListenAndServe() error {
	ps.mu.Lock()
	if ps.running {
		ps.mu.Unlock()
		return fmt.Errorf("proxy already running")
	}

	port := ps.port
	if port == 0 {
		var err error
		port, err = findAvailablePort(19876, 19999)
		if err != nil {
			ps.mu.Unlock()
			return fmt.Errorf("find available port: %w", err)
		}
	}
	ps.port = port
	ps.running = true

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", ps.handleMessages)
	mux.HandleFunc("/v1/messages/", ps.handleMessages)
	mux.HandleFunc("/health", ps.handleHealth)

	ps.server = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}
	ps.mu.Unlock()

	// Write PID file with real port now that it's known
	pidFile := ProxyPIDFile()
	os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n%d", os.Getpid(), port)), 0600)

	log.Printf("[proxy] listening on 127.0.0.1:%d", port)

	return ps.server.ListenAndServe()
}

// Shutdown gracefully stops the foreground proxy server.
func (ps *ProxyService) Shutdown() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.server != nil {
		ps.server.Close()
	}
	ps.running = false
}

// handleHealth responds to health checks.
func (ps *ProxyService) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleMessages translates Anthropic → OpenAI and proxies the request.
func (ps *ProxyService) handleMessages(w http.ResponseWriter, r *http.Request) {
	// Read Anthropic request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var anthropicReq AnthropicRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Translate to OpenAI format
	openaiReq := ps.translateRequest(&anthropicReq)

	// Forward to upstream
	respBody, respHeaders, statusCode, err := ps.forwardRequest(openaiReq, anthropicReq.Stream)
	if err != nil {
		log.Printf("[proxy] forward error: %v", err)
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		return
	}

	// If streaming, translate SSE stream
	if anthropicReq.Stream {
		ps.handleStreamResponse(w, respBody, respHeaders, statusCode)
		return
	}

	// Non-streaming: translate response
	ps.handleNonStreamResponse(w, respBody, statusCode)
}

// handleStreamResponse translates OpenAI SSE to Anthropic named-event SSE.
// References:
//   Anthropic: https://docs.anthropic.com/en/api/messages-streaming
//   OpenAI:    https://platform.openai.com/docs/api-reference/chat/streaming
func (ps *ProxyService) handleStreamResponse(w http.ResponseWriter, body []byte, headers http.Header, statusCode int) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(statusCode)

	reader := bufio.NewReader(bytes.NewReader(body))

	// State machine for streaming translation
	type toolCallState struct {
		id        string
		name      string
		arguments strings.Builder
		open      bool
		anthIndex int
	}

	var (
		messageID     string
		model         string
		started       bool
		textBlockOpen bool
		currentIndex  int = -1 // start at -1 so first block gets index 0
		toolStates    = make(map[int]*toolCallState)
	)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimRight(line, "\r\n")

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk OpenAIChatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		delta := choice.Delta

		// --- message_start (first chunk only) ---
		if !started {
			started = true
			messageID = chunk.ID
			model = chunk.Model
			fmt.Fprintf(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"%s\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"%s\",\"content\":[],\"usage\":{\"input_tokens\":0}}}\n\n",
				escapeJSON(messageID), escapeJSON(model))
			flusher.Flush()
		}

		// --- Text content ---
		if delta.Content != "" {
			if !textBlockOpen {
				textBlockOpen = true
				currentIndex++
				fmt.Fprintf(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":%d,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n", currentIndex)
				flusher.Flush()
			}
			fmt.Fprintf(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":%d,\"delta\":{\"type\":\"text_delta\",\"text\":%s}}\n\n",
				currentIndex, marshalJSONString(delta.Content))
			flusher.Flush()
		}

		// --- Tool calls ---
		for _, tc := range delta.ToolCalls {
			ts, exists := toolStates[tc.Index]
			if !exists {
				ts = &toolCallState{anthIndex: -1}
				toolStates[tc.Index] = ts
			}

			// Accumulate id/name as they arrive
			if tc.ID != "" {
				ts.id = tc.ID
			}
			if tc.Function.Name != "" {
				ts.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				ts.arguments.WriteString(tc.Function.Arguments)
			}

			// Open tool_use block when id + name are known but not yet opened
			if !ts.open && ts.id != "" && ts.name != "" {
				// Close text block if open
				if textBlockOpen {
					fmt.Fprintf(w, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":%d}\n\n", currentIndex)
					textBlockOpen = false
				}
				currentIndex++
				ts.anthIndex = currentIndex
				ts.open = true
				fmt.Fprintf(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":%d,\"content_block\":{\"type\":\"tool_use\",\"id\":\"%s\",\"name\":\"%s\",\"input\":{}}}\n\n",
					ts.anthIndex, escapeJSON(ts.id), escapeJSON(ts.name))
				flusher.Flush()
			}

			// Emit input_json_delta if arguments arrived and block is open
			if ts.open && tc.Function.Arguments != "" {
				fmt.Fprintf(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":%d,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":%s}}\n\n",
					ts.anthIndex, marshalJSONString(tc.Function.Arguments))
				flusher.Flush()
			}
		}

		// --- Finish reason → close blocks, send message_delta ---
		if choice.FinishReason != "" {
			// Close text block
			if textBlockOpen {
				fmt.Fprintf(w, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":%d}\n\n", currentIndex)
				textBlockOpen = false
			}
			// Close any open tool blocks and mark them closed
			for _, ts := range toolStates {
				if ts.open {
					fmt.Fprintf(w, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":%d}\n\n", ts.anthIndex)
					ts.open = false // prevent double-close in cleanup below
				}
			}

			stopReason := "end_turn"
			switch choice.FinishReason {
			case "stop":
				stopReason = "end_turn"
			case "tool_calls":
				stopReason = "tool_use"
			case "length":
				stopReason = "max_tokens"
			}

			outputTokens := 0
			if chunk.Usage != nil {
				outputTokens = chunk.Usage.CompletionTokens
			}

			fmt.Fprintf(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"%s\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":%d}}\n\n",
				stopReason, outputTokens)
			flusher.Flush()
		}
	}

	// Close any still-open blocks
	if textBlockOpen {
		fmt.Fprintf(w, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":%d}\n\n", currentIndex)
	}
	for _, ts := range toolStates {
		if ts.open {
			fmt.Fprintf(w, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":%d}\n\n", ts.anthIndex)
		}
	}

	// Final message_stop
	fmt.Fprintf(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	flusher.Flush()
}

// escapeJSON escapes a string for safe embedding in JSON string literals.
func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

// marshalJSONString returns a JSON-encoded string (with quotes).
func marshalJSONString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// handleNonStreamResponse translates a non-streaming OpenAI response to Anthropic format.
func (ps *ProxyService) handleNonStreamResponse(w http.ResponseWriter, body []byte, statusCode int) {
	var openaiResp OpenAIChatResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		log.Printf("[proxy] parse upstream response: %v", err)
		http.Error(w, "parse upstream response", http.StatusBadGateway)
		return
	}

	if len(openaiResp.Choices) == 0 {
		log.Printf("[proxy] upstream returned empty choices")
		http.Error(w, "empty response from upstream", http.StatusBadGateway)
		return
	}

	choice := openaiResp.Choices[0]
	msg := choice.Message

	// Build Anthropic content blocks
	content := make([]AnthropicContentBlock, 0)

	// Text content (may be "" when response is tool_calls only)
	if msg.Content != "" {
		content = append(content, AnthropicContentBlock{
			Type: "text",
			Text: msg.Content,
		})
	}

	// Tool calls → tool_use blocks
	stopReason := "end_turn"
	for _, tc := range msg.ToolCalls {
		var args map[string]interface{}
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				log.Printf("[proxy] failed to parse tool arguments: %v (raw: %s)", err, tc.Function.Arguments[:min(len(tc.Function.Arguments), 200)])
				args = make(map[string]interface{})
			}
		}
		if args == nil {
			args = make(map[string]interface{})
		}
		content = append(content, AnthropicContentBlock{
			Type:   "tool_use",
			ID:     tc.ID,
			Name:   tc.Function.Name,
			Input:  args,
			Caller: map[string]string{"type": "direct"},
		})
		stopReason = "tool_use"
	}

	if choice.FinishReason == "tool_calls" {
		stopReason = "tool_use"
	} else if choice.FinishReason == "stop" {
		stopReason = "end_turn"
	} else if choice.FinishReason == "length" {
		stopReason = "max_tokens"
	}

	anthropicResp := AnthropicResponse{
		ID:         openaiResp.ID,
		Type:       "message",
		Role:       "assistant",
		Model:      openaiResp.Model,
		Content:    content,
		StopReason: stopReason,
		Usage: AnthropicUsage{
			InputTokens:  openaiResp.Usage.PromptTokens,
			OutputTokens: openaiResp.Usage.CompletionTokens,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(anthropicResp)
}

// translateRequest converts an Anthropic request to OpenAI format.
func (ps *ProxyService) translateRequest(ar *AnthropicRequest) OpenAIRequest {
	model := ps.resolveModel(ar.Model)
	messages := ps.translateMessages(ar)

	maxTokens := ar.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	or := OpenAIRequest{
		Model:       model,
		Messages:    messages,
		Stream:      ar.Stream,
		MaxTokens:   maxTokens,
		Temperature: ar.Temperature,
		Tools:       ps.translateTools(ar.Tools),
	}

	if ar.TopP > 0 {
		or.TopP = ar.TopP
	}

	return or
}

// translateTools converts Anthropic tool definitions to OpenAI format.
// Anthropic: [{name, description, input_schema}]
// OpenAI:     [{type: "function", function: {name, description, parameters}}]
func (ps *ProxyService) translateTools(raw json.RawMessage) []OpenAITool {
	if len(raw) == 0 {
		return nil
	}
	var anthropicTools []struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"input_schema"`
	}
	if err := json.Unmarshal(raw, &anthropicTools); err != nil {
		return nil
	}
	tools := make([]OpenAITool, 0, len(anthropicTools))
	for _, at := range anthropicTools {
		tools = append(tools, OpenAITool{
			Type: "function",
			Function: OpenAIToolFunction{
				Name:        at.Name,
				Description: at.Description,
				Parameters:  at.InputSchema,
			},
		})
	}
	return tools
}

// resolveModel maps an Anthropic model name to the target provider's model.
func (ps *ProxyService) resolveModel(anthropicModel string) string {
	if m, ok := ps.modelMap[anthropicModel]; ok {
		return m
	}
	if d, ok := ps.modelMap["*"]; ok {
		return d
	}
	return anthropicModel
}

// translateMessages converts Anthropic messages to OpenAI format.
// Preserves tool_use and tool_result as structured content (not flattened to text).
// Anthropic assistant messages with tool_use → OpenAI assistant with tool_calls[].
// Anthropic user messages with tool_result → OpenAI tool-role messages.
func (ps *ProxyService) translateMessages(ar *AnthropicRequest) []OpenAIMessage {
	var messages []OpenAIMessage

	// System prompt — handles both string and array-of-content-blocks
	if len(ar.System) > 0 {
		systemText := ps.parseSystem(ar.System)
		if systemText != "" {
			messages = append(messages, OpenAIMessage{
				Role:    "system",
				Content: systemText,
			})
		}
	}

	for _, msg := range ar.Messages {
		parsed := ps.parseContentBlocks(msg.Content, msg.Role)
		messages = append(messages, parsed...)
	}

	return messages
}

// parseContentBlocks splits an Anthropic message content into one or more OpenAI messages.
// Assistant messages with tool_use blocks get their tool calls preserved as structured JSON.
// User messages with tool_result blocks become OpenAI tool-role messages.
func (ps *ProxyService) parseContentBlocks(raw json.RawMessage, role string) []OpenAIMessage {
	var messages []OpenAIMessage

	// Try plain string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []OpenAIMessage{{Role: role, Content: s}}
	}

	// Try array of content blocks
	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return []OpenAIMessage{{Role: role, Content: ""}}
	}

	if role == "assistant" {
		// Separate text parts and tool_use blocks
		var textParts []string
		var toolCalls []OpenAIToolCall

		for _, b := range blocks {
			switch b.Type {
			case "text":
				if b.Text != "" {
					textParts = append(textParts, b.Text)
				}
			case "tool_use":
				argsJSON, _ := json.Marshal(b.Input)
				toolCalls = append(toolCalls, OpenAIToolCall{
					ID:   b.ID,
					Type: "function",
					Function: OpenAIToolFunction{
						Name:      b.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		}

		if len(toolCalls) > 0 || len(textParts) > 0 {
			messages = append(messages, OpenAIMessage{
				Role:      "assistant",
				Content:   strings.Join(textParts, "\n"),
				ToolCalls: toolCalls,
			})
		}
	} else {
		// User messages: text blocks → user message, tool_result blocks → tool messages
		var textParts []string
		for _, b := range blocks {
			switch b.Type {
			case "text":
				if b.Text != "" {
					textParts = append(textParts, b.Text)
				}
			case "tool_result":
				resultText := extractTextContent(b.ContentRaw)
				tid := b.ToolUseID
				if tid == "" {
					tid = b.ID
				}
				messages = append(messages, OpenAIMessage{
					Role:       "tool",
					Content:    resultText,
					ToolCallID: tid,
				})
			}
		}
		if len(textParts) > 0 {
			messages = append(messages, OpenAIMessage{
				Role:    "user",
				Content: strings.Join(textParts, "\n"),
			})
		}
	}

	return messages
}

// parseSystem handles system as either a string or [{type:"text", text:"..."}, ...].
func (ps *ProxyService) parseSystem(raw json.RawMessage) string {
	// Try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try array of content blocks
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// extractTextContent extracts the text from Anthropic content, which can be
// a plain string or an array of content blocks.
func extractTextContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try plain string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try array of content blocks
	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, b.Text)
		case "tool_use":
			// Serialize as proper JSON so the model can understand it
			argsJSON, _ := json.Marshal(b.Input)
			parts = append(parts, fmt.Sprintf("[ToolCall id=%s name=%s args=%s]", b.ID, b.Name, string(argsJSON)))
		case "tool_result":
			toolText := extractTextContent(b.ContentRaw)
			if toolText != "" {
				tid := b.ToolUseID
				if tid == "" {
					tid = b.ID // fallback
				}
				parts = append(parts, fmt.Sprintf("[ToolResult id=%s result=%s]", tid, toolText))
			}
		}
	}
	return strings.Join(parts, "\n")
}

// forwardRequest sends the OpenAI-format request to the upstream provider.
func (ps *ProxyService) forwardRequest(or OpenAIRequest, stream bool) ([]byte, http.Header, int, error) {
	body, err := json.Marshal(or)
	if err != nil {
		return nil, nil, 0, err
	}

	url := ps.baseURL + "/v1/chat/completions"

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ps.apiKey)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, 0, err
	}

	return respBody, resp.Header, resp.StatusCode, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// findAvailablePort finds an available port in the given range.
func findAvailablePort(start, end int) (int, error) {
	for port := start; port <= end; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", start, end)
}

// ---- Daemon process management ----

// ProxyPIDFile returns the path to the proxy daemon PID file.
func ProxyPIDFile() string {
	return filepath.Join(storeDir(), "proxy.pid")
}

// storeDir returns the data directory (matches store.DefaultDataDir).
func storeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cc-switch-server")
}

// NewProxyDaemon creates a ProxyService pre-configured for daemon mode.
func NewProxyDaemon(apiKey, baseURL string, models []string, defaultModel string, port int) *ProxyService {
	ps := &ProxyService{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
	ps.modelMap = make(map[string]string)
	for _, m := range models {
		ps.modelMap[m] = m
	}
	if defaultModel != "" && len(models) > 0 {
		ps.modelMap["*"] = defaultModel
	}
	if port > 0 {
		ps.port = port
	}
	return ps
}

// daemonRunning checks if a proxy daemon process is currently running.
func daemonRunning() bool {
	data, err := os.ReadFile(ProxyPIDFile())
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(strings.Split(string(data), "\n")[0]))
	if err != nil || pid <= 0 {
		return false
	}
	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// daemonPort reads the proxy port from the PID file.
func daemonPort() int {
	data, err := os.ReadFile(ProxyPIDFile())
	if err != nil {
		return 0
	}
	parts := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(parts) < 2 {
		return 0
	}
	port, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	return port
}

// KillDaemon stops a running proxy daemon by sending SIGTERM.
func KillDaemon() error {
	data, err := os.ReadFile(ProxyPIDFile())
	if err != nil {
		return nil // no daemon running
	}
	pid, err := strconv.Atoi(strings.TrimSpace(strings.Split(string(data), "\n")[0]))
	if err != nil || pid <= 0 {
		os.Remove(ProxyPIDFile())
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(ProxyPIDFile())
		return nil
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		os.Remove(ProxyPIDFile())
		return nil
	}
	os.Remove(ProxyPIDFile())
	return nil
}

// SpawnProxyDaemon launches a proxy daemon as a background process.
// Returns the port the daemon is listening on.
func SpawnProxyDaemon(apiKey, baseURL string, models []string, defaultModel string) (int, error) {
	// Kill any existing daemon first
	KillDaemon()

	// Build args
	bin, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("find executable: %w", err)
	}

	args := []string{
		"proxy-daemon",
		"--api-key", apiKey,
		"--base-url", baseURL,
	}
	for _, m := range models {
		args = append(args, "--models", m)
	}
	if defaultModel != "" {
		args = append(args, "--default-model", defaultModel)
	}

	cmd := exec.Command(bin, args...)
	// Detach from parent: set PGID so the daemon survives parent exit
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start daemon: %w", err)
	}

	// Wait briefly for PID file to appear with a real port
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if daemonRunning() && daemonPort() > 0 {
			return daemonPort(), nil
		}
		// Check if process died
		if err := cmd.Wait(); err != nil {
			// Process exited — check for error
			return 0, fmt.Errorf("daemon exited early")
		}
	}

	// Timeout — process might still be starting, check PID file one more time
	if daemonRunning() {
		return daemonPort(), nil
	}
	return 0, fmt.Errorf("daemon did not start within 2 seconds")
}

// ----- Anthropic data types -----

// AnthropicRequest is the incoming request from Claude Code.
type AnthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []AnthropicMessage `json:"messages"`
	System      json.RawMessage    `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Stream      bool               `json:"stream"`
	Temperature float64            `json:"temperature,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
	Tools       json.RawMessage    `json:"tools,omitempty"`
}

// AnthropicMessage is a single message in the conversation.
type AnthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// AnthropicContentBlock is a block within an Anthropic message.
type AnthropicContentBlock struct {
	Type       string                 `json:"type"`
	Text       string                 `json:"text,omitempty"`
	Name       string                 `json:"name,omitempty"`
	Input      map[string]interface{} `json:"input"`
	ContentRaw json.RawMessage        `json:"content,omitempty"`
	ID         string                 `json:"id,omitempty"`
	ToolUseID  string                 `json:"tool_use_id,omitempty"`
	Caller     map[string]string      `json:"caller,omitempty"`
	IsError    *bool                  `json:"is_error,omitempty"`
}

// AnthropicResponse is the translated response sent back to Claude Code.
type AnthropicResponse struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Role        string                 `json:"role"`
	Model       string                 `json:"model"`
	Content     []AnthropicContentBlock `json:"content"`
	StopReason  string                 `json:"stop_reason"`
	StopDetails interface{}            `json:"stop_details"`
	Usage       AnthropicUsage         `json:"usage"`
}

// AnthropicUsage contains token counts.
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ----- OpenAI data types -----

// OpenAIRequest is the translated request sent to the upstream provider.
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	Tools       []OpenAITool    `json:"tools,omitempty"`
}

// OpenAITool is a tool definition in OpenAI format.
type OpenAITool struct {
	Type     string             `json:"type"`
	Function OpenAIToolFunction `json:"function"`
}

// OpenAIMessage is a single message in the conversation.
type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// OpenAIChatResponse is the upstream provider's response.
type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

// OpenAIChoice is a single choice in the response.
type OpenAIChoice struct {
	Index        int              `json:"index"`
	Message      OpenAIRespMessage `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

// OpenAIRespMessage is the assistant message in a response (may have tool_calls).
type OpenAIRespMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

// OpenAIUsage contains token counts.
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIChatChunk is a single SSE chunk from the upstream provider.
type OpenAIChatChunk struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Choices []OpenAIChunkChoice `json:"choices"`
	Usage   *OpenAIUsage       `json:"usage,omitempty"`
}

// OpenAIChunkChoice is a single choice in a streaming chunk.
type OpenAIChunkChoice struct {
	Index        int            `json:"index"`
	Delta        OpenAIDelta    `json:"delta"`
	FinishReason string         `json:"finish_reason,omitempty"`
}

// OpenAIDelta is the delta content in a streaming chunk.
type OpenAIDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

// OpenAIToolCall is a tool call in the delta or response message.
type OpenAIToolCall struct {
	Index    int                `json:"index"`
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function OpenAIToolFunction `json:"function"`
}

// OpenAIToolFunction is the function call details.
type OpenAIToolFunction struct {
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Arguments   string          `json:"arguments,omitempty"`
}
