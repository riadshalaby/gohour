package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseServeMonthBounds_NoFlagsUsesCurrentMonth(t *testing.T) {
	t.Parallel()

	bounds, err := parseServeMonthBounds("", "")
	if err != nil {
		t.Fatalf("parse bounds: %v", err)
	}
	want := time.Now().In(time.Local).Format("2006-01")
	if bounds.defaultMonth != want {
		t.Fatalf("expected default month %q, got %q", want, bounds.defaultMonth)
	}
}

func TestParseServeMonthBounds_ClampsWhenNowOutsideRange(t *testing.T) {
	t.Parallel()

	now := time.Now().In(time.Local)
	fromFuture := now.AddDate(0, 2, 0).Format("2006-01")
	toPast := now.AddDate(0, -2, 0).Format("2006-01")

	futureBounds, err := parseServeMonthBounds(fromFuture, "")
	if err != nil {
		t.Fatalf("parse future bounds: %v", err)
	}
	if futureBounds.defaultMonth != fromFuture {
		t.Fatalf("expected future clamp %q, got %q", fromFuture, futureBounds.defaultMonth)
	}

	pastBounds, err := parseServeMonthBounds("", toPast)
	if err != nil {
		t.Fatalf("parse past bounds: %v", err)
	}
	if pastBounds.defaultMonth != toPast {
		t.Fatalf("expected past clamp %q, got %q", toPast, pastBounds.defaultMonth)
	}
}

func TestWithServeMonthRedirect(t *testing.T) {
	t.Parallel()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	handler := withServeMonthRedirect(next, serveMonthBounds{defaultMonth: "2026-03"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("expected redirect status, got %d", res.Code)
	}
	if got := res.Header().Get("Location"); got != "/month/2026-03" {
		t.Fatalf("unexpected redirect target: %q", got)
	}
	if nextCalled {
		t.Fatalf("expected wrapper to intercept root redirect")
	}
}
