package game

import "testing"

func TestNewDeck(t *testing.T) {
	deck := NewDeck()
	if deck.Remaining() != 108 {
		t.Errorf("expected 108 cards, got %d", deck.Remaining())
	}
}

func TestDeckShuffle(t *testing.T) {
	deck1 := NewDeck()
	deck2 := NewDeck()
	deck2.Shuffle()

	// Check that at least some cards are in different positions
	different := false
	for i := 0; i < 10; i++ {
		c1, _ := deck1.Draw()
		c2, _ := deck2.Draw()
		if c1 != c2 {
			different = true
			break
		}
	}
	if !different {
		t.Error("shuffle doesn't seem to change card order")
	}
}

func TestNewGame(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob", "Charlie"})

	if g == nil {
		t.Fatal("game should not be nil")
	}
	if len(g.Players) != 3 {
		t.Errorf("expected 3 players, got %d", len(g.Players))
	}
	// Players should have at least 7 cards (may have more if first card was Draw Two)
	for _, p := range g.Players {
		if p.HandSize() < 7 {
			t.Errorf("player %s should have at least 7 cards, got %d", p.Name, p.HandSize())
		}
	}
	if g.CurrentCard().Value == ValueWildDrawFour {
		t.Error("first card should not be Wild Draw Four")
	}
}

func TestNewGameInvalidPlayers(t *testing.T) {
	if NewGame([]string{"Solo"}) != nil {
		t.Error("should not allow 1 player")
	}
	if NewGame([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}) != nil {
		t.Error("should not allow 9 players")
	}
}

func TestCardCanPlayOn(t *testing.T) {
	red5 := Card{Color: ColorRed, Value: Value5}
	red7 := Card{Color: ColorRed, Value: Value7}
	blue5 := Card{Color: ColorBlue, Value: Value5}
	green3 := Card{Color: ColorGreen, Value: Value3}
	wild := Card{Color: ColorWild, Value: ValueWild}

	// Same color
	if !red7.CanPlayOn(red5, ColorRed) {
		t.Error("should be able to play red on red")
	}

	// Same value
	if !blue5.CanPlayOn(red5, ColorRed) {
		t.Error("should be able to play 5 on 5")
	}

	// Different color and value
	if green3.CanPlayOn(red5, ColorRed) {
		t.Error("should not be able to play green 3 on red 5")
	}

	// Wild can always be played
	if !wild.CanPlayOn(red5, ColorRed) {
		t.Error("wild should be playable on anything")
	}

	// After wild, must match chosen color
	if !red5.CanPlayOn(wild, ColorRed) {
		t.Error("red should be playable when red is chosen")
	}
	if blue5.CanPlayOn(wild, ColorRed) {
		t.Error("blue should not be playable when red is chosen")
	}
}

func TestPlayCardValid(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob"})

	// Find a playable card in current player's hand
	player := g.CurrentPlayerObj()
	playable := player.GetPlayableCards(g.CurrentCard(), g.ChosenColor)

	if len(playable) > 0 {
		err := g.PlayCard(player.ID, playable[0], g.ChosenColor)
		if err != nil {
			t.Errorf("should be able to play valid card: %v", err)
		}
	}
}

func TestPlayCardWrongTurn(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob"})
	g.CurrentPlayer = 0 // Force player 0's turn

	// Try to play as player 1 (wrong player)
	wrongPlayer := g.Players[1]
	err := g.PlayCard(wrongPlayer.ID, 0, ColorRed)
	if err != ErrNotYourTurn {
		t.Errorf("expected ErrNotYourTurn, got %v", err)
	}
}

func TestPlayCardInvalid(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob"})

	// Force a scenario where a card can't be played
	g.DiscardPile = []Card{{Color: ColorRed, Value: Value5}}
	g.ChosenColor = ColorRed

	player := g.CurrentPlayerObj()
	// Give player only a non-matching card
	player.Hand = []Card{{Color: ColorBlue, Value: Value3}}

	err := g.PlayCard(player.ID, 0, ColorRed)
	if err != ErrInvalidCard {
		t.Errorf("expected ErrInvalidCard, got %v", err)
	}
}

