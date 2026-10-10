package game

import (
	"poker-human-simulator/internal/randutil"
	"testing"
)

func TestBettingOnlyActionNeverDeals(t *testing.T) {
	h := NewHandState(randutil.New(71), []string{"a", "b", "c"}, 0, 5, 10, WithChips(1000))
	h.Deck = nil // Any accidental NextStreet would panic.
	for step := 0; step < 3; step++ {
		done, err := h.ProcessBettingAction(Call, 0)
		if err != nil {
			t.Fatal(err)
		}
		if done != (step == 2) {
			t.Fatal("unexpected betting completion", step, done)
		}
	}
	if h.Street != Preflop || h.Board != 0 {
		t.Fatal("betting-only method advanced board")
	}
	for _, p := range h.Players {
		if p.TotalBet != 10 || p.Chips != 990 {
			t.Fatal("wrong betting accounting", p)
		}
	}
}
