package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRuntimePort(t *testing.T) {
	t.Setenv("ADDR", "")
	t.Setenv("PORT", "")
	if address, err := listenAddress(); err != nil || address != "0.0.0.0:8080" {
		t.Fatalf("default %q %v", address, err)
	}
	t.Setenv("PORT", "9090")
	if address, err := listenAddress(); err != nil || address != "0.0.0.0:9090" {
		t.Fatal(address, err)
	}
	for _, port := range []string{"0", "65536", "bad"} {
		t.Setenv("PORT", port)
		if _, err := listenAddress(); err == nil {
			t.Fatal("invalid port accepted")
		}
	}
	t.Setenv("ADDR", "127.0.0.1:8081")
	if address, err := listenAddress(); err != nil || address != "127.0.0.1:8081" {
		t.Fatal(address, err)
	}
}
func TestRuntimeHealthAndProxyCookie(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("ALLOWED_ORIGINS", "https://game.example.trap.show")
	l := newLobby()
	handler := l.handler()
	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest("GET", "http://backend/healthz", nil))
	var body map[string]string
	if err := json.Unmarshal(health.Body.Bytes(), &body); err != nil || body["status"] != "ok" || health.Code != 200 {
		t.Fatal("unhealthy", health.Body.String())
	}
	if len(l.sessions) != 0 {
		t.Fatal("health check created a session")
	}
	req := httptest.NewRequest("POST", "http://backend/api/session", nil)
	req.Header.Set("Origin", "https://game.example.trap.show")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	cookies := rec.Result().Cookies()
	if rec.Code != 200 || len(cookies) != 1 {
		t.Fatal("session failed")
	}
	cookie := cookies[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Domain != "" || cookie.Path != "/" {
		t.Fatal("bad proxy cookie")
	}
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 403 || l.originOK(req) {
		t.Fatal("foreign origin accepted")
	}
	t.Setenv("COOKIE_SECURE", "")
	req.Header.Set("Origin", "")
	req.Header.Set("X-Forwarded-Proto", "https")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Result().Cookies()[0].Secure {
		t.Fatal("untrusted forwarded header accepted")
	}
}
