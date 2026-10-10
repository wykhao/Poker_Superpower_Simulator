package table

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func fixture(seed uint64, mode string) Pending {
	return Pending{ID: 1, Mode: mode, Seed: seed, Button: int(seed % Players), Session: "test", StartedAt: "2026-10-10T00:00:00Z"}
}
func TestInformationBoundariesAndSameDeal(t *testing.T) {
	for seed := uint64(1); seed <= 30; seed++ {
		a, err := Restore(fixture(seed, "A"))
		if err != nil {
			t.Fatal(err)
		}
		b, err := Restore(fixture(seed, "B"))
		if err != nil {
			t.Fatal(err)
		}
		av, bv := a.View(), b.View()
		if av.Done {
			continue
		}
		if len(av.FutureBoard) != 0 || av.FutureHand != "" {
			t.Fatal("A received future cards")
		}
		if len(bv.FutureBoard) != 5 {
			t.Fatal("B did not receive full board")
		}
		for seat := 0; seat < Players; seat++ {
			if len(av.Seats[seat].Hole) != 2 {
				t.Fatal("A lost hole visibility")
			}
			if seat > 0 && len(bv.Seats[seat].Hole) != 0 {
				t.Fatal("B received opponent holes")
			}
		}
		if !reflect.DeepEqual(av.Seats[0].Hole, bv.Seats[0].Hole) || !reflect.DeepEqual(av.Events, bv.Events) {
			t.Fatal("ability changed deal or bot actions")
		}
		encoded, _ := json.Marshal(bv)
		if strings.Contains(string(encoded), `"seed"`) {
			t.Fatal("live view leaked seed")
		}
		if av.Legal == nil || bv.Legal == nil {
			t.Fatal("game did not pause for human")
		}
	}
}
func TestHumanActionsResumeAndAccounting(t *testing.T) {
	modes := []string{"A", "B"}
	for seed := uint64(1); seed <= 350; seed++ {
		p := fixture(seed, modes[seed%2])
		g, err := Restore(p)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		for steps := 0; !g.Done(); steps++ {
			if steps > 50 {
				t.Fatalf("human action loop seed %d", seed)
			}
			v := g.View()
			choice := Action{Kind: "call"}
			// Test drivers supply explicit human commands; they are not app policies.
			switch seed % 5 {
			case 0:
				choice.Kind = "fold"
			case 1:
				choice.Kind = "allin"
			case 2:
				if v.Legal.CanRaise {
					choice = Action{Kind: "raise", Amount: v.Legal.MinRaiseTo}
				}
			}
			if err := g.Act(choice); err != nil {
				t.Fatalf("seed %d step %d action %+v: %v", seed, steps, choice, err)
			}
			restored, err := Restore(g.Pending())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(g.View(), restored.View()) {
				t.Fatalf("resume changed seed %d step %d", seed, steps)
			}
		}
		r := g.Record("now")
		sum := 0
		for _, n := range r.Net {
			sum += n
		}
		if sum != 0 || r.Net[0] < -Stack || r.Net[0] > (Players-1)*Stack {
			t.Fatal("invalid payout", r.Net)
		}
		if err := g.Act(Action{Kind: "allin"}); err == nil {
			t.Fatal("completed hand accepted action")
		}
	}
}
func TestInvalidHumanActionDoesNotReachEngine(t *testing.T) {
	g, err := Restore(fixture(12, "A"))
	if err != nil {
		t.Fatal(err)
	}
	if g.Done() {
		t.Fatal("fixture ended")
	}
	before := g.View()
	for _, bad := range []Action{{Kind: "dance"}, {Kind: "raise", Amount: -1}, {Kind: "raise", Amount: 2001}, {Kind: "fold", Amount: 10}} {
		if err := g.Act(bad); err == nil {
			t.Fatal("accepted", bad)
		}
		if !reflect.DeepEqual(before, g.View()) {
			t.Fatal("invalid action mutated table", bad)
		}
	}
}
