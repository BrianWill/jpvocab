package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// JSON-RPC 2.0 envelopes
type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type historyEntry struct {
	Role string `json:"role"` // "user" | "bot" | "error"
	Text string `json:"text"`
}

// Web protocol
type webMessage struct {
	Type    string         `json:"type"`
	Content string         `json:"content,omitempty"`
	History []historyEntry `json:"history,omitempty"`
}

var (
	upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	wsMutex  sync.Mutex
	activeWs *websocket.Conn

	geminiCmd    *exec.Cmd
	geminiStdin  io.WriteCloser
	geminiStdout io.ReadCloser
	stdinMutex   sync.Mutex

	pendingRPC   = make(map[string]chan *rpcResponse)
	rpcMutex     sync.Mutex
	rpcIDCounter int

	sessionID string

	// Project root that gemini is rooted at; remembered so we can restart.
	projectRoot string

	// Serialize prompts and lifecycle transitions.
	// Acquired by runPrompt and restartGemini.
	promptMutex sync.Mutex

	// Per-turn state (chunks + accumulated text), guarded by historyMutex.
	turnChunkCount  int
	currentTurnText strings.Builder

	historyMutex sync.Mutex
	history      []historyEntry
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("getwd:", err)
	}
	projectRoot = strings.TrimSuffix(cwd, "/web")

	fmt.Println("Starting Gemini ACP session...")
	if err := startGemini(); err != nil {
		log.Fatal("start gemini:", err)
	}
	fmt.Printf("Session ready (id=%s)\n", sessionID)

	go handleTerminalInput()

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/ws", handleWs)

	port := "8090"
	fmt.Printf("Web interface: http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// startGemini launches the gemini --acp child process and runs initACP.
// Must be called with promptMutex held during a restart (main() calls it
// before any prompt goroutines exist).
func startGemini() error {
	cmd := exec.Command("gemini", "--acp", "--skip-trust", "--yolo")
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("exec gemini: %w", err)
	}

	geminiCmd = cmd
	geminiStdin = stdin
	geminiStdout = stdout

	go handleGeminiOutput(stdout)

	if err := initACP(projectRoot); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	return nil
}

func stopGemini() {
	if geminiCmd == nil || geminiCmd.Process == nil {
		return
	}
	_ = geminiCmd.Process.Kill()
	_ = geminiCmd.Wait() // pipes close, output goroutine exits

	rpcMutex.Lock()
	for id, ch := range pendingRPC {
		// Unblock any in-flight callers with an error response.
		select {
		case ch <- &rpcResponse{Error: &rpcError{Code: -32000, Message: "session restarted"}}:
		default:
		}
		delete(pendingRPC, id)
	}
	rpcIDCounter = 0
	rpcMutex.Unlock()

	geminiCmd = nil
	geminiStdin = nil
	geminiStdout = nil
	sessionID = ""
}

// restartGemini tears down the current gemini process, clears history,
// and starts a fresh session. Safe to call from a WS handler.
func restartGemini() error {
	promptMutex.Lock()
	defer promptMutex.Unlock()

	stopGemini()

	historyMutex.Lock()
	history = nil
	turnChunkCount = 0
	currentTurnText.Reset()
	historyMutex.Unlock()

	return startGemini()
}

func initACP(cwd string) error {
	resp, err := callRPC("initialize", map[string]interface{}{
		"protocolVersion": 1,
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]string{"name": "jp-tutor-web", "version": "1.0.0"},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize: %s", resp.Error.Message)
	}

	resp, err = callRPC("session/new", map[string]interface{}{
		"cwd":        cwd,
		"mcpServers": []interface{}{},
	})
	if err != nil {
		return fmt.Errorf("session/new: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("session/new: %s", resp.Error.Message)
	}

	var sn struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(resp.Result, &sn); err != nil {
		return fmt.Errorf("decode session/new result: %w", err)
	}
	if sn.SessionID == "" {
		return fmt.Errorf("session/new returned no sessionId")
	}
	sessionID = sn.SessionID
	return nil
}

func handleGeminiOutput(stdout io.ReadCloser) {
	scanner := bufio.NewScanner(stdout)
	// Some notifications (e.g. available_commands_update) are larger than the default 64KB cap.
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var head struct {
			ID     interface{} `json:"id"`
			Method string      `json:"method"`
		}
		if err := json.Unmarshal([]byte(line), &head); err != nil {
			log.Printf("[gemini] non-JSON: %s", line)
			continue
		}

		switch {
		case head.ID != nil && head.Method == "":
			var resp rpcResponse
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				log.Printf("decode response: %v", err)
				continue
			}
			idStr := fmt.Sprintf("%v", resp.ID)
			rpcMutex.Lock()
			ch, ok := pendingRPC[idStr]
			if ok {
				delete(pendingRPC, idStr)
			}
			rpcMutex.Unlock()
			if ok {
				ch <- &resp
			}

		case head.ID != nil && head.Method != "":
			handleAgentRequest(line)

		case head.Method != "":
			handleAgentNotification(line)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("scanner error: %v", err)
	}
}

