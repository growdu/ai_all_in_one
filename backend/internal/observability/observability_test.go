package observability

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthHandler_OK(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rw := httptest.NewRecorder()
	HealthHandler(func() bool { return true })(rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rw.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
	if body["db_ok"] != true {
		t.Errorf("db_ok = %v, want true", body["db_ok"])
	}
}

func TestHealthHandler_Degraded(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rw := httptest.NewRecorder()
	HealthHandler(func() bool { return false })(rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (always 200, status field shows degraded)", rw.Code)
	}
	var body map[string]any
	json.Unmarshal(rw.Body.Bytes(), &body)
	if body["status"] != "degraded" {
		t.Errorf("status = %v, want degraded", body["status"])
	}
}

func TestHealthHandler_NoDB(t *testing.T) {
	// dbOK 为 nil 时默认 true
	req := httptest.NewRequest("GET", "/health", nil)
	rw := httptest.NewRecorder()
	HealthHandler(nil)(rw, req)

	var body map[string]any
	json.Unmarshal(rw.Body.Bytes(), &body)
	if body["db_ok"] != true {
		t.Error("db_ok should be true when dbOK is nil")
	}
}

func TestMetricsHandler_Format(t *testing.T) {
	req := httptest.NewRequest("GET", "/metrics", nil)
	rw := httptest.NewRecorder()
	RecordChat("doubao", "doubao-1-5-pro", true, 10, 20)
	RecordChat("doubao", "doubao-1-5-pro", false, 5, 0)
	RecordRateLimit("user")
	MetricsHandler()(rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("status = %d", rw.Code)
	}
	ct := rw.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content-type = %q, want text/plain", ct)
	}
	body := rw.Body.String()
	for _, expect := range []string{
		"# HELP aiio_version",
		"aiio_chat_total",
		"aiio_chat_failed_total",
		"aiio_rate_limit_hits_total",
	} {
		if !strings.Contains(body, expect) {
			t.Errorf("metrics body missing %q", expect)
		}
	}
}

func TestUptime(t *testing.T) {
	u := UptimeSec()
	if u < 0 {
		t.Errorf("uptime = %d, want >= 0", u)
	}
}
