package web

import (
	"embed"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"

	"poker-human-simulator/internal/app"
	"poker-human-simulator/internal/table"
)

//go:embed assets/*
var assets embed.FS

func Handler(a *app.App, quit func()) http.Handler {
	mux := http.NewServeMux()
	send := func(w http.ResponseWriter, value any) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(value)
	}
	fail := func(w http.ResponseWriter, err error) {
		code := http.StatusBadRequest
		var conflict app.Conflict
		if errors.As(err, &conflict) {
			code = http.StatusConflict
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	}
	read := func(w http.ResponseWriter, r *http.Request, dst any) error {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		defer r.Body.Close()
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(dst); err != nil {
			return err
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			return fmt.Errorf("only one JSON object is allowed")
		}
		return nil
	}
	state := func(r *http.Request) app.State {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		return a.StatePage(r.URL.Query().Get("mode"), r.URL.Query().Get("scope"), offset)
	}
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) { send(w, state(r)) })
	mux.HandleFunc("POST /api/start", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Mode string `json:"mode"`
		}
		if err := read(w, r, &body); err != nil {
			fail(w, err)
			return
		}
		if err := a.Start(body.Mode); err != nil {
			fail(w, err)
			return
		}
		send(w, state(r))
	})
	mux.HandleFunc("POST /api/action", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID       int    `json:"hand_id"`
			Revision int    `json:"revision"`
			Action   string `json:"action"`
			Amount   int    `json:"amount"`
		}
		if err := read(w, r, &body); err != nil {
			fail(w, err)
			return
		}
		if err := a.Act(body.ID, body.Revision, table.Action{Kind: body.Action, Amount: body.Amount}); err != nil {
			fail(w, err)
			return
		}
		send(w, state(r))
	})
	mux.HandleFunc("POST /api/session", func(w http.ResponseWriter, r *http.Request) {
		var body struct{}
		if err := read(w, r, &body); err != nil {
			fail(w, err)
			return
		}
		if err := a.NewSession(); err != nil {
			fail(w, err)
			return
		}
		send(w, state(r))
	})
	mux.HandleFunc("POST /api/quit", func(w http.ResponseWriter, r *http.Request) {
		var body struct{}
		if err := read(w, r, &body); err != nil {
			fail(w, err)
			return
		}
		send(w, map[string]bool{"saved": true})
		if quit != nil {
			go quit()
		}
	})
	mux.HandleFunc("GET /api/hand/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			fail(w, err)
			return
		}
		h, ok := a.Hand(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		send(w, h)
	})
	mux.HandleFunc("GET /api/export", func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("mode")
		if mode == "" {
			mode = "all"
		}
		if mode != "all" && !table.ValidMode(mode) {
			fail(w, fmt.Errorf("invalid mode"))
			return
		}
		records := a.Records(mode, r.URL.Query().Get("scope"))
		if r.URL.Query().Get("format") == "json" {
			w.Header().Set("Content-Disposition", `attachment; filename="poker-completed-hands.json"`)
			send(w, map[string]any{"version": 1, "rules": table.RulesVersion, "hands": records})
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="poker-%s-history.csv"`, mode))
		io.WriteString(w, "\ufeff")
		cw := csv.NewWriter(w)
		defer cw.Flush()
		cw.Write([]string{"hand_id", "ability", "completed_at_utc", "net_bb", "vpip", "pfr", "hero_showdown", "button", "hole", "dealt_board", "scheduled_board", "session", "rules"})
		for _, h := range records {
			cw.Write([]string{strconv.Itoa(h.ID), h.Mode, h.CompletedAt, strconv.FormatFloat(float64(h.Net[0])/table.BB, 'f', 1, 64), strconv.FormatBool(h.VPIP), strconv.FormatBool(h.PFR), strconv.FormatBool(h.Showdown), strconv.Itoa(h.Button), strings.Join(h.View.Seats[0].Hole, " "), strings.Join(h.View.Board, " "), strings.Join(h.View.FutureBoard, " "), h.Session, h.Rules})
		}
	})
	static, _ := fs.Sub(assets, "assets")
	mux.Handle("/", http.FileServer(http.FS(static)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bind locally AND reject arbitrary Host names to prevent DNS rebinding.
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" && host != "::1" {
			http.Error(w, "local access only", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Poker-Simulator", "human-local-v1")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			origin := r.Header.Get("Origin")
			if r.Header.Get("X-Poker-Request") != "1" || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") || (origin != "" && origin != "http://"+r.Host) {
				http.Error(w, "same-origin JSON request required", http.StatusForbidden)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
