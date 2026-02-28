package render

import (
	"bytes"
	"embed"
	"image"
	_ "image/png"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mr1hm/go-uno/internal/game"
)

//go:embed assets/*
var assetsFS embed.FS

var (
	// Pre-composited card images
	cardSprites    = make(map[string]*ebiten.Image)
	cardBackSprite *ebiten.Image
	backgroundImg  *ebiten.Image

	// Base images for compositing
	colorBases    = make(map[game.Color]*ebiten.Image)
	valueOverlays = make(map[game.Value]*ebiten.Image)
)

func init() {
	loadBaseAssets()
	preCompositeCards()
}

func loadBaseAssets() {
	// Load color bases
	colorFiles := map[game.Color]string{
		game.ColorRed:    "assets/red_base.png",
		game.ColorYellow: "assets/yellow_base.png",
		game.ColorGreen:  "assets/green_base.png",
		game.ColorBlue:   "assets/blue_base.png",
	}
	for color, path := range colorFiles {
		colorBases[color] = loadImage(path)
	}

	// Load value overlays
	valueFiles := map[game.Value]string{
		game.Value0:        "assets/_0.png",
		game.Value1:        "assets/_1.png",
		game.Value2:        "assets/_2.png",
		game.Value3:        "assets/_3.png",
		game.Value4:        "assets/_4.png",
		game.Value5:        "assets/_5.png",
		game.Value6:        "assets/_6.png",
		game.Value7:        "assets/_7.png",
		game.Value8:        "assets/_8.png",
		game.Value9:        "assets/_9.png",
		game.ValueSkip:     "assets/_interdit.png",
		game.ValueReverse:  "assets/_revers.png",
		game.ValueDrawTwo:  "assets/_draw2.png",
	}
	for value, path := range valueFiles {
		valueOverlays[value] = loadImage(path)
	}

	// Load wild cards (complete images)
	cardSprites["wild"] = loadImage("assets/_wild.png")
	cardSprites["wild_draw_four"] = loadImage("assets/_wild_draw.png")

	// Load card back
	cardBackSprite = loadImage("assets/back.png")

	// Load background
	backgroundImg = loadImage("assets/background.png")
}

// GetBackgroundSprite returns the background image
func GetBackgroundSprite() *ebiten.Image {
	return backgroundImg
}

func preCompositeCards() {
	colors := []game.Color{game.ColorRed, game.ColorYellow, game.ColorGreen, game.ColorBlue}
	values := []game.Value{
		game.Value0, game.Value1, game.Value2, game.Value3, game.Value4,
		game.Value5, game.Value6, game.Value7, game.Value8, game.Value9,
		game.ValueSkip, game.ValueReverse, game.ValueDrawTwo,
	}

	for _, color := range colors {
		base := colorBases[color]
		if base == nil {
			continue
		}

		for _, value := range values {
			overlay := valueOverlays[value]
			if overlay == nil {
				continue
			}

			// Create composited card
			bounds := base.Bounds()
			composited := ebiten.NewImage(bounds.Dx(), bounds.Dy())
			composited.DrawImage(base, nil)
			composited.DrawImage(overlay, nil)

			key := cardKey(color, value)
			cardSprites[key] = composited
		}
	}
}

func loadImage(path string) *ebiten.Image {
	data, err := assetsFS.ReadFile(path)
	if err != nil {
		log.Printf("Failed to load %s: %v", path, err)
		return nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		log.Printf("Failed to decode %s: %v", path, err)
		return nil
	}

	return ebiten.NewImageFromImage(img)
}

func cardKey(color game.Color, value game.Value) string {
	return color.String() + "_" + value.String()
}

// GetCardSprite returns the pre-composited sprite for a card
func GetCardSprite(card game.Card) *ebiten.Image {
	if card.Value == game.ValueWild {
		return cardSprites["wild"]
	}
	if card.Value == game.ValueWildDrawFour {
		return cardSprites["wild_draw_four"]
	}
	return cardSprites[cardKey(card.Color, card.Value)]
}

// GetCardBackSprite returns the card back sprite
func GetCardBackSprite() *ebiten.Image {
	return cardBackSprite
}
