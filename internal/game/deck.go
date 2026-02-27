package game

import (
	"math/rand"
	"time"
)

type Deck struct {
	cards []Card
}

// NewDeck creates a standard 108-card Uno deck
func NewDeck() *Deck {
	d := &Deck{
		cards: make([]Card, 0, 108),
	}

	colors := []Color{ColorRed, ColorYellow, ColorGreen, ColorBlue}

	for _, color := range colors {
		// One 0 per color
		d.cards = append(d.cards, Card{Color: color, Value: Value0})

		// Two of each 1-9 per color
		for v := Value1; v <= Value9; v++ {
			d.cards = append(d.cards, Card{Color: color, Value: v})
			d.cards = append(d.cards, Card{Color: color, Value: v})
		}

		// Two of each action card per color
		for _, v := range []Value{ValueSkip, ValueReverse, ValueDrawTwo} {
			d.cards = append(d.cards, Card{Color: color, Value: v})
			d.cards = append(d.cards, Card{Color: color, Value: v})
		}
	}

	// Four Wild cards
	for i := 0; i < 4; i++ {
		d.cards = append(d.cards, Card{Color: ColorWild, Value: ValueWild})
	}

	// Four Wild Draw Four cards
	for i := 0; i < 4; i++ {
		d.cards = append(d.cards, Card{Color: ColorWild, Value: ValueWildDrawFour})
	}

	return d
}

// Shuffle randomizes the deck order
func (d *Deck) Shuffle() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(d.cards), func(i, j int) {
		d.cards[i], d.cards[j] = d.cards[j], d.cards[i]
	})
}

// Draw removes and returns the top card
func (d *Deck) Draw() (Card, bool) {
	if len(d.cards) == 0 {
		return Card{}, false
	}
	card := d.cards[len(d.cards)-1]
	d.cards = d.cards[:len(d.cards)-1]
	return card, true
}

// DrawN draws n cards, returns slice
func (d *Deck) DrawN(n int) []Card {
	drawn := make([]Card, 0, n)
	for i := 0; i < n; i++ {
		if card, ok := d.Draw(); ok {
			drawn = append(drawn, card)
		} else {
			break
		}
	}
	return drawn
}

// AddToBottom adds cards to bottom (for reshuffling discard)
func (d *Deck) AddToBottom(cards []Card) {
	d.cards = append(cards, d.cards...)
}

// Remaining returns number of cards left
func (d *Deck) Remaining() int {
	return len(d.cards)
}
