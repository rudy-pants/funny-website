package main

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testApplication(t *testing.T) *application {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate test source")
	}
	pagePath := filepath.Join(filepath.Dir(sourceFile), "..", "..", "templates", "index.html")
	page := template.Must(template.New("index.html").Funcs(template.FuncMap{
		"add": func(left, right int) int { return left + right },
	}).ParseFiles(pagePath))
	return newApplication(page)
}

func TestAPIJokeReturnsSelectedRegion(t *testing.T) {
	app := testApplication(t)
	req := httptest.NewRequest(http.MethodGet, "/api/joke?region=dunes&pick=1", nil)
	res := httptest.NewRecorder()

	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got := res.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content type = %q, want JSON", got)
	}
	if got := res.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("missing security header: X-Content-Type-Options = %q", got)
	}

	var payload struct {
		Region struct {
			ID string `json:"id"`
		} `json:"region"`
		Joke joke `json:"joke"`
		Pick int  `json:"pick"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Region.ID != "dunes" || payload.Pick != 1 {
		t.Fatalf("payload selection = region %q, pick %d; want dunes, 1", payload.Region.ID, payload.Pick)
	}
	if payload.Joke.Punchline == "" {
		t.Fatal("expected a non-empty punchline")
	}
}

func TestHandlersRejectInvalidInputs(t *testing.T) {
	app := testApplication(t)
	tests := []string{
		"/jokes",
		"/jokes?region=not-a-region",
		"/unlock?region=../dunes",
		"/api/joke?region=dunes&pick=99",
	}
	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, target, nil)
			res := httptest.NewRecorder()
			app.routes().ServeHTTP(res, req)

			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
			}
			body, _ := io.ReadAll(res.Body)
			if strings.Contains(string(body), "Dad Joke Dunes") {
				t.Fatal("invalid request leaked page content")
			}
		})
	}
}

func TestHTMXUnlockReturnsOnlyDiscoveryFragment(t *testing.T) {
	app := testApplication(t)
	req := httptest.NewRequest(http.MethodGet, "/unlock?region=prairie&pick=0", nil)
	req.Header.Set("HX-Request", "true")
	res := httptest.NewRecorder()

	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	body := res.Body.String()
	if !strings.Contains(body, `data-discovery="prairie"`) {
		t.Fatalf("fragment did not contain the unlocked region: %s", body)
	}
	if !strings.Contains(body, "data-unlocks=") || strings.Contains(body, "<!doctype html>") {
		t.Fatalf("response was not an HTMX discovery fragment")
	}
}

func TestJokeRouteProgressivelyRendersFullPage(t *testing.T) {
	app := testApplication(t)
	req := httptest.NewRequest(http.MethodGet, "/jokes?region=borough&pick=2", nil)
	res := httptest.NewRecorder()

	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	body := res.Body.String()
	if !strings.Contains(body, "<!doctype html>") || !strings.Contains(body, "So it was pointless.") {
		t.Fatal("plain link did not return the full page with the selected joke")
	}
}

func TestPostIsNotAllowed(t *testing.T) {
	app := testApplication(t)
	req := httptest.NewRequest(http.MethodPost, "/api/joke?region=dunes", nil)
	res := httptest.NewRecorder()

	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusMethodNotAllowed)
	}
	if got := res.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q, want GET, HEAD", got)
	}
}

func TestUnknownPathIsNotServedByIndex(t *testing.T) {
	app := testApplication(t)
	req := httptest.NewRequest(http.MethodGet, "/not-a-route", nil)
	res := httptest.NewRecorder()

	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}
