package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/example/game-statement-export/internal/statement"
)

func main() {
	key := strings.TrimSpace(os.Getenv("INFRAI_API_KEY"))
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	baseURL := os.Getenv("INFRAI_BASE_URL")
	exporter := statement.NewExporter(statement.NewClient(baseURL, key, nil))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /statements", statementHandler(exporter))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("statement service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

type statementExporter interface {
	Export(ctx context.Context, input statement.ExportRequest) (statement.ExportResult, error)
}

func statementHandler(exporter statementExporter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var input statement.ExportRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
			return
		}
		result, err := exporter.Export(r.Context(), input)
		if err != nil {
			status := http.StatusBadGateway
			var apiErr *statement.APIError
			if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
				status = apiErr.HTTPStatus
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, result)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