func TestSkipCard(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob", "Charlie"})
	g.CurrentPlayer = 0

	// Set up a known state
	g.DiscardPile = []Card{{Color: ColorRed, Value: Value5}}
	g.ChosenColor = ColorRed

	player := g.CurrentPlayerObj()
	// Give player 2 cards so they don't win immediately
	player.Hand = []Card{{Color: ColorRed, Value: ValueSkip}, {Color: ColorBlue, Value: Value1}}

	err := g.PlayCard(player.ID, 0, ColorRed)
	if err != nil {
		t.Fatalf("failed to play skip card: %v", err)
	}

	// Should skip player 1, now player 2's turn
	if g.CurrentPlayer != 2 {
		t.Errorf("expected player 2's turn after skip, got player %d", g.CurrentPlayer)
	}
}

func TestReverseCard(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob", "Charlie"})
	g.CurrentPlayer = 0
	g.Direction = DirectionClockwise

	// Set up a known state
	g.DiscardPile = []Card{{Color: ColorRed, Value: Value5}}
	g.ChosenColor = ColorRed

	player := g.CurrentPlayerObj()
	// Give player 2 cards so they don't win immediately
	player.Hand = []Card{{Color: ColorRed, Value: ValueReverse}, {Color: ColorBlue, Value: Value1}}

	err := g.PlayCard(player.ID, 0, ColorRed)
	if err != nil {
		t.Fatalf("failed to play reverse card: %v", err)
	}

	if g.Direction != DirectionCounterClockwise {
		t.Error("direction should be counter-clockwise after reverse")
	}
}

func TestReverseTwoPlayers(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob"})
	g.CurrentPlayer = 0

	player := g.CurrentPlayerObj()
	player.Hand = []Card{{Color: g.CurrentCard().Color, Value: ValueReverse}}
	g.DiscardPile = []Card{{Color: g.CurrentCard().Color, Value: Value5}}

	g.PlayCard(player.ID, 0, ColorRed)

	// In 2-player, reverse acts as skip, so still player 0's turn
	if g.CurrentPlayer != 0 {
		t.Errorf("in 2-player reverse should act as skip, expected player 0, got %d", g.CurrentPlayer)
	}
}

func TestDrawTwo(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob"})
	g.CurrentPlayer = 0

	// Set up a known state
	g.DiscardPile = []Card{{Color: ColorRed, Value: Value5}}
	g.ChosenColor = ColorRed

	player := g.CurrentPlayerObj()
	// Give player 2 cards so they don't win immediately
	player.Hand = []Card{{Color: ColorRed, Value: ValueDrawTwo}, {Color: ColorBlue, Value: Value1}}

	nextPlayer := g.Players[1]
	initialCards := nextPlayer.HandSize()

	err := g.PlayCard(player.ID, 0, ColorRed)
	if err != nil {
		t.Fatalf("failed to play draw two card: %v", err)
	}

	// Next player should have drawn 2
	if nextPlayer.HandSize() != initialCards+2 {
		t.Errorf("next player should have %d cards, got %d", initialCards+2, nextPlayer.HandSize())
	}

	// Should be back to player 0's turn (player 1 was skipped)
	if g.CurrentPlayer != 0 {
		t.Errorf("expected player 0's turn after draw two, got %d", g.CurrentPlayer)
	}
}

func TestWildDrawFour(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob"})
	g.CurrentPlayer = 0

	// Set up a known state
	g.DiscardPile = []Card{{Color: ColorRed, Value: Value5}}
	g.ChosenColor = ColorRed

	player := g.CurrentPlayerObj()
	// Give player 2 cards so they don't win immediately
	player.Hand = []Card{{Color: ColorWild, Value: ValueWildDrawFour}, {Color: ColorBlue, Value: Value1}}

	nextPlayer := g.Players[1]
	initialCards := nextPlayer.HandSize()

	err := g.PlayCard(player.ID, 0, ColorBlue)
	if err != nil {
		t.Fatalf("failed to play wild draw four: %v", err)
	}

	// Color should be changed
	if g.ChosenColor != ColorBlue {
		t.Errorf("chosen color should be blue, got %v", g.ChosenColor)
	}

	// Next player should have drawn 4
	if nextPlayer.HandSize() != initialCards+4 {
		t.Errorf("next player should have %d cards, got %d", initialCards+4, nextPlayer.HandSize())
	}
}

