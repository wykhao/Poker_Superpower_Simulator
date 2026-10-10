// Package table adapts the PokerForBots engine for exactly one human seat.
// There is deliberately no A/B policy, equity search or automatic human action.
package table

import (
	"fmt"
	"slices"

	"poker-human-simulator/internal/game"
	"poker-human-simulator/internal/randutil"
	"poker-human-simulator/poker"
	"poker-human-simulator/protocol"
	"poker-human-simulator/sdk/bots/complex"
	"poker-human-simulator/sdk/client"
)

const Players = 6
const Stack = 1000
const BB = 10
const RulesVersion = "six-max-100bb-complex-v1"

type Action struct {
	Kind   string `json:"action"`
	Amount int    `json:"amount,omitempty"`
}
type Pending struct {
	ID        int      `json:"id"`
	Mode      string   `json:"mode"`
	Session   string   `json:"session"`
	Seed      uint64   `json:"seed,string"`
	Button    int      `json:"button"`
	StartedAt string   `json:"started_at"`
	Actions   []Action `json:"actions"`
}
type Event struct {
	Street  string   `json:"street"`
	Seat    int      `json:"seat"`
	Action  string   `json:"action"`
	Paid    int      `json:"paid"`
	RaiseTo int      `json:"raise_to,omitempty"`
	Board   []string `json:"board"`
}
type SeatView struct {
	Seat     int      `json:"seat"`
	Name     string   `json:"name"`
	Chips    int      `json:"chips"`
	Bet      int      `json:"bet"`
	Hole     []string `json:"hole,omitempty"`
	Folded   bool     `json:"folded"`
	AllIn    bool     `json:"all_in"`
	Active   bool     `json:"active"`
	Position string   `json:"position,omitempty"`
	Net      int      `json:"net"`
}
type Legal struct {
	Actions    []string `json:"actions"`
	ToCall     int      `json:"to_call"`
	MinRaiseTo int      `json:"min_raise_to"`
	MaxRaiseTo int      `json:"max_raise_to"`
	Bet        int      `json:"bet"`
	CanRaise   bool     `json:"can_raise"`
}
type View struct {
	ID          int        `json:"id"`
	Mode        string     `json:"mode"`
	Revision    int        `json:"revision"`
	Done        bool       `json:"done"`
	Street      string     `json:"street"`
	Button      int        `json:"button"`
	Pot         int        `json:"pot"`
	Board       []string   `json:"board"`
	FutureBoard []string   `json:"future_board,omitempty"`
	FutureHand  string     `json:"future_hand,omitempty"`
	CurrentHand string     `json:"current_hand,omitempty"`
	Seats       []SeatView `json:"seats"`
	Legal       *Legal     `json:"legal,omitempty"`
	Events      []Event    `json:"events"`
	Net         int        `json:"net"`
}
type Record struct {
	Pending
	CompletedAt string       `json:"completed_at"`
	Rules       string       `json:"rules"`
	Net         [Players]int `json:"net_chips"`
	VPIP        bool         `json:"vpip"`
	PFR         bool         `json:"pfr"`
	Showdown    bool         `json:"showdown"`
	View        View         `json:"view"`
}

type Table struct {
	pending             Pending
	h                   *game.HandState
	bots                [Players]*complex.Handler
	states              [Players]*client.GameState
	future              [5]poker.Card
	events              []Event
	done                bool
	vpip, pfr, showdown bool
	finalPot            int
}

