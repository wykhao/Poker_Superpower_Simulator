// Package app owns a single local human table and durable A/B history.
package app

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"poker-human-simulator/internal/table"
)

var Version = "0.1.0"

type Database struct {
	Version       int            `json:"version"`
	NextID        int            `json:"next_id"`
	Started       map[string]int `json:"started_by_mode"`
	Session       string         `json:"session"`
	PreferredMode string         `json:"preferred_mode"`
	Active        *table.Pending `json:"active,omitempty"`
	Hands         []table.Record `json:"hands"`
}
type App struct {
	mu      sync.Mutex
	path    string
	db      Database
	current *table.Table
	save    func(Database) error
}
type Conflict struct{ Message string }

func (e Conflict) Error() string { return e.Message }
func token() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func Open(dir string) (*App, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	a := &App{path: filepath.Join(dir, "history.json")}
	a.save = func(db Database) error { return atomicJSON(a.path, db) }
	data, err := os.ReadFile(a.path)
	if errors.Is(err, os.ErrNotExist) {
		session, e := token()
		if e != nil {
			return nil, e
		}
		a.db = Database{Version: 1, NextID: 1, Started: map[string]int{"A": 0, "B": 0}, Session: session, PreferredMode: "A", Hands: []table.Record{}}
		if e = a.save(a.db); e != nil {
			return nil, e
		}
	} else if err != nil {
		return nil, err
	} else {
		if err = json.Unmarshal(data, &a.db); err != nil {
			return nil, fmt.Errorf("cannot read saved history; original file preserved: %w", err)
		}
		if a.db.Version != 1 || a.db.NextID < 1 || a.db.Started == nil || !table.ValidMode(a.db.PreferredMode) {
			return nil, fmt.Errorf("unsupported or invalid saved history; file preserved")
		}
		seen := map[int]bool{}
		for _, h := range a.db.Hands {
			sum := 0
			for _, n := range h.Net {
				sum += n
			}
			if h.ID < 1 || seen[h.ID] || h.ID >= a.db.NextID || !table.ValidMode(h.Mode) || sum != 0 || len(h.View.Seats) != table.Players || !h.View.Done || h.View.ID != h.ID || h.View.Mode != h.Mode {
				return nil, fmt.Errorf("invalid saved hand accounting")
			}
			seen[h.ID] = true
		}
		if a.db.Active != nil && (seen[a.db.Active.ID] || a.db.Active.ID >= a.db.NextID) {
			return nil, fmt.Errorf("invalid pending hand ID")
		}
	}
	if a.db.Active != nil {
		a.current, err = table.Restore(*a.db.Active)
		if err != nil || a.current.Done() {
			return nil, fmt.Errorf("cannot restore pending hand: %v", err)
		}
	}
	return a, nil
}

func atomicJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".history-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}

func (a *App) commit(db Database, t *table.Table) error {
	if t.Done() {
		db.Hands = append(append([]table.Record(nil), db.Hands...), t.Record(now()))
		db.Active = nil
	} else {
		p := t.Pending()
		db.Active = &p
	}
	if err := a.save(db); err != nil {
		return fmt.Errorf("save failed; action was not committed: %w", err)
	}
	a.db, a.current = db, t
	return nil
}
func (a *App) Start(mode string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !table.ValidMode(mode) {
		return fmt.Errorf("choose ability A or B")
	}
	if a.db.Active != nil {
		return Conflict{"finish or fold the current hand before changing ability"}
	}
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return err
	}
	db := a.db
	db.Started = map[string]int{"A": db.Started["A"], "B": db.Started["B"]}
	p := table.Pending{ID: db.NextID, Mode: mode, Session: db.Session, Seed: binary.LittleEndian.Uint64(bytes[:]), Button: db.Started[mode] % table.Players, StartedAt: now()}
	db.NextID++
	db.Started[mode]++
	db.PreferredMode = mode
	t, err := table.Restore(p)
	if err != nil {
		return err
	}
	return a.commit(db, t)
}
func (a *App) Act(id, revision int, choice table.Action) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.db.Active
	if p == nil || p.ID != id || len(p.Actions) != revision {
		return Conflict{"this turn has changed; refresh before acting again"}
	}
	// Work on a reconstructed candidate: invalid actions or failed disk writes
	// cannot advance the real state or silently discard the pending hand.
	t, err := table.Restore(*p)
	if err != nil {
		return err
	}
	if err = t.Act(choice); err != nil {
		return err
	}
	return a.commit(a.db, t)
}
func (a *App) NewSession() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.db.Active != nil {
		return Conflict{"finish the current hand before starting a new session"}
	}
	session, err := token()
	if err != nil {
		return err
	}
	db := a.db
	db.Session = session
	if err = a.save(db); err != nil {
		return err
	}
	a.db = db
	return nil
}

