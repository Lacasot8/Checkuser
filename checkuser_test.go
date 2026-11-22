package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckLocalSupportsUsernameAndTemplateFields(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "checkuser-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	limits := filepath.Join(tempDir, "usuarios.db")
	if err := ioutil.WriteFile(limits, []byte("cliente01 3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{LimitsFile: limits}
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
	} {
		if got[key] != want {
			t.Errorf("%s = %#v, want %#v", key, got[key], want)
		}
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
