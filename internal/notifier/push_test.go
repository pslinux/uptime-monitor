package notifier

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReportSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("status") != "up" {
			t.Fatalf("期望 status=up, 得到 %q", r.URL.Query().Get("status"))
		}
		if r.URL.Query().Get("ping") != "123" {
			t.Fatalf("期望 ping=123")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p, err := New(srv.URL, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Report("up", "ok", 123); err != nil {
		t.Fatalf("期望上报成功, 得到: %v", err)
	}
}

func TestReportNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, err := New(srv.URL, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	err = p.Report("down", "db error", 10)
	if err == nil {
		t.Fatal("期望返回错误")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("期望包含 500, 得到: %v", err)
	}
}
