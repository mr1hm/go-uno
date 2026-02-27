package render

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/mr1hm/go-uno/internal/game"
)

// Pre-allocated card images to avoid creating new images every frame
var (
	cardImages    = make(map[string]*ebiten.Image)
	cardBackImage *ebiten.Image
)

func init() {
	// Pre-create card back image
	cardBackImage = ebiten.NewImage(CardWidth, CardHeight)
	cardBackImage.Fill(color.RGBA{30, 30, 80, 255})
}

// getCardImage returns a cached card image or creates one
func getCardImage(card game.Card) *ebiten.Image {
	key := fmt.Sprintf("%d-%d", card.Color, card.Value)
	if img, exists := cardImages[key]; exists {
		return img
	}

	img := ebiten.NewImage(CardWidth, CardHeight)
	img.Fill(getCardColor(card))
	cardImages[key] = img
	return img
}

// getCardColor returns the RGBA color for a card
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

// DrawCard draws a card at the specified position
func DrawCard(screen *ebiten.Image, card game.Card, x, y float64, highlight bool) {
	img := getCardImage(card)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	screen.DrawImage(img, op)

	// Draw border
	borderColor := color.RGBA{255, 255, 255, 255}
	if highlight {
		borderColor = color.RGBA{255, 255, 0, 255}
	}
	drawRect(screen, int(x), int(y), CardWidth, CardHeight, borderColor)

	// Draw card text
	label := card.Value.String()
	if card.IsWild() {
		label = "W"
		if card.Value == game.ValueWildDrawFour {
			label = "+4"
		}
	}
	ebitenutil.DebugPrintAt(screen, label, int(x)+CardWidth/2-8, int(y)+CardHeight/2-8)
}

// DrawCardBack draws a face-down card
func DrawCardBack(screen *ebiten.Image, x, y float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	screen.DrawImage(cardBackImage, op)

	drawRect(screen, int(x), int(y), CardWidth, CardHeight, color.RGBA{100, 100, 150, 255})
}

// drawRect draws a rectangle border (no fill)
func drawRect(screen *ebiten.Image, x, y, width, height int, c color.RGBA) {
	fx, fy := float32(x), float32(y)
	fw, fh := float32(width), float32(height)
	strokeWidth := float32(1)

	vector.StrokeLine(screen, fx, fy, fx+fw, fy, strokeWidth, c, false)
	vector.StrokeLine(screen, fx, fy+fh, fx+fw, fy+fh, strokeWidth, c, false)
	vector.StrokeLine(screen, fx, fy, fx, fy+fh, strokeWidth, c, false)
	vector.StrokeLine(screen, fx+fw, fy, fx+fw, fy+fh, strokeWidth, c, false)
}
