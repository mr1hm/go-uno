package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/mr1hm/go-uno/internal/game"
)

// DrawCard draws a card at the specified position using sprites
func DrawCard(screen *ebiten.Image, card game.Card, x, y float64, highlight bool) {
	sprite := GetCardSprite(card)
	if sprite == nil {
		// Fallback to colored rectangle if sprite not found
		drawFallbackCard(screen, card, x, y)
		return
	}

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	screen.DrawImage(sprite, op)

	// Draw highlight border if selected (white glow effect)
	if highlight {
		drawRect(screen, int(x)-3, int(y)-3, CardWidth+6, CardHeight+6, color.RGBA{255, 255, 255, 200})
		drawRect(screen, int(x)-2, int(y)-2, CardWidth+4, CardHeight+4, color.RGBA{255, 255, 255, 255})
		drawRect(screen, int(x)-1, int(y)-1, CardWidth+2, CardHeight+2, color.RGBA{255, 255, 255, 200})
	}
}

// DrawCardBack draws a face-down card using sprite
func DrawCardBack(screen *ebiten.Image, x, y float64) {
	sprite := GetCardBackSprite()
	if sprite == nil {
		// Fallback
		fallback := ebiten.NewImage(CardWidth, CardHeight)
		fallback.Fill(color.RGBA{30, 30, 80, 255})
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(x, y)
		screen.DrawImage(fallback, op)
		return
	}

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	screen.DrawImage(sprite, op)
}

// drawFallbackCard draws a simple colored rectangle as fallback
func drawFallbackCard(screen *ebiten.Image, card game.Card, x, y float64) {
	fallback := ebiten.NewImage(CardWidth, CardHeight)
	fallback.Fill(getCardColor(card))
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	screen.DrawImage(fallback, op)
}

// getCardColor returns the RGBA color for a card (fallback only)
func getCardColor(card game.Card) color.RGBA {
	switch card.Color {
	case game.ColorRed:
		return color.RGBA{220, 50, 50, 255}
	case game.ColorYellow:
		return color.RGBA{255, 220, 0, 255}
	case game.ColorGreen:
		return color.RGBA{50, 180, 50, 255}
	case game.ColorBlue:
		return color.RGBA{50, 100, 220, 255}
	default:
		return color.RGBA{50, 50, 50, 255}
	}
}

// drawRect draws a rectangle border (no fill)
func drawRect(screen *ebiten.Image, x, y, width, height int, c color.RGBA) {
	fx, fy := float32(x), float32(y)
	fw, fh := float32(width), float32(height)
	strokeWidth := float32(2)

	vector.StrokeLine(screen, fx, fy, fx+fw, fy, strokeWidth, c, false)
	vector.StrokeLine(screen, fx, fy+fh, fx+fw, fy+fh, strokeWidth, c, false)
	vector.StrokeLine(screen, fx, fy, fx, fy+fh, strokeWidth, c, false)
	vector.StrokeLine(screen, fx+fw, fy, fx+fw, fy+fh, strokeWidth, c, false)
}
