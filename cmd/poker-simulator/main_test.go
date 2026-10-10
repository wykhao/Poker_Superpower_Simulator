package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestExistingInstanceAddressIsLocalAndVerified(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("X-Poker-Simulator", "human-local-v1")
		w.Write([]byte(`{}`))
	}))
	defer server.Close()
	if err := os.WriteFile(filepath.Join(dir, "local-url"), []byte(server.URL), 0600); err != nil {
		t.Fatal(err)
	}
	if got := existingAddress(dir); got != server.URL {
		t.Fatal(got)
	}
	for _, bad := range []string{"https://example.com", "http://127.0.0.1:123/path", "http://user@127.0.0.1:123", "http://127.0.0.1:123?x=1"} {
		os.WriteFile(filepath.Join(dir, "local-url"), []byte(bad), 0600)
		if existingAddress(dir) != "" {
			t.Fatal("accepted unsafe address")
		}
	}
	if calls != 1 {
		t.Fatal("unexpected requests", calls)
	}
}
