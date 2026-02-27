package game

import "fmt"

type Color int

const (
	ColorRed Color = iota
	ColorYellow
	ColorGreen
	ColorBlue
	ColorWild // For Wild and Wild Draw Four
)

func (c Color) String() string {
	switch c {
	case ColorRed:
		return "Red"
	case ColorYellow:
		return "Yellow"
	case ColorGreen:
		return "Green"
	case ColorBlue:
		return "Blue"
	case ColorWild:
		return "Wild"
	default:
		return "Unknown"
	}
}

type Value int

const (
	Value0 Value = iota
	Value1
	Value2
	Value3
	Value4
	Value5
	Value6
	Value7
	Value8
	Value9
	ValueSkip
	ValueReverse
	ValueDrawTwo
	ValueWild
	ValueWildDrawFour
)

func (v Value) String() string {
	switch v {
	case Value0:
		return "0"
	case Value1:
		return "1"
	case Value2:
		return "2"
	case Value3:
		return "3"
	case Value4:
		return "4"
	case Value5:
		return "5"
	case Value6:
		return "6"
	case Value7:
		return "7"
	case Value8:
		return "8"
	case Value9:
		return "9"
	case ValueSkip:
		return "Skip"
	case ValueReverse:
		return "Reverse"
	case ValueDrawTwo:
		return "Draw Two"
	case ValueWild:
		return "Wild"
	case ValueWildDrawFour:
		return "Wild Draw Four"
	default:
		return "Unknown"
	}
}

type Card struct {
	Color Color
	Value Value
}

// String returns a human-readable card name
func (c Card) String() string {
	if c.IsWild() {
		return c.Value.String()
	}
	return fmt.Sprintf("%s %s", c.Color, c.Value)
}

// IsWild returns true if the card is Wild or Wild Draw Four
func (c Card) IsWild() bool {
	return c.Value == ValueWild || c.Value == ValueWildDrawFour
}

// IsActionCard returns true for Skip, Reverse, Draw Two
func (c Card) IsActionCard() bool {
	return c.Value == ValueSkip || c.Value == ValueReverse || c.Value == ValueDrawTwo
}

// CanPlayOn checks if this card can be played on the given card
// chosenColor is the color chosen when a wild was played (ignored if top card isn't wild)
func (c Card) CanPlayOn(topCard Card, chosenColor Color) bool {
	// Wild cards can always be played
	if c.IsWild() {
		return true
	}

	// If top card is wild, must match chosen color
	if topCard.IsWild() {
		return c.Color == chosenColor
	}

	// Match by color or value
	return c.Color == topCard.Color || c.Value == topCard.Value
}
