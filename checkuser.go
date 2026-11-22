package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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
	OK               bool   `json:"ok"`
	User             string `json:"user"`
	Username         string `json:"username"`
	UUID             string `json:"uuid,omitempty"`
	HWID             string `json:"hwid,omitempty"`
	Limit            int    `json:"limit,omitempty"`
	ExpirationDate   string `json:"expiration_date"`
	ExpirationDays   int    `json:"expiration_days"`
	CountConnections int    `json:"count_connections"`
	LimitConnections int    `json:"limit_connections"`
	Source           string `json:"source"`
	Message          string `json:"message,omitempty"`
	CheckedAt        string `json:"checked_at"`
}

var safeValue = regexp.MustCompile(`^[A-Za-z0-9._:@+\-]{1,128}$`)

func main() {
	cfg := Config{
		Host: os.Getenv("CHECKUSER_HOST"), Port: envInt("CHECKUSER_PORT", 2052),
		LimitsFile:     envString("CHECKUSER_LIMITS_FILE", "/root/usuarios.db"),
		DragonCoreURL:  strings.TrimRight(os.Getenv("CHECKUSER_DRAGONCORE_URL"), "/"),
		DragonCorePath: envString("CHECKUSER_DRAGONCORE_PATH", "/check"),
		RequestTimeout: time.Duration(envInt("CHECKUSER_TIMEOUT_SECONDS", 5)) * time.Second,
	}
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
	mux.HandleFunc("/check", cfg.check)
	mux.HandleFunc("/", info)
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), Handler: logging(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("CheckUser ouvindo em %s; fallback=%s; dragoncore=%s", server.Addr, cfg.LimitsFile, cfg.DragonCoreURL)
	log.Fatal(server.ListenAndServe())
}

func (c Config) check(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, Result{Message: "use GET", CheckedAt: time.Now().UTC().Format(time.RFC3339)})
		return
	}
	q := r.URL.Query()
	user, uuid, hwid := q.Get("user"), q.Get("uuid"), q.Get("hwid")
	if user == "" {
		user = q.Get("username")
	}
	if !safeValue.MatchString(user) || (uuid != "" && !safeValue.MatchString(uuid)) || (hwid != "" && !safeValue.MatchString(hwid)) {
		writeJSON(w, http.StatusBadRequest, Result{OK: false, User: user, Username: user, Message: "parâmetros inválidos", Source: "validation", CheckedAt: time.Now().UTC().Format(time.RFC3339)})
		return
	}
	if c.DragonCoreURL != "" {
		if c.proxyDragonCore(w, user, uuid, hwid) {
			return
		}
	}
	limit, found, err := readLimit(c.LimitsFile, user)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, Result{OK: false, User: user, Username: user, UUID: uuid, HWID: hwid, Message: "fonte local indisponível", Source: "local", CheckedAt: time.Now().UTC().Format(time.RFC3339)})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, Result{OK: false, User: user, Username: user, UUID: uuid, HWID: hwid, Message: "usuário não encontrado", Source: "local", CheckedAt: time.Now().UTC().Format(time.RFC3339)})
		return
	}
	writeJSON(w, http.StatusOK, Result{OK: true, User: user, Username: user, UUID: uuid, HWID: hwid, Limit: limit, LimitConnections: limit, Source: "local", Message: "usuário encontrado; expiração e conexões ativas não disponíveis no fallback local", CheckedAt: time.Now().UTC().Format(time.RFC3339)})
}

func (c Config) proxyDragonCore(w http.ResponseWriter, user, uuid, hwid string) bool {
	// O cliente possui timeout próprio para evitar que um DragonCore indisponível bloqueie o fallback local.
	client := &http.Client{Timeout: c.RequestTimeout}
	u, err := url.Parse(c.DragonCoreURL + c.DragonCorePath)
	if err != nil {
		return false
	}
	v := u.Query()
	v.Set("user", user)
	v.Set("uuid", uuid)
	v.Set("hwid", hwid)
	u.RawQuery = v.Encode()
	resp, err := client.Get(u.String())
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	// Repassar HTML/texto sem alterar. Em JSON, preservamos todos os campos e só
	// acrescentamos os aliases e a mensagem HTML quando houver dados suficientes.
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "json") {
		var raw map[string]interface{}
		if json.Unmarshal(body, &raw) == nil {
			if _, ok := raw["user"]; !ok {
				raw["user"] = valueString(raw, "username", user)
			}
			if _, ok := raw["username"]; !ok {
				raw["username"] = valueString(raw, "user", user)
			}
			if _, ok := raw["uuid"]; !ok && uuid != "" {
				raw["uuid"] = uuid
			}
			if _, ok := raw["hwid"]; !ok && hwid != "" {
				raw["hwid"] = hwid
			}
			if _, ok := raw["expiration_date"]; !ok {
				raw["expiration_date"] = ""
			}
			if _, ok := raw["expiration_days"]; !ok {
				raw["expiration_days"] = 0
			}
			if _, ok := raw["count_connections"]; !ok {
				raw["count_connections"] = 0
			}
			if _, ok := raw["limit_connections"]; !ok {
				if value, exists := raw["limit"]; exists && value != nil {
					raw["limit_connections"] = value
				} else {
					raw["limit_connections"] = 0
				}
			}
			if _, ok := raw["message_html"]; !ok {
				if html := panelHTML(raw, user); html != "" {
					raw["message_html"] = html
				}
			}
			if updated, marshalErr := json.Marshal(raw); marshalErr == nil {
				body = updated
				resp.Header.Del("Content-Length")
			}
		}
	}
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
	return true
}

func panelHTML(raw map[string]interface{}, fallbackUser string) string {
	username := valueString(raw, "username", fallbackUser)
	expiration := valueString(raw, "expiration_date", "")
	days := valueString(raw, "expiration_days", "")
	connections := valueString(raw, "count_connections", "")
	limit := valueString(raw, "limit_connections", "")
	if expiration == "" && days == "" && connections == "" && limit == "" {
		return ""
	}
	return fmt.Sprintf("👤 Nome de usuario: %s<br>📆 Expira em: %s<br>📅 Dias restantes: %s<br>🚫 Conexoes: %s|%s", username, expiration, days, connections, limit)
}

func valueString(raw map[string]interface{}, key, fallback string) string {
	if value, ok := raw[key]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return fallback
}

func readLimit(path, user string) (int, bool, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") || fields[0] != user {
			continue
		}
		limit, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0, false, errors.New("limite inválido")
		}
		return limit, true, nil
	}
	return 0, false, nil
}
func health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "service": "checkuser"})
}
func info(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": "checkuser", "endpoint": "/check?user=...&uuid=...&hwid=...", "health": "/health"})
}
func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started))
	})
}
func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func envInt(key string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return fallback
}
