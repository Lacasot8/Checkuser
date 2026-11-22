package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Host           string
	Port           int
	LimitsFile     string
	DragonCoreURL  string
	DragonCorePath string
	RequestTimeout time.Duration
}

type Result struct {
	OK         bool   `json:"ok"`
	User       string `json:"user"`
	UUID       string `json:"uuid,omitempty"`
	HWID       string `json:"hwid,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Source     string `json:"source"`
	Message    string `json:"message,omitempty"`
	CheckedAt  string `json:"checked_at"`
}

var safeValue = regexp.MustCompile(`^[A-Za-z0-9._:@+\-]{1,128}$`)

func main() {
	cfg := Config{
		Host: os.Getenv("CHECKUSER_HOST"), Port: envInt("CHECKUSER_PORT", 2052),
		LimitsFile: envString("CHECKUSER_LIMITS_FILE", "/root/usuarios.db"),
		DragonCoreURL: strings.TrimRight(os.Getenv("CHECKUSER_DRAGONCORE_URL"), "/"),
		DragonCorePath: envString("CHECKUSER_DRAGONCORE_PATH", "/check"),
		RequestTimeout: time.Duration(envInt("CHECKUSER_TIMEOUT_SECONDS", 5)) * time.Second,
	}
	if cfg.Host == "" { cfg.Host = "0.0.0.0" }
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
	mux.HandleFunc("/check", cfg.check)
	mux.HandleFunc("/", info)
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), Handler: logging(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("CheckUser ouvindo em %s; fallback=%s; dragoncore=%s", server.Addr, cfg.LimitsFile, cfg.DragonCoreURL)
	log.Fatal(server.ListenAndServe())
}

func (c Config) check(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodGet { writeJSON(w, http.StatusMethodNotAllowed, Result{Message: "use GET", CheckedAt: time.Now().UTC().Format(time.RFC3339) }); return }
	q := r.URL.Query()
	user, uuid, hwid := q.Get("user"), q.Get("uuid"), q.Get("hwid")
	if user == "" { user = q.Get("username") }
	if !safeValue.MatchString(user) || (uuid != "" && !safeValue.MatchString(uuid)) || (hwid != "" && !safeValue.MatchString(hwid)) {
		writeJSON(w, http.StatusBadRequest, Result{OK: false, User: user, Message: "parâmetros inválidos", Source: "validation", CheckedAt: time.Now().UTC().Format(time.RFC3339) }); return
	}
	if c.DragonCoreURL != "" {
		if result, ok := c.fromDragonCore(user, uuid, hwid); ok { writeJSON(w, http.StatusOK, result); return }
	}
	limit, found, err := readLimit(c.LimitsFile, user)
	if err != nil { writeJSON(w, http.StatusServiceUnavailable, Result{OK: false, User: user, UUID: uuid, HWID: hwid, Message: "fonte local indisponível", Source: "local", CheckedAt: time.Now().UTC().Format(time.RFC3339) }); return }
	if !found { writeJSON(w, http.StatusNotFound, Result{OK: false, User: user, UUID: uuid, HWID: hwid, Message: "usuário não encontrado", Source: "local", CheckedAt: time.Now().UTC().Format(time.RFC3339) }); return }
	writeJSON(w, http.StatusOK, Result{OK: true, User: user, UUID: uuid, HWID: hwid, Limit: limit, Source: "local", Message: "usuário encontrado", CheckedAt: time.Now().UTC().Format(time.RFC3339) })
}

func (c Config) fromDragonCore(user, uuid, hwid string) (Result, bool) {
	// O cliente possui timeout próprio para evitar que um DragonCore indisponível bloqueie o fallback local.
	client := &http.Client{Timeout: c.RequestTimeout}
	u, err := url.Parse(c.DragonCoreURL + c.DragonCorePath); if err != nil { return Result{}, false }
	v := u.Query(); v.Set("user", user); v.Set("uuid", uuid); v.Set("hwid", hwid); u.RawQuery = v.Encode()
	resp, err := client.Get(u.String()); if err != nil { return Result{}, false }; defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return Result{}, false }
	var raw map[string]interface{}; if json.NewDecoder(resp.Body).Decode(&raw) != nil { return Result{}, false }
	result := Result{OK: true, User: user, UUID: uuid, HWID: hwid, Source: "dragoncore", Message: "resposta do DragonCore", CheckedAt: time.Now().UTC().Format(time.RFC3339)}
	if value, ok := raw["ok"].(bool); ok { result.OK = value }
	if value, ok := raw["limit"].(float64); ok { result.Limit = int(value) }
	if value, ok := raw["message"].(string); ok { result.Message = value }
	return result, true
}

func readLimit(path, user string) (int, bool, error) {
	data, err := os.ReadFile(path); if err != nil { return 0, false, err }
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line); if len(fields) < 2 || strings.HasPrefix(fields[0], "#") || fields[0] != user { continue }
		limit, err := strconv.Atoi(fields[1]); if err != nil { return 0, false, errors.New("limite inválido") }; return limit, true, nil
	}
	return 0, false, nil
}
func health(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "service": "checkuser"}) }
func info(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"service": "checkuser", "endpoint": "/check?user=...&uuid=...&hwid=...", "health": "/health"}) }
func writeJSON(w http.ResponseWriter, status int, value interface{}) { w.WriteHeader(status); _ = json.NewEncoder(w).Encode(value) }
func logging(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { started := time.Now(); next.ServeHTTP(w, r); log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started)) }) }
func envString(key, fallback string) string { if v := os.Getenv(key); v != "" { return v }; return fallback }
func envInt(key string, fallback int) int { if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 { return v }; return fallback }
