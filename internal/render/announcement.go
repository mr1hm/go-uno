package render

import (
	"bytes"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/gobold"
)

// Announcement constants
const (
	AnnouncementUNO        = "UNO!"
	AnnouncementFalseCatch = "CAUGHT!"
)

var (
	announcementFace    *text.GoTextFace
	announcementFaceBig *text.GoTextFace // For big fan-style announcements
	labelFace           *text.GoTextFace
	labelFaceSmall      *text.GoTextFace
	fontSource          *text.GoTextFaceSource
)

func init() {
	// Load the Go Bold font (embedded in Go)
	var err error
	fontSource, err = text.NewGoTextFaceSource(bytes.NewReader(gobold.TTF))
	if err != nil {
		panic(err)
	}
	announcementFace = &text.GoTextFace{
		Source: fontSource,
		Size:   72,
	}
	announcementFaceBig = &text.GoTextFace{
		Source: fontSource,
		Size:   140,
	}
	labelFace = &text.GoTextFace{
		Source: fontSource,
		Size:   20,
	}
	labelFaceSmall = &text.GoTextFace{
		Source: fontSource,
		Size:   16,
	}
}

// DrawLabel draws a styled label with shadow at the given position
func DrawLabel(screen *ebiten.Image, label string, x, y float64, size string) {
	face := labelFace
	if size == "small" {
		face = labelFaceSmall
	}

	// Draw shadow
	shadowOp := &text.DrawOptions{}
	shadowOp.GeoM.Translate(x+2, y+2)
	shadowOp.ColorScale.Scale(0, 0, 0, 0.7)
	text.Draw(screen, label, face, shadowOp)

	// Draw main text (white)
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	text.Draw(screen, label, face, op)
}

// DrawLabelRotated draws a styled label with rotation
func DrawLabelRotated(screen *ebiten.Image, label string, x, y float64, rotation float64) {
	// Measure text
	w, h := text.Measure(label, labelFace, 0)

	// Draw shadow
	shadowOp := &text.DrawOptions{}
	shadowOp.GeoM.Translate(-w/2, -h/2)
	shadowOp.GeoM.Rotate(rotation)
	shadowOp.GeoM.Translate(x+2, y+2)
	shadowOp.ColorScale.Scale(0, 0, 0, 0.7)
	text.Draw(screen, label, labelFace, shadowOp)

	// Draw main text
	op := &text.DrawOptions{}
	op.GeoM.Translate(-w/2, -h/2)
	op.GeoM.Rotate(rotation)
	op.GeoM.Translate(x, y)
	text.Draw(screen, label, labelFace, op)
}

const (
	announceFadeInFrames  = 15 // ~0.25 seconds
	announceHoldFrames    = 90 // ~1.5 seconds
	announceFadeOutFrames = 30 // ~0.5 seconds
)

// ShowAnnouncement displays a centered announcement with fade in/out
// playerIndex is the index of the player who triggered the announcement
// isFailure indicates if this is a failure/penalty announcement (changes color)
func (g *UnoGame) ShowAnnouncement(text string, playerIndex int, isFailure bool) {
	g.announcement = text
	g.announcementTimer = announceFadeInFrames + announceHoldFrames + announceFadeOutFrames
	g.announcementFade = 0
	g.announcementPhase = 0 // Start with fade in
	g.announcementPlayer = playerIndex
	g.announcementFailure = isFailure
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

	alpha := g.announcementFade
	centerX := float64(g.screenWidth) / 2
	centerY := float64(g.screenHeight) / 2

	// Use big fan style for UNO-related announcements
	if g.announcement == AnnouncementUNO {
		// Show player name above if not the current viewer
		if g.announcementPlayer != g.playerIndex && g.announcementPlayer >= 0 && g.announcementPlayer < len(g.state.Players) {
			playerName := g.state.Players[g.announcementPlayer].Name
			nameW, _ := text.Measure(playerName, announcementFace, 0)
			nameX := centerX - nameW/2
			nameY := centerY - 80

			// Draw name with shadow
			shadowOp := &text.DrawOptions{}
			shadowOp.GeoM.Translate(nameX+2, nameY+2)
			shadowOp.ColorScale.Scale(0, 0, 0, float32(alpha)*0.7)
			text.Draw(screen, playerName, announcementFace, shadowOp)

			op := &text.DrawOptions{}
			op.GeoM.Translate(nameX, nameY)
			op.ColorScale.Scale(1, 1, 1, float32(alpha))
			text.Draw(screen, playerName, announcementFace, op)
		}

		// Adjust spacing based on text length
		arcRadius := 280.0
		fanAngle := 0.9
		if len(g.announcement) > 5 {
			// Longer text needs more spacing - scale with length
			arcRadius = 250.0 + float64(len(g.announcement))*45.0
			fanAngle = 0.6 + float64(len(g.announcement))*0.06
		}

		// Color: gold for success, orange-red for failure
		var r, gn, b float32 = 1, 0.85, 0.1 // Gold
		if g.announcementFailure {
			r, gn, b = 1, 0.4, 0.2 // Orange-red
		}
		DrawBigFanText(screen, g.announcement, centerX, centerY, arcRadius, fanAngle, float32(alpha), r, gn, b)
		return
	}

	// Regular text for other announcements
	textWidth, textHeight := text.Measure(g.announcement, announcementFace, 0)
	x := (float64(g.screenWidth) - textWidth) / 2
	y := (float64(g.screenHeight) - textHeight) / 2

	// Draw shadow first
	shadowOp := &text.DrawOptions{}
	shadowOp.GeoM.Translate(x+3, y+3)
	shadowOp.ColorScale.Scale(0, 0, 0, float32(alpha)*0.7)
	text.Draw(screen, g.announcement, announcementFace, shadowOp)

	// Draw main text (white)
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.Scale(1, 1, 1, float32(alpha))
	text.Draw(screen, g.announcement, announcementFace, op)
}

