package subserver

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/BX-Team/Nexon/internal/core"
	"github.com/BX-Team/Nexon/internal/store"
)

// rulesReloadTTL bounds how stale the UA rules / client-app caches may get:
// CLI/TUI edits run in a separate process and cannot invalidate them directly.
const rulesReloadTTL = 30 * time.Second

// Server is the subscription HTTP server.
type Server struct {
	svc      *core.Service
	detector *Detector
	baseURL  string

	reloadMu   sync.Mutex
	lastReload time.Time
}

// New builds a subscription server.
func New(svc *core.Service, baseURL string) (*Server, error) {
	d, err := NewDetector(svc.Store())
	if err != nil {
		return nil, err
	}
	return &Server{svc: svc, detector: d, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

// Handler returns the http.Handler for mounting.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/sub/", s.handleSub)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	return mux
}

// refreshRules re-reads the UA detection rules and managed client apps from
// the store at most once per TTL, so CLI edits reach a running server.
func (s *Server) refreshRules() {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	if time.Since(s.lastReload) < rulesReloadTTL {
		return
	}
	s.lastReload = time.Now()
	_ = s.detector.Reload(s.svc.Store())
	_ = s.svc.ReloadClients()
}

func (s *Server) handleSub(w http.ResponseWriter, r *http.Request) {
	s.refreshRules()
	token := strings.TrimPrefix(r.URL.Path, "/sub/")
	token = strings.Trim(token, "/")
	if token == "" {
		http.NotFound(w, r)
		return
	}
	ua := r.Header.Get("User-Agent")
	hwid := firstHeader(r, "x-hwid", "X-HWID")
	ip := clientIP(r)

	res, err := s.svc.Subscription(token, ua, hwid, ip)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if errors.Is(err, core.ErrSubDenied) {
			http.Error(w, "subscription unavailable", http.StatusForbidden)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Browser → HTML dashboard with QR.
	if IsBrowser(ua) {
		s.renderDashboard(w, res, token)
		return
	}

	// A client app may pin an explicit output format (e.g. mihomo → clash);
	// otherwise fall back to the UA detection rules.
	format := s.svc.SubFormat(ua)
	if format == "" {
		format = s.detector.Detect(ua)
	}
	body, ctype := s.svc.RenderSubscription(format, res.User, res.Endpoints)

	// Subscription-userinfo header so clients can show quota/expiry.
	w.Header().Set("Subscription-Userinfo", subUserinfo(res.User))
	w.Header().Set("Profile-Update-Interval", "12")
	// Set global profile headers and per-client-app custom headers.
	for k, v := range s.svc.SubResponseHeaders(ua) {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", ctype)
	w.Write(body)
}

func subUserinfo(u *store.User) string {
	var expire int64
	if u.ExpireAt != nil {
		expire = u.ExpireAt.Unix()
	}
	// upload/download split is not tracked separately yet; report total as down.
	return fmt.Sprintf("upload=0; download=%d; total=%d; expire=%d", u.UsedTraffic, u.DataLimit, expire)
}

func firstHeader(r *http.Request, keys ...string) string {
	for _, k := range keys {
		if v := r.Header.Get(k); v != "" {
			return v
		}
	}
	return ""
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
