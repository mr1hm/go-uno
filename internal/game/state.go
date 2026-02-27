package game

import "fmt"

type Direction int

const (
	DirectionClockwise Direction = iota
	DirectionCounterClockwise
)

type GameState struct {
	Players       []*Player
	CurrentPlayer int
	Direction     Direction
	DrawPile      *Deck
	DiscardPile   []Card
	ChosenColor   Color // Set when wild is played
	GameOver      bool
	Winner        *Player
	StrictMode    bool // If true, Wild Draw Four only playable when no matching color
}

// NewGame creates a new game with given player names
// Deals 7 cards to each player, flips first card
func NewGame(playerNames []string) *GameState {
	if len(playerNames) < 2 || len(playerNames) > 8 {
		return nil
	}

	deck := NewDeck()
	deck.Shuffle()

	players := make([]*Player, len(playerNames))
	for i, name := range playerNames {
		players[i] = NewPlayer(fmt.Sprintf("player-%d", i), name)
		players[i].AddCards(deck.DrawN(7))
	}

	// Draw first card for discard pile
	// If it's a Wild Draw Four, put it back and draw again
	var firstCard Card
	for {
		card, ok := deck.Draw()
		if !ok {
			return nil // shouldn't happen with fresh deck
		}
		if card.Value == ValueWildDrawFour {
			deck.AddToBottom([]Card{card})
			deck.Shuffle()
			continue
		}
		firstCard = card
		break
	}

	g := &GameState{
		Players:       players,
		CurrentPlayer: 0,
		Direction:     DirectionClockwise,
		DrawPile:      deck,
		DiscardPile:   []Card{firstCard},
		ChosenColor:   firstCard.Color,
	}

	// Apply first card effect if it's an action card
	g.applyFirstCardEffect(firstCard)

	return g
}

// applyFirstCardEffect handles special first card rules
func (g *GameState) applyFirstCardEffect(card Card) {
	switch card.Value {
	case ValueSkip:
		g.NextPlayer() // First player is skipped
	case ValueReverse:
		g.ReverseDirection()
		if len(g.Players) == 2 {
			g.NextPlayer() // Acts as skip in 2-player
		}
	case ValueDrawTwo:
		// First player draws 2 and is skipped
		g.CurrentPlayerObj().AddCards(g.DrawPile.DrawN(2))
		g.NextPlayer()
	case ValueWild:
		// First player chooses color (handled in first PlayCard)
		g.ChosenColor = ColorRed // Default, will be changed
	}
}

// CurrentCard returns the top of discard pile
func (g *GameState) CurrentCard() Card {
	if len(g.DiscardPile) == 0 {
		return Card{}
	}
	return g.DiscardPile[len(g.DiscardPile)-1]
}

// CurrentPlayerObj returns the current player
func (g *GameState) CurrentPlayerObj() *Player {
	return g.Players[g.CurrentPlayer]
}

// NextPlayer advances to next player (respects direction)
func (g *GameState) NextPlayer() {
	if g.Direction == DirectionClockwise {
		g.CurrentPlayer = (g.CurrentPlayer + 1) % len(g.Players)
	} else {
		g.CurrentPlayer = (g.CurrentPlayer - 1 + len(g.Players)) % len(g.Players)
	}
}

// ReverseDirection flips play direction
func (g *GameState) ReverseDirection() {
	if g.Direction == DirectionClockwise {
		g.Direction = DirectionCounterClockwise
	} else {
		g.Direction = DirectionClockwise
	}
}

// GetPlayerByID returns player by ID or nil
func (g *GameState) GetPlayerByID(id string) *Player {
	for _, p := range g.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// reshuffleDiscardIntoDraw moves discard pile (except top) back to draw pile
func (g *GameState) reshuffleDiscardIntoDraw() {
	if len(g.DiscardPile) <= 1 {
		return
	}
	// Keep top card
	topCard := g.DiscardPile[len(g.DiscardPile)-1]
	// Move rest to draw pile
	g.DrawPile.AddToBottom(g.DiscardPile[:len(g.DiscardPile)-1])
	g.DrawPile.Shuffle()
	g.DiscardPile = []Card{topCard}
}
