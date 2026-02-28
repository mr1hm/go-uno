package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
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
}

// DrawCardBack draws a face-down card using sprite
func DrawCardBack(screen *ebiten.Image, x, y float64) {
	DrawCardBackRotated(screen, x, y, 0)
}

// DrawCardBackRotated draws a face-down card with rotation (radians)
func DrawCardBackRotated(screen *ebiten.Image, x, y, rotation float64) {
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
	// Rotate around center: translate to origin, rotate, translate back
	op.GeoM.Translate(-CardWidth/2, -CardHeight/2)
	op.GeoM.Rotate(rotation)
	op.GeoM.Translate(x+CardWidth/2, y+CardHeight/2)
	screen.DrawImage(sprite, op)
}

// DrawCardRotated draws a card with rotation (radians)
func DrawCardRotated(screen *ebiten.Image, card game.Card, x, y, rotation float64) {
	sprite := GetCardSprite(card)
	if sprite == nil {
		drawFallbackCard(screen, card, x, y)
		return
	}

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-CardWidth/2, -CardHeight/2)
	op.GeoM.Rotate(rotation)
	op.GeoM.Translate(x+CardWidth/2, y+CardHeight/2)
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