func handleAgentNotification(line string) {
	var n struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal([]byte(line), &n); err != nil {
		return
	}

	if n.Method != "session/update" {
		return
	}

	var p struct {
		SessionID string `json:"sessionId"`
		Update    struct {
			SessionUpdate string `json:"sessionUpdate"`
			Content       struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"update"`
	}
	if err := json.Unmarshal(n.Params, &p); err != nil {
		return
	}

	switch p.Update.SessionUpdate {
	case "agent_message_chunk":
		if p.Update.Content.Text != "" {
			historyMutex.Lock()
			turnChunkCount++
			currentTurnText.WriteString(p.Update.Content.Text)
			historyMutex.Unlock()
			fmt.Print(p.Update.Content.Text)
			sendToWeb(webMessage{Type: "chunk", Content: p.Update.Content.Text})
		}
	case "agent_thought_chunk":
		if p.Update.Content.Text != "" {
			sendToWeb(webMessage{Type: "status", Content: "thinking..."})
		}
	}
}

func handleAgentRequest(line string) {
	var req struct {
		ID     interface{}     `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return
	}

	switch req.Method {
	case "session/request_permission":
		var p struct {
			Options []struct {
				OptionID string `json:"optionId"`
				Kind     string `json:"kind"`
			} `json:"options"`
		}
		_ = json.Unmarshal(req.Params, &p)
		chosen := "allow_once"
		for _, o := range p.Options {
			if o.Kind == "allow_always" || o.OptionID == "allow_always" {
				chosen = o.OptionID
				break
			}
			if o.OptionID != "" {
				chosen = o.OptionID
			}
		}
		respondRPC(req.ID, map[string]interface{}{
			"outcome": map[string]interface{}{
				"outcome":  "selected",
				"optionId": chosen,
			},
		}, nil)
	default:
		respondRPC(req.ID, nil, &rpcError{
			Code:    -32601,
			Message: "Method not found: " + req.Method,
		})
	}
}

func handleTerminalInput() {
	reader := bufio.NewReader(os.Stdin)
	for {
		text, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		fmt.Printf("[Terminal User]: %s\n", text)
		sendToWeb(webMessage{Type: "echo-user", Content: text})
		go runPrompt(text)
	}
}

func handleWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	wsMutex.Lock()
	activeWs = ws
	wsMutex.Unlock()

	defer func() {
		wsMutex.Lock()
		if activeWs == ws {
			activeWs = nil
		}
		wsMutex.Unlock()
		ws.Close()
	}()

	// Replay history to the new connection.
	sendHistorySnapshot()

	for {
		var msg webMessage
		if err := ws.ReadJSON(&msg); err != nil {
			return
		}
		switch msg.Type {
		case "chat":
			if msg.Content == "" {
				continue
			}
			fmt.Printf("\n[Web User]: %s\n", msg.Content)
			go runPrompt(msg.Content)
		case "restart":
			go func() {
				fmt.Println("\n[Restart] tearing down session...")
				if err := restartGemini(); err != nil {
					log.Printf("restart failed: %v", err)
					sendToWeb(webMessage{Type: "error", Content: "Restart failed: " + err.Error()})
					return
				}
				fmt.Printf("[Restart] new session ready (id=%s)\n", sessionID)
				sendHistorySnapshot()
			}()
		}
	}
}

