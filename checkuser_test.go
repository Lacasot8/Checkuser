package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckLocalSupportsUsernameAndTemplateFields(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checkuser-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	limits := filepath.Join(tempDir, "usuarios.db")
	shadow := filepath.Join(tempDir, "shadow")
	if err := ioutil.WriteFile(limits, []byte("cliente01 3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(shadow, []byte("outro:x:1:1:1:1::20000:\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{LimitsFile: limits, ShadowFile: shadow}
	req := httptest.NewRequest(http.MethodGet, "/check?username=cliente01", nil)
	rec := httptest.NewRecorder()
	cfg.check(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]interface{}{
		"username": "cliente01", "user": "cliente01", "expiration_date": "",
		"expiration_days": float64(0), "count_connections": float64(0), "limit_connections": float64(3),
		"message_html": "👤 Nome de usuario: cliente01<br>📆 Expira em: <br>📅 Dias restantes: 0<br>🚫 Conexoes: 0|3",
	} {
		if got[key] != want {
			t.Errorf("%s = %#v, want %#v", key, got[key], want)
		}
	}
}

func TestReadAccountExpiration(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checkuser-shadow-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	shadow := filepath.Join(tempDir, "shadow")
	if err := ioutil.WriteFile(shadow, []byte("cliente01:!:1:1:99999:7::20000:\n"), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(1970, time.January, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, 19995)
	wantDate := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 20000).Format("02/01/2006")
	gotDate, gotDays := readAccountExpiration(shadow, "cliente01", now)
	if gotDate != wantDate || gotDays != 5 {
		t.Fatalf("readAccountExpiration() = (%q, %d), want (%q, 5)", gotDate, gotDays, wantDate)
	}
}

func TestCountSSHProcesses(t *testing.T) {
	got := countSSHProcesses("sshd\nbash\ndropbear\nsystemd\n")
	if got != 2 {
		t.Fatalf("countSSHProcesses() = %d, want 2", got)
	}
}

func TestCheckProxiesDragonCoreFieldsAndAddsTemplate(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("user") != "cliente01" {
			t.Errorf("upstream user = %q", r.URL.Query().Get("user"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"cliente01","expiration_date":"2027-01-31","expiration_days":70,"count_connections":1,"limit_connections":2}`))
	}))
	defer upstream.Close()
	cfg := Config{DragonCoreURL: upstream.URL, DragonCorePath: "/check"}
	req := httptest.NewRequest(http.MethodGet, "/check?username=cliente01&uuid=id1&hwid=id2", nil)
	rec := httptest.NewRecorder()
	cfg.check(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["username"] != "cliente01" || got["expiration_date"] != "2027-01-31" || got["count_connections"] != float64(1) || got["limit_connections"] != float64(2) {
		t.Errorf("DragonCore fields were not preserved: %#v", got)
	}
	if got["message_html"] != "👤 Nome de usuario: cliente01<br>📆 Expira em: 2027-01-31<br>📅 Dias restantes: 70<br>🚫 Conexoes: 1|2" {
		t.Errorf("unexpected message_html: %#v", got["message_html"])
	}
}
