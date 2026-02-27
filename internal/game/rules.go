package game

import "errors"

var (
	ErrNotYourTurn       = errors.New("not your turn")
	ErrInvalidCard       = errors.New("cannot play this card")
	ErrGameOver          = errors.New("game is over")
	ErrInvalidColor      = errors.New("must choose a color for wild card")
	ErrCannotCallUno     = errors.New("can only call uno with one card")
	ErrInvalidTarget     = errors.New("invalid target player")
	ErrWildDrawFourBluff = errors.New("cannot play Wild Draw Four when you have matching color (strict mode)")
)

// PlayCard attempts to play a card from the current player's hand
// For wild cards, chosenColor must be set (not ColorWild)
func (g *GameState) PlayCard(playerID string, cardIndex int, chosenColor Color) error {
	if g.GameOver {
		return ErrGameOver
	}

	player := g.GetPlayerByID(playerID)
	if player == nil || player != g.CurrentPlayerObj() {
		return ErrNotYourTurn
	}

	if cardIndex < 0 || cardIndex >= len(player.Hand) {
		return ErrInvalidCardIndex
	}

	card := player.Hand[cardIndex]

	// Check if card can be played
	if !card.CanPlayOn(g.CurrentCard(), g.ChosenColor) {
		return ErrInvalidCard
	}

	// Wild cards require a color choice
	if card.IsWild() && chosenColor == ColorWild {
		return ErrInvalidColor
	}

	// Strict mode: Wild Draw Four only playable if no matching color in hand
	if g.StrictMode && card.Value == ValueWildDrawFour {
		if player.HasColorMatch(g.ChosenColor) {
			return ErrWildDrawFourBluff
		}
	}

	// Remove card from hand
	player.RemoveCard(cardIndex)

	// Add to discard pile
	g.DiscardPile = append(g.DiscardPile, card)

	// Set chosen color
	if card.IsWild() {
		g.ChosenColor = chosenColor
	} else {
		g.ChosenColor = card.Color
	}

	// Check for win
	if player.HasWon() {
		g.GameOver = true
		g.Winner = player
		return nil
	}

	// Apply card effect and advance turn
	g.applyCardEffect(card)

	return nil
}

// applyCardEffect handles Skip, Reverse, Draw Two, Wild Draw Four
func (g *GameState) applyCardEffect(card Card) {
	switch card.Value {
	case ValueSkip:
		g.NextPlayer() // Skip next player
		g.NextPlayer() // Move to player after

	case ValueReverse:
		g.ReverseDirection()
		if len(g.Players) == 2 {
			// In 2-player, reverse acts as skip
			g.NextPlayer()
			g.NextPlayer()
		} else {
			g.NextPlayer()
		}

	case ValueDrawTwo:
		g.NextPlayer()
		// Next player draws 2
		g.drawCardsForCurrentPlayer(2)
		g.NextPlayer() // And loses their turn

	case ValueWildDrawFour:
		g.NextPlayer()
		// Next player draws 4
		g.drawCardsForCurrentPlayer(4)
		g.NextPlayer() // And loses their turn

	default:
		// Regular card or Wild (no special effect beyond color)
		g.NextPlayer()
	}
}

// drawCardsForCurrentPlayer draws cards for current player, reshuffling if needed
func (g *GameState) drawCardsForCurrentPlayer(n int) {
	for i := 0; i < n; i++ {
		if g.DrawPile.Remaining() == 0 {
			g.reshuffleDiscardIntoDraw()
		}
		if card, ok := g.DrawPile.Draw(); ok {
			g.CurrentPlayerObj().AddCard(card)
		}
	}
}

// DrawCard makes current player draw from pile
func (g *GameState) DrawCard(playerID string) (Card, error) {
	if g.GameOver {
		return Card{}, ErrGameOver
	}

	player := g.GetPlayerByID(playerID)
	if player == nil || player != g.CurrentPlayerObj() {
		return Card{}, ErrNotYourTurn
	}

	// Reshuffle if needed
	if g.DrawPile.Remaining() == 0 {
		g.reshuffleDiscardIntoDraw()
	}

	card, ok := g.DrawPile.Draw()
	if !ok {
		// Still no cards (very rare edge case)
		return Card{}, errors.New("no cards available")
	}

	player.AddCard(card)
	return card, nil
}

// PassTurn passes turn after drawing (if drawn card wasn't played)
func (g *GameState) PassTurn(playerID string) error {
	if g.GameOver {
		return ErrGameOver
	}

	player := g.GetPlayerByID(playerID)
	if player == nil || player != g.CurrentPlayerObj() {
		return ErrNotYourTurn
	}

	g.NextPlayer()
	return nil
}

// CallUno marks that player called Uno
func (g *GameState) CallUno(playerID string) error {
	player := g.GetPlayerByID(playerID)
	if player == nil {
		return ErrInvalidTarget
	}

	if player.HandSize() > 2 {
		return ErrCannotCallUno
	}

	player.HasCalledUno = true
	return nil
}

// ChallengeUno penalizes player who didn't call Uno (they draw 2)
func (g *GameState) ChallengeUno(challengerID, targetID string) error {
	challenger := g.GetPlayerByID(challengerID)
	target := g.GetPlayerByID(targetID)

	if challenger == nil || target == nil {
		return ErrInvalidTarget
	}

	// Target must have exactly 1 card and not have called Uno
	if target.HandSize() != 1 || target.HasCalledUno {
		return errors.New("invalid challenge")
	}

	// Target draws 2 penalty cards
	for i := 0; i < 2; i++ {
		if g.DrawPile.Remaining() == 0 {
			g.reshuffleDiscardIntoDraw()
		}
		if card, ok := g.DrawPile.Draw(); ok {
			target.AddCard(card)
		}
	}

	return nil
}
