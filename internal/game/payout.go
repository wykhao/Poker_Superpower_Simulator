package game

import "sort"

// AwardPots settles a completed hand exactly once. Odd chips go clockwise from
// the button independently for each pot. The returned values are gross payouts.
func (h *HandState) AwardPots() map[int]int {
	payouts := make(map[int]int)
	pots := h.GetPots()
	for potIndex, seats := range h.GetWinners() {
		if len(seats) == 0 {
			continue
		}
		amount := pots[potIndex].Amount
		share, remainder := amount/len(seats), amount%len(seats)
		n := len(h.Players)
		sort.Slice(seats, func(i, j int) bool {
			return (seats[i]-h.Button-1+n)%n < (seats[j]-h.Button-1+n)%n
		})
		for index, seat := range seats {
			paid := share
			if index < remainder {
				paid++
			}
			h.Players[seat].Chips += paid
			payouts[seat] += paid
		}
	}
	return payouts
}