func sendHistorySnapshot() {
	historyMutex.Lock()
	snap := make([]historyEntry, len(history))
	copy(snap, history)
	historyMutex.Unlock()
	sendToWeb(webMessage{Type: "history", History: snap})
}

func appendHistory(role, text string) {
	if text == "" {
		return
	}
	historyMutex.Lock()
	history = append(history, historyEntry{Role: role, Text: text})
	historyMutex.Unlock()
}

func runPrompt(text string) {
	promptMutex.Lock()
	defer promptMutex.Unlock()

	historyMutex.Lock()
	turnChunkCount = 0
	currentTurnText.Reset()
	history = append(history, historyEntry{Role: "user", Text: text})
	historyMutex.Unlock()

	resp, err := callRPC("session/prompt", map[string]interface{}{
		"sessionId": sessionID,
		"prompt": []map[string]interface{}{
			{"type": "text", "text": text},
		},
	})
	if err != nil {
		log.Printf("session/prompt: %v", err)
		appendHistory("error", err.Error())
		sendToWeb(webMessage{Type: "error", Content: err.Error()})
		sendToWeb(webMessage{Type: "done"})
		return
	}
	if resp.Error != nil {
		log.Printf("session/prompt error: %s", resp.Error.Message)
		appendHistory("error", resp.Error.Message)
		sendToWeb(webMessage{Type: "error", Content: resp.Error.Message})
		sendToWeb(webMessage{Type: "done"})
		return
	}

	var result struct {
		StopReason string `json:"stopReason"`
	}
	_ = json.Unmarshal(resp.Result, &result)

	fmt.Println()

	historyMutex.Lock()
	chunks := turnChunkCount
	replyText := currentTurnText.String()
	if chunks > 0 {
		history = append(history, historyEntry{Role: "bot", Text: replyText})
	}
	historyMutex.Unlock()

	if chunks == 0 {
		msg := emptyTurnMessage(result.StopReason)
		log.Printf("empty turn (stopReason=%q): %s", result.StopReason, msg)
		appendHistory("error", msg)
		sendToWeb(webMessage{Type: "error", Content: msg})
	} else if result.StopReason != "" && result.StopReason != "end_turn" {
		note := "(stopped: " + result.StopReason + ")"
		appendHistory("error", note)
		sendToWeb(webMessage{Type: "error", Content: note})
	}

	sendToWeb(webMessage{Type: "done"})
}

func emptyTurnMessage(stopReason string) string {
	switch stopReason {
	case "refusal":
		return "The model refused to answer."
	case "cancelled":
		return "The turn was cancelled."
	case "max_tokens":
		return "The model hit the output token limit before producing any text."
	case "max_turn_requests":
		return "The model hit the per-turn request limit before replying."
	case "", "end_turn":
		return "The model finished without sending a reply."
	default:
		return "The model finished without sending a reply (stopReason=" + stopReason + ")."
	}
}

func callRPC(method string, params interface{}) (*rpcResponse, error) {
	rpcMutex.Lock()
	rpcIDCounter++
	id := fmt.Sprintf("%d", rpcIDCounter)
	respCh := make(chan *rpcResponse, 1)
	pendingRPC[id] = respCh
	rpcMutex.Unlock()

	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	if err := writeJSON(req); err != nil {
		rpcMutex.Lock()
		delete(pendingRPC, id)
		rpcMutex.Unlock()
		return nil, err
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(5 * time.Minute):
		rpcMutex.Lock()
		delete(pendingRPC, id)
		rpcMutex.Unlock()
		return nil, fmt.Errorf("timeout waiting for %s", method)
	}
}

func respondRPC(id interface{}, result interface{}, errObj *rpcError) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
	}
	if errObj != nil {
		resp["error"] = errObj
	} else {
		resp["result"] = result
	}
	if err := writeJSON(resp); err != nil {
		log.Printf("respondRPC write: %v", err)
	}
}

func writeJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	stdinMutex.Lock()
	defer stdinMutex.Unlock()
	if geminiStdin == nil {
		return fmt.Errorf("gemini stdin not available")
	}
	if _, err := geminiStdin.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func sendToWeb(msg webMessage) {
	wsMutex.Lock()
	defer wsMutex.Unlock()
	if activeWs != nil {
		_ = activeWs.WriteJSON(msg)
	}
}
