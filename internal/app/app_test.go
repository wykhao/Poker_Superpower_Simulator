package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"poker-human-simulator/internal/table"
)

func finish(t *testing.T, a *App) {
	t.Helper()
	for i := 0; i < 60; i++ {
		v := a.State("A", "all").Current
		if v == nil || v.Done {
			return
		}
		if err := a.Act(v.ID, v.Revision, table.Action{Kind: "call"}); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("long test hand")
}
func TestSeparatedStatsPersistenceAndSessions(t *testing.T) {
	dir := t.TempDir()
	a, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"A", "B", "A", "B"} {
		if err = a.Start(mode); err != nil {
			t.Fatal(err)
		}
		finish(t, a)
	}
	before := a.State("A", "all")
	if before.Stats["A"].Hands != 2 || before.Stats["B"].Hands != 2 {
		t.Fatal("mixed ability accounting", before.Stats)
	}
	for _, s := range before.Stats {
		if s.BB100 != s.NetBB/float64(s.Hands)*100 {
			t.Fatal("BB/100 denominator")
		}
	}
	restored, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, restored.State("A", "all")) {
		t.Fatal("restart changed results")
	}
	if err = restored.NewSession(); err != nil {
		t.Fatal(err)
	}
	if restored.State("A", "session").Stats["A"].Hands != 0 || restored.State("A", "all").Stats["A"].Hands != 2 {
		t.Fatal("new session erased history")
	}
}
func TestPendingModeLockedAndRestartResumes(t *testing.T) {
	dir := t.TempDir()
	a, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if err = a.Start("B"); err != nil {
			t.Fatal(err)
		}
		if !a.current.Done() {
			break
		}
	}
	before := a.State("B", "all")
	if before.Current.Done {
		t.Fatal("no pending fixture")
	}
	if err = a.Start("A"); err == nil {
		t.Fatal("ability changed mid-hand")
	}
	if err = a.NewSession(); err == nil {
		t.Fatal("abandoned unfinished hand")
	}
	restored, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, restored.State("B", "all")) {
		t.Fatal("pending hand lost")
	}
	if _, ok := restored.Hand(before.Current.ID); ok {
		t.Fatal("unfinished hand exposed via history")
	}
	if got := len(restored.Records("all", "all")); got != len(a.db.Hands) {
		t.Fatal("export included pending hand")
	}
}
func TestStaleConcurrentActionCommitsOnlyOnce(t *testing.T) {
	a, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for {
		if err = a.Start("A"); err != nil {
			t.Fatal(err)
		}
		if !a.current.Done() {
			break
		}
	}
	v := a.State("A", "all").Current
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- a.Act(v.ID, v.Revision, table.Action{Kind: "call"}) }()
	}
	wg.Wait()
	close(errs)
	success, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else {
			var conflict Conflict
			if errors.As(err, &conflict) {
				conflicts++
			} else {
				t.Fatal(err)
			}
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatal(success, conflicts)
	}
}
func TestFailedSaveRollsBack(t *testing.T) {
	a, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for {
		if err = a.Start("A"); err != nil {
			t.Fatal(err)
		}
		if !a.current.Done() {
			break
		}
	}
	before := a.State("A", "all")
	a.save = func(Database) error { return errors.New("disk full") }
	if err = a.Act(before.Current.ID, before.Current.Revision, table.Action{Kind: "allin"}); err == nil {
		t.Fatal("ignored disk failure")
	}
	if !reflect.DeepEqual(before, a.State("A", "all")) {
		t.Fatal("unsaved action mutated history")
	}
}
func TestCorruptHistoryPreserved(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "history.json")
	data := []byte(`{"version": broken`)
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("corruption ignored")
	}
	after, _ := os.ReadFile(p)
	if string(after) != string(data) {
		t.Fatal("overwrote corrupt user data")
	}
}
func TestStatsWindowAndNeutralOutcomes(t *testing.T) {
	records := []table.Record{}
	for i := 0; i < 120; i++ {
		records = append(records, table.Record{Pending: table.Pending{ID: i + 1, Mode: "A", Session: "x"}, Net: [table.Players]int{10, -10}})
	}
	s := stats("A", filter(records, "A", "20", "x"))
	if s.Hands != 20 || s.NetBB != 20 || s.BB100 != 100 {
		t.Fatal(s)
	}
	if stats("B", nil).BB100 != 0 {
		t.Fatal("empty sample divided by zero")
	}
}

func TestHistoryPaginationKeepsStatsWhole(t *testing.T) {
	a, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 75; i++ {
		a.db.Hands = append(a.db.Hands, table.Record{Pending: table.Pending{ID: i + 1, Mode: "A", Session: a.db.Session}, Net: [table.Players]int{10, -10}, View: table.View{Seats: []table.SeatView{{Hole: []string{"Ac", "Kd"}}}}})
	}
	s := a.StatePage("A", "all", 30)
	if s.TotalHistory != 75 || s.HistoryOffset != 30 || len(s.History) != 30 || s.History[0].ID != 45 || s.Stats["A"].NetBB != 75 {
		t.Fatal("pagination changed accounting", s)
	}
	s = a.StatePage("A", "all", 9999)
	if s.HistoryOffset != 60 || len(s.History) != 15 || s.History[14].ID != 1 {
		t.Fatal("last page clamp", s)
	}
}
