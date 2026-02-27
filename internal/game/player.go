package game

import "errors"

var ErrInvalidCardIndex = errors.New("invalid card index")

type Player struct {
	ID           string
	Name         string
	Hand         []Card
	HasCalledUno bool
}

// NewPlayer creates a player with empty hand
func NewPlayer(id, name string) *Player {
	return &Player{
		ID:   id,
		Name: name,
		Hand: make([]Card, 0),
	}
}

// AddCard adds a card to player's hand
func (p *Player) AddCard(card Card) {
	p.Hand = append(p.Hand, card)
	// Reset Uno call when picking up cards
	if len(p.Hand) > 1 {
		p.HasCalledUno = false
	}
}

// AddCards adds multiple cards
func (p *Player) AddCards(cards []Card) {
	for _, card := range cards {
		p.AddCard(card)
	}
}

// RemoveCard removes card at index, returns it
func (p *Player) RemoveCard(index int) (Card, error) {
	if index < 0 || index >= len(p.Hand) {
		return Card{}, ErrInvalidCardIndex
	}
	card := p.Hand[index]
	p.Hand = append(p.Hand[:index], p.Hand[index+1:]...)
	return card, nil
}

// HandSize returns number of cards
func (p *Player) HandSize() int {
	return len(p.Hand)
}

// HasWon returns true if hand is empty
func (p *Player) HasWon() bool {
	return len(p.Hand) == 0
}

// GetPlayableCards returns indices of cards that can be played on the given card
func (p *Player) GetPlayableCards(topCard Card, chosenColor Color) []int {
	playable := make([]int, 0)
	for i, card := range p.Hand {
		if card.CanPlayOn(topCard, chosenColor) {
			playable = append(playable, i)
		}
	}
	return playable
}

// HasColorMatch returns true if player has any card matching the given color
func (p *Player) HasColorMatch(color Color) bool {
	for _, card := range p.Hand {
		if !card.IsWild() && card.Color == color {
			return true
		}
	}
	return false
}
