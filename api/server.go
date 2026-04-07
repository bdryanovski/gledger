// Package api provides the DoubleBook REST API server running on port 5555.
// All endpoints return JSON. The server bridges web/external clients to the
// core DoubleBook engine without implementing any new business logic.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"doublebook/api/handlers"
	"doublebook/currency"
	"doublebook/db"
	Interpreter "doublebook/interpreter"
)

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

// Server holds the API server state.
type Server struct {
	port      int
	interp    *Interpreter.Interpreter
	db        *db.DB
	converter *currency.CachingConverter
	mux       *http.ServeMux
}

// NewServer creates a new API server.
func NewServer(
	port int,
	interp *Interpreter.Interpreter,
	database *db.DB,
	conv *currency.CachingConverter,
) *Server {
	s := &Server{
		port:      port,
		interp:    interp,
		db:        database,
		converter: conv,
		mux:       http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// Handler returns the http.Handler (used by the web server to mount under /api).
func (s *Server) Handler() http.Handler {
	return chain(s.mux, corsMiddleware, jsonMiddleware, loggerMiddleware, recoveryMiddleware)
}

// Start starts the HTTP server and blocks until it is stopped.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	log.Printf("DoubleBook API listening on http://localhost%s", addr)
	return srv.ListenAndServe()
}

// ---------------------------------------------------------------------------
// Route registration
// ---------------------------------------------------------------------------

func (s *Server) registerRoutes() {
	h := handlers.New(s.interp, s.db, s.converter)

	s.mux.HandleFunc("/api/transactions", h.ListTransactions)
	s.mux.HandleFunc("/api/accounts", h.ListAccounts)
	s.mux.HandleFunc("/api/reports/balance", h.BalanceReport)
	s.mux.HandleFunc("/api/reports/income-statement", h.IncomeStatement)
	s.mux.HandleFunc("/api/fql", h.FQLQuery)
	s.mux.HandleFunc("/api/exchange-rates", h.ExchangeRate)
	s.mux.HandleFunc("/api/import", h.ImportCSV)
	s.mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		respond(w, http.StatusOK, map[string]string{"status": "ok", "version": "0.1.0"})
	})
}

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

type middleware func(http.Handler) http.Handler

func chain(h http.Handler, ms ...middleware) http.Handler {
	for i := len(ms) - 1; i >= 0; i-- {
		h = ms[i](h)
	}
	return h
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				respondError(w, http.StatusInternalServerError, fmt.Sprintf("internal error: %v", rec))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// ---------------------------------------------------------------------------
// Response helpers
// ---------------------------------------------------------------------------

func respond(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data) //nolint:errcheck
}

func respondError(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