type Stats struct {
	Mode       string  `json:"mode"`
	Hands      int     `json:"hands"`
	NetBB      float64 `json:"net_bb"`
	BB100      float64 `json:"bb100"`
	Profitable int     `json:"profitable"`
	ProfitRate float64 `json:"profit_rate"`
	VPIP       float64 `json:"vpip"`
	PFR        float64 `json:"pfr"`
	Showdowns  int     `json:"showdowns"`
	BestBB     float64 `json:"best_bb"`
	WorstBB    float64 `json:"worst_bb"`
	Curve      []Point `json:"curve"`
}
type Point struct {
	Hand  int     `json:"hand"`
	ID    int     `json:"id"`
	NetBB float64 `json:"net_bb"`
}
type HandRow struct {
	ID          int      `json:"id"`
	Mode        string   `json:"mode"`
	CompletedAt string   `json:"completed_at"`
	NetBB       float64  `json:"net_bb"`
	Showdown    bool     `json:"showdown"`
	Board       []string `json:"board"`
	Hole        []string `json:"hole"`
}
type State struct {
	Version       string           `json:"version"`
	Session       string           `json:"session"`
	PreferredMode string           `json:"preferred_mode"`
	Current       *table.View      `json:"current"`
	Stats         map[string]Stats `json:"stats"`
	History       []HandRow        `json:"history"`
	TotalHistory  int              `json:"total_history"`
	HistoryOffset int              `json:"history_offset"`
	Scope         string           `json:"scope"`
}

func filter(hands []table.Record, mode, scope, session string) []table.Record {
	out := make([]table.Record, 0)
	for _, h := range hands {
		if h.Mode == mode && (scope != "session" || h.Session == session) {
			out = append(out, h)
		}
	}
	limit := 0
	switch scope {
	case "20":
		limit = 20
	case "100":
		limit = 100
	case "500":
		limit = 500
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}
func stats(mode string, records []table.Record) Stats {
	s := Stats{Mode: mode, Hands: len(records), Curve: []Point{{}}}
	vpip, pfr := 0, 0
	sum := 0
	for i, h := range records {
		n := h.Net[0]
		sum += n
		if n > 0 {
			s.Profitable++
		}
		if h.VPIP {
			vpip++
		}
		if h.PFR {
			pfr++
		}
		if h.Showdown {
			s.Showdowns++
		}
		bb := float64(n) / table.BB
		s.BestBB = max(s.BestBB, bb)
		s.WorstBB = min(s.WorstBB, bb)
		// Retain a bounded chart while all accounting still uses every hand.
		step := max(1, len(records)/300)
		if (i+1)%step == 0 || i == len(records)-1 {
			s.Curve = append(s.Curve, Point{Hand: i + 1, ID: h.ID, NetBB: float64(sum) / table.BB})
		}
	}
	s.NetBB = float64(sum) / table.BB
	if s.Hands > 0 {
		n := float64(s.Hands)
		s.BB100 = s.NetBB / n * 100
		s.ProfitRate = float64(s.Profitable) / n * 100
		s.VPIP = float64(vpip) / n * 100
		s.PFR = float64(pfr) / n * 100
	}
	return s
}
func (a *App) State(mode, scope string) State {
	return a.StatePage(mode, scope, 0)
}
func (a *App) StatePage(mode, scope string, offset int) State {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !table.ValidMode(mode) {
		mode = a.db.PreferredMode
	}
	s := State{Version: Version, Session: a.db.Session, PreferredMode: a.db.PreferredMode, Stats: map[string]Stats{}, History: []HandRow{}, Scope: scope}
	if a.current != nil {
		v := a.current.View()
		s.Current = &v
	} else if len(a.db.Hands) > 0 {
		v := a.db.Hands[len(a.db.Hands)-1].View
		s.Current = &v
	}
	for _, m := range []string{"A", "B"} {
		s.Stats[m] = stats(m, filter(a.db.Hands, m, scope, a.db.Session))
	}
	records := filter(a.db.Hands, mode, scope, a.db.Session)
	s.TotalHistory = len(records)
	offset = max(0, offset)
	if len(records) > 0 {
		offset = min(offset, ((len(records)-1)/30)*30)
	} else {
		offset = 0
	}
	s.HistoryOffset = offset
	end := len(records) - offset
	for i := end - 1; i >= max(0, end-30); i-- {
		h := records[i]
		s.History = append(s.History, HandRow{ID: h.ID, Mode: h.Mode, CompletedAt: h.CompletedAt, NetBB: float64(h.Net[0]) / table.BB, Showdown: h.Showdown, Board: h.View.Board, Hole: h.View.Seats[0].Hole})
	}
	return s
}
func (a *App) Records(mode, scope string) []table.Record {
	a.mu.Lock()
	defer a.mu.Unlock()
	if mode == "all" {
		return append([]table.Record(nil), a.db.Hands...)
	}
	return filter(a.db.Hands, mode, scope, a.db.Session)
}
func (a *App) Hand(id int) (table.Record, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, h := range a.db.Hands {
		if h.ID == id {
			return h, true
		}
	}
	return table.Record{}, false
}
