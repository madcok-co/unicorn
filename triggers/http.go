// ============================================
// 1. HTTP TRIGGER (Smart Routing)
// ============================================
package triggers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/madcok-co/unicorn"
)

type HTTPTrigger struct {
	addr     string
	server   *http.Server
	services map[string]*unicorn.Definition
	mu       sync.RWMutex
}

func NewHTTPTrigger(addr string) *HTTPTrigger {
	return &HTTPTrigger{
		addr:     addr,
		services: make(map[string]*unicorn.Definition),
	}
}

func (t *HTTPTrigger) RegisterService(def *unicorn.Definition) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.services[def.Name] = def
	return nil
}

func (t *HTTPTrigger) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", t.handleRequest)

	t.server = &http.Server{
		Addr:    t.addr,
		Handler: mux,
	}

	return t.server.ListenAndServe()
}

func (t *HTTPTrigger) Stop() error {
	if t.server != nil {
		return t.server.Shutdown(context.Background())
	}
	return nil
}

func (t *HTTPTrigger) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Extract service name from path: /api/{service}
	path := strings.TrimPrefix(r.URL.Path, "/api/")
	serviceName := strings.Split(path, "/")[0]

	// Get service
	t.mu.RLock()
	def := t.services[serviceName]
	t.mu.RUnlock()

	if def == nil {
		t.writeError(w, http.StatusNotFound, "service not found")
		return
	}

	// Parse request
	var request map[string]interface{}

	// URL params + Query params
	request = make(map[string]interface{})
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			request[key] = values[0]
		}
	}

	// Body (if POST/PUT)
	if r.Method == "POST" || r.Method == "PUT" {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			for k, v := range body {
				request[k] = v
			}
		}
	}

	// Create context
	ctx := unicorn.NewFullContext(r.Context(), nil, nil, unicorn.GetGlobalLogger())
	ctx.SetMetadata("service_name", serviceName)
	ctx.SetMetadata("http_method", r.Method)
	ctx.SetMetadata("http_path", r.URL.Path)
	ctx.SetMetadata("client_ip", r.RemoteAddr)

	// Execute service
	start := time.Now()
	result, err := def.Handler.Handle(ctx, request)
	duration := time.Since(start)

	// Write response
	if err != nil {
		t.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	t.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"data":       result,
		"request_id": ctx.RequestID(),
		"duration":   duration.String(),
	})
}

func (t *HTTPTrigger) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (t *HTTPTrigger) writeError(w http.ResponseWriter, status int, message string) {
	t.writeJSON(w, status, map[string]interface{}{
		"success": false,
		"error":   message,
	})
}
