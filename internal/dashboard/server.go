// internal/dashboard/server.go
package dashboard

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

var ErrDashboard = errors.New("invalid dashboard configuration")

const cspValue = "default-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

type Config struct {
	BindAddr string
	Token    string
	Store    *Store
}

type Server struct {
	tokenHash [32]byte
	mux       *http.ServeMux
	store     *Store
}

func loopbackHost(addr string) (string, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	if host == "" {
		return "", fmt.Errorf("empty host")
	}
	if strings.EqualFold(host, "localhost") {
		return host, nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", fmt.Errorf("not loopback")
	}
	return host, nil
}

func NewServer(cfg Config) (*Server, error) {
	if _, err := loopbackHost(cfg.BindAddr); err != nil {
		return nil, fmt.Errorf("%w: bind %q: %v", ErrDashboard, cfg.BindAddr, err)
	}
	if len([]byte(cfg.Token)) < 16 {
		return nil, fmt.Errorf("%w: operator token too short", ErrDashboard)
	}
	store := cfg.Store
	if store == nil {
		store = NewStore()
	}
	s := &Server{tokenHash: sha256.Sum256([]byte(cfg.Token)), mux: http.NewServeMux(), store: store}
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /items/{id}", s.handleItem)
	s.mux.HandleFunc("POST /api/commands", s.handleCommand)
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.withCSP(s.withAuth(s.mux))
}

func (s *Server) withCSP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", cspValue)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authorized(r *http.Request) bool {
	const prefix = "Bearer "
	got := r.Header.Get("Authorization")
	if !strings.HasPrefix(got, prefix) {
		return false
	}
	sum := sha256.Sum256([]byte(strings.TrimPrefix(got, prefix)))
	return subtle.ConstantTimeCompare(sum[:], s.tokenHash[:]) == 1
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
			origin := r.Header.Get("Origin")
			if origin != "http://"+r.Host && origin != "https://"+r.Host {
				http.Error(w, "foreign origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Task 1 stubs: replaced by real views (Task 2) and intake (Task 3).
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleItem(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not found", http.StatusNotFound)
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