func ValidMode(mode string) bool { return mode == "A" || mode == "B" }
func cards(h poker.Hand) []string {
	out := make([]string, h.CountCards())
	for i := range out {
		out[i] = h.GetCard(i).String()
	}
	return out
}
func board(cs []poker.Card) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.String()
	}
	return out
}
func (t *Table) pot() int {
	n := 0
	for _, p := range t.h.GetPots() {
		n += p.Amount
	}
	return n
}
func (t *Table) publicPlayers() []protocol.Player {
	out := make([]protocol.Player, Players)
	for i, p := range t.h.Players {
		out[i] = protocol.Player{Seat: i, Name: p.Name, Chips: p.Chips, Bet: p.Bet, Folded: p.Folded, AllIn: p.AllInFlag}
	}
	return out
}
func (t *Table) request() protocol.ActionRequest {
	h := t.h
	p := h.Players[h.ActivePlayer]
	req := protocol.ActionRequest{Type: protocol.TypeActionRequest, HandID: fmt.Sprint(t.pending.ID), Pot: t.pot(), ToCall: h.Betting.CurrentBet - p.Bet, MinBet: h.Betting.CurrentBet + h.Betting.MinRaise, MinRaise: h.Betting.MinRaise}
	for _, a := range h.GetValidActions() {
		req.ValidActions = append(req.ValidActions, a.String())
	}
	return req
}
func seedFor(seed uint64, seat int) uint64 {
	x := seed + uint64(seat+1)*0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// Restore deterministically replays only recorded human choices. Every other
// action belongs to one of the five unmodified independently seeded bots.
func Restore(p Pending) (*Table, error) {
	if !ValidMode(p.Mode) || p.ID < 1 || p.Button < 0 || p.Button >= Players {
		return nil, fmt.Errorf("invalid saved hand")
	}
	t := &Table{pending: p}
	t.pending.Actions = nil
	t.h = game.NewHandState(randutil.New(int64(p.Seed)), []string{"You", "Bot 1", "Bot 2", "Bot 3", "Bot 4", "Bot 5"}, p.Button, 5, BB, game.WithChips(Stack))
	deck := *t.h.Deck
	copy(t.future[:], deck.Deal(5))
	ps := t.publicPlayers()
	for seat := 1; seat < Players; seat++ {
		pl := t.h.Players[seat]
		t.bots[seat] = complex.NewSeededHandler(seedFor(p.Seed, seat))
		st := &client.GameState{HandID: fmt.Sprint(p.ID), Seat: seat, Chips: pl.Chips, StartingChips: Stack, Players: ps, HoleCards: cards(pl.HoleCards), Street: "preflop", Button: p.Button, ActiveCount: Players}
		t.states[seat] = st
		if err := t.bots[seat].OnHandStart(st, protocol.HandStart{Type: protocol.TypeHandStart, HandID: st.HandID, YourSeat: seat, Button: p.Button, Players: ps, HoleCards: st.HoleCards, SmallBlind: 5, BigBlind: BB}); err != nil {
			return nil, err
		}
	}
	if err := t.advance(); err != nil {
		return nil, err
	}
	for _, a := range p.Actions {
		if err := t.Act(a); err != nil {
			return nil, fmt.Errorf("saved action: %w", err)
		}
	}
	return t, nil
}

func (t *Table) syncBots() error {
	ps := t.publicPlayers()
	h := t.h
	for seat := 1; seat < Players; seat++ {
		st := t.states[seat]
		st.Players = ps
		st.Pot = t.pot()
		st.Chips = h.Players[seat].Chips
		st.Street = h.Street.String()
		st.Board = board(h.BoardCards())
		if err := t.bots[seat].OnGameUpdate(st, protocol.GameUpdate{Type: protocol.TypeGameUpdate, HandID: st.HandID, Players: ps, Pot: st.Pot}); err != nil {
			return err
		}
		if err := t.bots[seat].OnStreetChange(st, protocol.StreetChange{Type: protocol.TypeStreetChange, HandID: st.HandID, Street: st.Street, Board: st.Board}); err != nil {
			return err
		}
	}
	return nil
}

func (t *Table) apply(a Action, human bool) error {
	h := t.h
	p := h.Players[h.ActivePlayer]
	req := t.request()
	seat := h.ActivePlayer
	var kind game.Action
	amount := a.Amount
	switch a.Kind {
	case "fold":
		kind = game.Fold
	case "check":
		if req.ToCall != 0 {
			return fmt.Errorf("cannot check facing a bet")
		}
		kind = game.Check
	case "call":
		if req.ToCall == 0 {
			kind = game.Check
		} else if req.ToCall >= p.Chips {
			kind = game.AllIn
			amount = 0
		} else {
			kind = game.Call
		}
	case "raise":
		if !slices.Contains(req.ValidActions, "raise") || a.Amount < req.MinBet || a.Amount > p.Chips+p.Bet {
			return fmt.Errorf("raise must be between %d and %d chips", req.MinBet, p.Chips+p.Bet)
		}
		kind = game.Raise
		if a.Amount == p.Chips+p.Bet {
			kind = game.AllIn
			amount = 0
		}
	case "allin":
		// A call-sized all-in is legal whenever calling is legal. A raise-sized
		// all-in must also respect raise reopening; never bypass the action list.
		if p.Chips > req.ToCall && !slices.Contains(req.ValidActions, "allin") && !slices.Contains(req.ValidActions, "raise") {
			return fmt.Errorf("raising is not allowed")
		}
		if p.Chips <= req.ToCall && !slices.Contains(req.ValidActions, "call") && !slices.Contains(req.ValidActions, "allin") {
			return fmt.Errorf("cannot call all-in")
		}
		kind = game.AllIn
	default:
		return fmt.Errorf("unknown action")
	}
	if human && a.Kind != "raise" && a.Amount != 0 {
		return fmt.Errorf("amount is only allowed for a raise")
	}
	validKind := kind
	if kind == game.Check {
		validKind = game.Call
	}
	if kind != game.AllIn && !slices.Contains(h.GetValidActions(), validKind) {
		return fmt.Errorf("action is not legal now")
	}
	if len(t.events) >= 2000 {
		return fmt.Errorf("hand action limit exceeded")
	}
	street := h.Street.String()
	publicBoard := board(h.BoardCards())
	before, bet := p.Chips, p.Bet
	raised := kind == game.Raise || (kind == game.AllIn && p.Chips+p.Bet > h.Betting.CurrentBet)
	if err := h.ProcessAction(kind, amount); err != nil {
		return err
	}
	paid := before - p.Chips
	e := Event{Street: street, Seat: seat, Action: kind.String(), Paid: paid, Board: publicBoard}
	if raised {
		e.RaiseTo = bet + paid
	}
	t.events = append(t.events, e)
	if human && street == "preflop" {
		if paid > 0 {
			t.vpip = true
		}
		if raised {
			t.pfr = true
		}
	}
	for observer := 1; observer < Players; observer++ {
		notice := protocol.PlayerAction{Type: protocol.TypePlayerAction, HandID: fmt.Sprint(t.pending.ID), Street: street, Seat: seat, PlayerName: p.Name, Action: kind.String(), AmountPaid: paid, PlayerBet: p.Bet, PlayerChips: p.Chips, Pot: t.pot()}
		t.states[observer].LastAction = notice
		if err := t.bots[observer].OnPlayerAction(t.states[observer], notice); err != nil {
			return err
		}
	}
	total := t.pot()
	for _, pl := range h.Players {
		if pl.Chips < 0 {
			return fmt.Errorf("negative stack")
		}
		total += pl.Chips
	}
	if total != Players*Stack {
		return fmt.Errorf("chip accounting failed: %d", total)
	}
	return nil
}

func (t *Table) advance() error {
	for !t.h.IsComplete() {
		h := t.h
		if h.ActivePlayer < 0 {
			h.NextStreet()
			continue
		}
		count := 0
		for _, p := range h.Players {
			if p.IsActive() {
				count++
			}
		}
		if count == 1 && h.Players[h.ActivePlayer].Bet == h.Betting.CurrentBet {
			h.NextStreet()
			continue
		}
		if h.ActivePlayer == 0 {
			return nil
		} // The human is NEVER played automatically.
		if err := t.syncBots(); err != nil {
			return err
		}
		seat := h.ActivePlayer
		action, amount, err := t.bots[seat].OnActionRequest(t.states[seat], t.request())
		if err != nil {
			return err
		}
		// Upstream bots occasionally express a stack-sized raise as "raise".
		if action == "raise" && amount >= h.Players[seat].Chips+h.Players[seat].Bet {
			action = "allin"
			amount = 0
		}
		if err := t.apply(Action{Kind: action, Amount: amount}, false); err != nil {
			return fmt.Errorf("bot %d: %w", seat, err)
		}
	}
	return t.settle()
}
func (t *Table) settle() error {
	if t.done {
		return nil
	}
	h := t.h
	if h.Street != game.Showdown {
		h.NextStreet()
	}
	remaining := 0
	for _, p := range h.Players {
		if !p.Folded {
			remaining++
		}
	}
	if remaining > 1 && h.Board.CountCards() != 5 {
		return fmt.Errorf("incomplete showdown board")
	}
	for i, c := range h.BoardCards() {
		if c != t.future[i] {
			return fmt.Errorf("precommitted board mismatch")
		}
	}
	t.showdown = remaining > 1 && !h.Players[0].Folded
	t.finalPot = t.pot()
	h.AwardPots()
	total := 0
	for _, p := range h.Players {
		total += p.Chips
	}
	if total != Players*Stack {
		return fmt.Errorf("settlement is not zero-sum")
	}
	t.done = true
	return nil
}
func (t *Table) Act(a Action) error {
	if t.done || t.h.ActivePlayer != 0 {
		return fmt.Errorf("not the human's turn")
	}
	if err := t.apply(a, true); err != nil {
		return err
	}
	t.pending.Actions = append(t.pending.Actions, a)
	return t.advance()
}
func (t *Table) Pending() Pending {
	p := t.pending
	p.Actions = append([]Action(nil), p.Actions...)
	return p
}
func (t *Table) Done() bool { return t.done }

// View is the sole live API projection. A never receives future cards or a
// seed; B never receives opponent holes before completion, including via logs.
func (t *Table) View() View {
	h := t.h
	v := View{ID: t.pending.ID, Mode: t.pending.Mode, Revision: len(t.pending.Actions), Done: t.done, Street: h.Street.String(), Button: h.Button, Pot: t.pot(), Board: board(h.BoardCards()), Events: append([]Event{}, t.events...)}
	if t.done {
		v.Pot = t.finalPot
		v.Net = h.Players[0].Chips - Stack
	}
	if t.pending.Mode == "B" || t.done {
		v.FutureBoard = board(t.future[:])
		v.FutureHand = poker.Evaluate7Cards(poker.NewHand(t.future[:]...) | h.Players[0].HoleCards).String()
	}
	if h.Board.CountCards() >= 3 {
		v.CurrentHand = poker.EvaluateCards(h.Board | h.Players[0].HoleCards).String()
	}
	for i, p := range h.Players {
		s := SeatView{Seat: i, Name: p.Name, Chips: p.Chips, Bet: p.Bet, Folded: p.Folded, AllIn: p.AllInFlag, Active: !t.done && h.ActivePlayer == i}
		if i == 0 || t.pending.Mode == "A" || t.done {
			s.Hole = cards(p.HoleCards)
		}
		switch i {
		case h.Button:
			s.Position = "D"
		case (h.Button + 1) % Players:
			s.Position = "SB"
		case (h.Button + 2) % Players:
			s.Position = "BB"
		}
		if t.done {
			s.Net = p.Chips - Stack
			s.Bet = 0
			s.Active = false
		}
		v.Seats = append(v.Seats, s)
	}
	if !t.done && h.ActivePlayer == 0 {
		req := t.request()
		p := h.Players[0]
		l := &Legal{ToCall: min(req.ToCall, p.Chips), MinRaiseTo: req.MinBet, MaxRaiseTo: p.Chips + p.Bet, Bet: p.Bet, CanRaise: slices.Contains(req.ValidActions, "raise")}
		for _, a := range h.GetValidActions() {
			l.Actions = append(l.Actions, a.String())
		}
		v.Legal = l
	}
	return v
}
func (t *Table) Record(completed string) Record {
	if !t.done {
		panic("cannot record unfinished hand")
	}
	r := Record{Pending: t.Pending(), CompletedAt: completed, Rules: RulesVersion, VPIP: t.vpip, PFR: t.pfr, Showdown: t.showdown, View: t.View()}
	for i, p := range t.h.Players {
		r.Net[i] = p.Chips - Stack
	}
	return r
}
