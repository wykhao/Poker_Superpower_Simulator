package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"poker-human-simulator/internal/app"
)

func TestLiveAPIRedactionAndCSRF(t *testing.T) {
	a, err := app.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(Handler(a, nil))
	defer ts.Close()
	post := func(path, body, origin, host string, header bool) *http.Response {
		t.Helper()
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if header {
			req.Header.Set("X-Poker-Request", "1")
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if host != "" {
			req.Host = host
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	for _, tc := range []struct {
		origin, host string
		header       bool
	}{{"https://attacker.example", "", true}, {"", "attacker.example", true}, {"", "", false}} {
		r := post("/api/start", `{"mode":"A"}`, tc.origin, tc.host, tc.header)
		r.Body.Close()
		if r.StatusCode != 403 {
			t.Fatal("cross-origin mutation accepted", r.StatusCode)
		}
	}
	for i := 0; i < 20; i++ {
		r := post("/api/start", `{"mode":"B"}`, "", "", true)
		data, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatal(string(data))
		}
		var s app.State
		if err = json.Unmarshal(data, &s); err != nil {
			t.Fatal(err)
		}
		if s.Current.Done {
			continue
		}
		if strings.Contains(string(data), `"seed"`) {
			t.Fatal("live API exposed seed")
		}
		for seat := 1; seat < 6; seat++ {
			if len(s.Current.Seats[seat].Hole) != 0 {
				t.Fatal("B can inspect holes")
			}
		}
		if len(s.Current.FutureBoard) != 5 {
			t.Fatal("B lacks future board")
		}
		r = post("/api/start", `{"mode":"A"}`, "", "", true)
		r.Body.Close()
		if r.StatusCode != 409 {
			t.Fatal("mode switch allowed")
		}
		response, e := http.Get(ts.URL + "/api/state?mode=A")
		if e != nil {
			t.Fatal(e)
		}
		var spoof app.State
		json.NewDecoder(response.Body).Decode(&spoof)
		response.Body.Close()
		if spoof.Current.Mode != "B" || len(spoof.Current.Seats[1].Hole) != 0 {
			t.Fatal("stats query upgraded ability")
		}
		response, e = http.Get(ts.URL + "/api/hand/" + jsonNumber(s.Current.ID))
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		if response.StatusCode != 404 {
			t.Fatal("unfinished hand exposed")
		}
		return
	}
	t.Fatal("no live hand fixture")
}
func jsonNumber(n int) string { b, _ := json.Marshal(n); return string(b) }
func TestEmbeddedUIAndExport(t *testing.T) {
	a, err := app.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(Handler(a, nil))
	defer ts.Close()
	for _, path := range []string{"/", "/app.js", "/style.css", "/icon.svg", "/api/export?mode=A", "/api/export?format=json"} {
		r, e := http.Get(ts.URL + path)
		if e != nil {
			t.Fatal(e)
		}
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode != 200 || len(body) == 0 {
			t.Fatal(path, r.StatusCode)
		}
		if r.Header.Get("Content-Security-Policy") == "" {
			t.Fatal("missing security headers")
		}
	}
}