func TestDrawCard(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob"})

	player := g.CurrentPlayerObj()
	initialCards := player.HandSize()

	_, err := g.DrawCard(player.ID)
	if err != nil {
		t.Errorf("should be able to draw: %v", err)
	}

	if player.HandSize() != initialCards+1 {
		t.Errorf("player should have %d cards, got %d", initialCards+1, player.HandSize())
	}
}

func TestWinCondition(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob"})
	g.CurrentPlayer = 0

	player := g.CurrentPlayerObj()
	// Give player one playable card
	player.Hand = []Card{{Color: g.CurrentCard().Color, Value: Value5}}
	g.DiscardPile = []Card{{Color: g.CurrentCard().Color, Value: Value3}}

	g.PlayCard(player.ID, 0, ColorRed)

	if !g.GameOver {
		t.Error("game should be over")
	}
	if g.Winner != player {
		t.Error("winner should be player who emptied hand")
	}
}

func TestCallUno(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob"})

	player := g.Players[0]
	player.Hand = []Card{{Color: ColorRed, Value: Value5}, {Color: ColorBlue, Value: Value3}}

	err := g.CallUno(player.ID)
	if err != nil {
		t.Errorf("should be able to call uno with 2 cards: %v", err)
	}
	if !player.HasCalledUno {
		t.Error("player should have HasCalledUno set")
	}
}

func TestChallengeUno(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob"})

	target := g.Players[0]
	target.Hand = []Card{{Color: ColorRed, Value: Value5}}
	target.HasCalledUno = false

	challenger := g.Players[1]

	err := g.ChallengeUno(challenger.ID, target.ID)
	if err != nil {
		t.Errorf("challenge should succeed: %v", err)
	}

	// Target should now have 3 cards (1 + 2 penalty)
	if target.HandSize() != 3 {
		t.Errorf("target should have 3 cards after challenge, got %d", target.HandSize())
	}
}

func TestStrictModeWildDrawFour(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob"})
	g.StrictMode = true
	g.CurrentPlayer = 0
	g.DiscardPile = []Card{{Color: ColorRed, Value: Value5}}
	g.ChosenColor = ColorRed

	player := g.CurrentPlayerObj()

	// Player has a red card AND Wild Draw Four - should NOT be able to play Wild Draw Four
	player.Hand = []Card{
		{Color: ColorRed, Value: Value3},
		{Color: ColorWild, Value: ValueWildDrawFour},
	}

	err := g.PlayCard(player.ID, 1, ColorBlue)
	if err != ErrWildDrawFourBluff {
		t.Errorf("expected ErrWildDrawFourBluff in strict mode, got %v", err)
	}

	// Now give player only non-matching colors + Wild Draw Four
	player.Hand = []Card{
		{Color: ColorBlue, Value: Value3},
		{Color: ColorWild, Value: ValueWildDrawFour},
	}

	err = g.PlayCard(player.ID, 1, ColorBlue)
	if err != nil {
		t.Errorf("should be able to play Wild Draw Four with no matching color: %v", err)
	}
}

func TestNonStrictModeWildDrawFour(t *testing.T) {
	g := NewGame([]string{"Alice", "Bob"})
	g.StrictMode = false // Default, but explicit
	g.CurrentPlayer = 0
	g.DiscardPile = []Card{{Color: ColorRed, Value: Value5}}
	g.ChosenColor = ColorRed

	player := g.CurrentPlayerObj()

	// Player has a red card AND Wild Draw Four - should be able to play Wild Draw Four
	player.Hand = []Card{
		{Color: ColorRed, Value: Value3},
		{Color: ColorWild, Value: ValueWildDrawFour},
		{Color: ColorBlue, Value: Value1}, // Extra card so they don't win
	}

	err := g.PlayCard(player.ID, 1, ColorBlue)
	if err != nil {
		t.Errorf("should be able to play Wild Draw Four in non-strict mode: %v", err)
	}
}