// DrawFanText draws text in an arc/fan style, each character at a different angle
// centerX, centerY is the center of the arc, arcRadius is distance from center to characters
// fanAngle is total arc span in radians (e.g., 0.5 for ~30 degrees)
func DrawFanText(screen *ebiten.Image, label string, centerX, centerY, arcRadius, fanAngle float64) {
	if len(label) == 0 {
		return
	}

	chars := []rune(label)
	numChars := len(chars)

	for i, char := range chars {
		// Calculate angle for this character
		var angle float64
		if numChars == 1 {
			angle = 0
		} else {
			t := float64(i) / float64(numChars-1) // 0 to 1
			angle = (t - 0.5) * fanAngle
		}

		// Position on arc (character hangs below center point)
		charX := centerX + arcRadius*math.Sin(angle)
		charY := centerY + arcRadius*(1-math.Cos(angle))

		// Measure single character
		charStr := string(char)
		charW, charH := text.Measure(charStr, labelFace, 0)

		// Draw shadow
		shadowOp := &text.DrawOptions{}
		shadowOp.GeoM.Translate(-charW/2, -charH/2)
		shadowOp.GeoM.Rotate(angle)
		shadowOp.GeoM.Translate(charX+1, charY+1)
		shadowOp.ColorScale.Scale(0, 0, 0, 0.7)
		text.Draw(screen, charStr, labelFace, shadowOp)

		// Draw character
		op := &text.DrawOptions{}
		op.GeoM.Translate(-charW/2, -charH/2)
		op.GeoM.Rotate(angle)
		op.GeoM.Translate(charX, charY)
		// Yellow/gold color for UNO text
		op.ColorScale.Scale(1, 0.9, 0.2, 1)
		text.Draw(screen, charStr, labelFace, op)
	}
}

// DrawBigFanText draws big fan-style text for center announcements
// r, g, b are color values (0-1)
func DrawBigFanText(screen *ebiten.Image, label string, centerX, centerY, arcRadius, fanAngle float64, alpha float32, r, g, b float32) {
	if len(label) == 0 {
		return
	}

	chars := []rune(label)
	numChars := len(chars)

	for i, char := range chars {
		var angle float64
		if numChars == 1 {
			angle = 0
		} else {
			t := float64(i) / float64(numChars-1)
			angle = (t - 0.5) * fanAngle
		}

		// Position on arc
		charX := centerX + arcRadius*math.Sin(angle)
		charY := centerY + arcRadius*(1-math.Cos(angle))*0.3 // Flatter arc for big text

		charStr := string(char)
		charW, charH := text.Measure(charStr, announcementFaceBig, 0)

		// Draw shadow
		shadowOp := &text.DrawOptions{}
		shadowOp.GeoM.Translate(-charW/2, -charH/2)
		shadowOp.GeoM.Rotate(angle)
		shadowOp.GeoM.Translate(charX+4, charY+4)
		shadowOp.ColorScale.Scale(0, 0, 0, alpha*0.7)
		text.Draw(screen, charStr, announcementFaceBig, shadowOp)

		// Draw character with specified color
		op := &text.DrawOptions{}
		op.GeoM.Translate(-charW/2, -charH/2)
		op.GeoM.Rotate(angle)
		op.GeoM.Translate(charX, charY)
		op.ColorScale.Scale(r, g, b, alpha)
		text.Draw(screen, charStr, announcementFaceBig, op)
	}
}

// DrawFanTextRotated draws fan text with an additional base rotation (for opponent positions)
func DrawFanTextRotated(screen *ebiten.Image, label string, centerX, centerY, arcRadius, fanAngle, baseRotation float64) {
	if len(label) == 0 {
		return
	}

	chars := []rune(label)
	numChars := len(chars)

	for i, char := range chars {
		// Calculate angle for this character within the fan
		var fanOffset float64
		if numChars == 1 {
			fanOffset = 0
		} else {
			t := float64(i) / float64(numChars-1) // 0 to 1
			fanOffset = (t - 0.5) * fanAngle
		}

		// Position on arc relative to center, then rotate by base
		localX := arcRadius * math.Sin(fanOffset)
		localY := arcRadius * (1 - math.Cos(fanOffset))

		// Rotate local position by base rotation
		rotatedX := localX*math.Cos(baseRotation) - localY*math.Sin(baseRotation)
		rotatedY := localX*math.Sin(baseRotation) + localY*math.Cos(baseRotation)

		charX := centerX + rotatedX
		charY := centerY + rotatedY

		totalAngle := baseRotation + fanOffset

		// Measure single character
		charStr := string(char)
		charW, charH := text.Measure(charStr, labelFace, 0)

		// Draw shadow
		shadowOp := &text.DrawOptions{}
		shadowOp.GeoM.Translate(-charW/2, -charH/2)
		shadowOp.GeoM.Rotate(totalAngle)
		shadowOp.GeoM.Translate(charX+1, charY+1)
		shadowOp.ColorScale.Scale(0, 0, 0, 0.7)
		text.Draw(screen, charStr, labelFace, shadowOp)

		// Draw character
		op := &text.DrawOptions{}
		op.GeoM.Translate(-charW/2, -charH/2)
		op.GeoM.Rotate(totalAngle)
		op.GeoM.Translate(charX, charY)
		// Yellow/gold color for UNO text
		op.ColorScale.Scale(1, 0.9, 0.2, 1)
		text.Draw(screen, charStr, labelFace, op)
	}
}
