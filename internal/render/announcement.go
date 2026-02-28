package render

import (
	"bytes"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/gobold"
)

var (
	announcementFace *text.GoTextFace
)

func init() {
	// Load the Go Bold font (embedded in Go)
	source, err := text.NewGoTextFaceSource(bytes.NewReader(gobold.TTF))
	if err != nil {
		panic(err)
	}
	announcementFace = &text.GoTextFace{
		Source: source,
		Size:   48,
	}
}

const (
	announceFadeInFrames  = 15  // ~0.25 seconds
	announceHoldFrames    = 90  // ~1.5 seconds
	announceFadeOutFrames = 30  // ~0.5 seconds
)

// ShowAnnouncement displays a centered announcement with fade in/out
func (g *UnoGame) ShowAnnouncement(text string) {
	g.announcement = text
	g.announcementTimer = announceFadeInFrames + announceHoldFrames + announceFadeOutFrames
	g.announcementFade = 0
	g.announcementPhase = 0 // Start with fade in
}

// updateAnnouncement handles the fade animation
func (g *UnoGame) updateAnnouncement() {
	if g.announcementTimer <= 0 {
		return
	}

	g.announcementTimer--

	// Calculate which phase we're in
	if g.announcementTimer > announceHoldFrames+announceFadeOutFrames {
		// Fade in phase
		g.announcementPhase = 0
		elapsed := announceFadeInFrames - (g.announcementTimer - announceHoldFrames - announceFadeOutFrames)
		g.announcementFade = float64(elapsed) / float64(announceFadeInFrames)
	} else if g.announcementTimer > announceFadeOutFrames {
		// Hold phase
		g.announcementPhase = 1
		g.announcementFade = 1.0
	} else {
		// Fade out phase
		g.announcementPhase = 2
		g.announcementFade = float64(g.announcementTimer) / float64(announceFadeOutFrames)
	}
}

// drawAnnouncement renders the centered announcement with fade effect
func (g *UnoGame) drawAnnouncement(screen *ebiten.Image) {
	if g.announcementTimer <= 0 || g.announcement == "" {
		return
	}

	// Measure text size
	textWidth, textHeight := text.Measure(g.announcement, announcementFace, 0)

	// Center position
	x := (float64(g.screenWidth) - textWidth) / 2
	y := (float64(g.screenHeight) - textHeight) / 2

	// Draw with fade
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)

	// White text with alpha based on fade
	alpha := g.announcementFade
	op.ColorScale.Scale(1, 1, 1, float32(alpha))

	// Draw shadow first (slightly offset, darker)
	shadowOp := &text.DrawOptions{}
	shadowOp.GeoM.Translate(x+3, y+3)
	shadowOp.ColorScale.Scale(0, 0, 0, float32(alpha)*0.7)
	text.Draw(screen, g.announcement, announcementFace, shadowOp)

	// Draw main text
	text.Draw(screen, g.announcement, announcementFace, op)
}
