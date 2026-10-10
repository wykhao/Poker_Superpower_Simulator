package poker

import (
	"math/rand/v2"
	"testing"
)

func TestEvaluateCardsBestFive(t *testing.T) {
	rng := rand.New(rand.NewPCG(413, 88))
	for n := 5; n <= 7; n++ {
		for trial := 0; trial < 1000; trial++ {
			var h Hand
			for h.CountCards() < n {
				h |= Hand(1) << rng.IntN(52)
			}
			var best HandRank
			for a := 0; a < n-4; a++ {
				for b := a + 1; b < n-3; b++ {
					for c := b + 1; c < n-2; c++ {
						for d := c + 1; d < n-1; d++ {
							for e := d + 1; e < n; e++ {
								rank := EvaluateCards(NewHand(h.GetCard(a), h.GetCard(b), h.GetCard(c), h.GetCard(d), h.GetCard(e)))
								best = max(best, rank)
							}
						}
					}
				}
			}
			if got := EvaluateCards(h); got != best {
				t.Fatalf("n=%d hand=%v got=%x best=%x", n, h, got, best)
			}
		}
	}
	if EvaluateCards(parseCards("Ac", "Ad", "Ah", "As")) != 0 {
		t.Fatal("invalid size accepted")
	}
}

func TestStraightChoosesSixHighOverWheel(t *testing.T) {
	for _, cs := range [][]string{{"Ac", "2d", "3h", "4s", "5c", "6d"}, {"Ac", "2c", "3c", "4c", "5c", "6c", "Kh"}} {
		h := parseCards(cs...)
		got := EvaluateCards(h)
		want := Straight | HandRank(4)<<24
		if len(cs) == 7 {
			want = StraightFlush | HandRank(4)<<24
		}
		if got != want {
			t.Fatalf("must pick 6 high: got %x want %x", got, want)
		}
	}
}
