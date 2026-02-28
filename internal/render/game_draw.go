package render

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/mr1hm/go-uno/internal/game"
)

// Cached UI images to avoid allocations every frame
var (
	colorIndicator *ebiten.Image
	turnBannerGlow *ebiten.Image
	turnBanner     *ebiten.Image
	waitBanner     *ebiten.Image
	passButton     *ebiten.Image
	unoButtonRed   *ebiten.Image
	challengeBtn   *ebiten.Image
	colorBoxes     [4]*ebiten.Image
)

func (g *UnoGame) drawDiscardPile(screen *ebiten.Image) {
	x := float64(g.screenWidth/2 + 20)
	y := float64(g.screenHeight/2 - CardHeight/2 + PlayAreaOffsetY)

	// Draw a few cards underneath with stagger for depth effect
	// Use deterministic offsets based on discard pile size
	pileSize := len(g.state.DiscardPile)
	staggerCards := min(pileSize-1, 4) // Show up to 4 cards underneath

	for i := range staggerCards {
		// Deterministic "random" offset based on card index
		seed := float64((pileSize - staggerCards + i) * 7)
		offsetX := (seed/10 - float64(int(seed/10))) * 8 - 4      // -4 to 4
		offsetY := (seed/13 - float64(int(seed/13))) * 6 - 3      // -3 to 3
		rotation := (seed/17 - float64(int(seed/17))) * 0.2 - 0.1 // -0.1 to 0.1 radians

		DrawCardBackRotated(screen, x+offsetX, y+offsetY, rotation)
	}

	// Draw top card with slight rotation
	topRotation := 0.0
	if pileSize > 1 {
		topRotation = (float64(pileSize*13%100) / 100) * 0.15 - 0.075
	}
	DrawCardRotated(screen, g.state.CurrentCard(), x, y, topRotation)

	// Show chosen color indicator (cached image)
	if colorIndicator == nil {
		colorIndicator = ebiten.NewImage(20, 20)
	}
	colorIndicator.Fill(getColorRGBA(g.state.ChosenColor))
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x+CardWidth/2-10, y-25)
	screen.DrawImage(colorIndicator, op)
}

func (g *UnoGame) drawDirectionArrow(screen *ebiten.Image) {
	arrow := GetDirectionArrow(g.state.Direction == game.DirectionClockwise)
	if arrow == nil {
		return
	}

	// Position arrow above the card piles, centered
	arrowBounds := arrow.Bounds()
	arrowSize := 48.0 // Scale the arrow to this size
	scale := arrowSize / float64(arrowBounds.Dx())

	x := float64(g.screenWidth/2) - arrowSize/2
	y := float64(g.screenHeight/2) - CardHeight/2 + PlayAreaOffsetY - arrowSize - 20

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(x, y)
	// Tint the arrow gold/yellow to match UNO theme
	op.ColorScale.Scale(1.0, 0.85, 0.2, 1.0) // RGB: gold/yellow
	screen.DrawImage(arrow, op)
}

func (g *UnoGame) drawDrawPile(screen *ebiten.Image) {
	x := float64(g.screenWidth/2 - CardWidth - 20)
	y := float64(g.screenHeight/2 - CardHeight/2 + PlayAreaOffsetY)

	// Draw staggered cards for depth effect
	remaining := g.state.DrawPile.Remaining()
	staggerCards := min(remaining, 5) // Show up to 5 cards in stack

	for i := range staggerCards {
		// Deterministic offset based on index
		seed := float64(i * 17)
		offsetX := (seed/10 - float64(int(seed/10))) * 6 - 3       // -3 to 3
		offsetY := (seed/13 - float64(int(seed/13))) * 4 - 2       // -2 to 2
		rotation := (seed/19 - float64(int(seed/19))) * 0.12 - 0.06 // -0.06 to 0.06 radians

		DrawCardBackRotated(screen, x+offsetX, y+offsetY, rotation)
	}

	ebitenutil.DebugPrintAt(screen, "DRAW", int(x)+25, int(y)+CardHeight/2-8)
}

func (g *UnoGame) drawPlayerHand(screen *ebiten.Image) {
	player := g.state.Players[g.playerIndex]
	isMyTurn := g.state.CurrentPlayer == g.playerIndex

	// Hide cards that are still animating (they're at the end of the hand)
	animatingCount := g.CountAnimatingCards(g.playerIndex)
	visibleCards := len(player.Hand) - animatingCount

	if visibleCards == 0 {
		return
	}

	// Fan parameters
	arcRadius := 800.0 // Radius of the arc (larger = flatter)
	centerX := float64(g.screenWidth) / 2
	centerY := float64(g.screenHeight) + arcRadius - CardHeight - 20 // Push hand to bottom edge

	// Adjust fan angle based on number of cards (collapse inward with fewer cards)
	var actualFanAngle float64
	switch {
	case visibleCards <= 1:
		actualFanAngle = 0
	case visibleCards <= 3:
		actualFanAngle = 0.15
	case visibleCards <= 5:
		actualFanAngle = 0.25
	case visibleCards <= 7:
		actualFanAngle = 0.4
	default:
		actualFanAngle = min(0.6, 0.4+float64(visibleCards-7)*0.03)
	}

	// Update card lift animations
	g.updateCardLiftAnimations(visibleCards)

	// Store hovered card info for drawing on top (z-index via draw order)
	var hoveredCardX, hoveredCardY, hoveredAngle float64
	var hoveredCard *game.Card

	for i := range visibleCards {
		card := player.Hand[i]
		// Skip drawing card if it's being dragged
		if g.dragging && i == g.dragCardIndex {
			continue
		}

		// Calculate angle for this card (-fanAngle/2 to +fanAngle/2)
		var angle float64
		if visibleCards == 1 {
			angle = 0
		} else {
			t := float64(i) / float64(visibleCards-1) // 0 to 1
			angle = (t - 0.5) * actualFanAngle
		}

		// Calculate position on arc
		x := centerX + arcRadius*math.Sin(angle) - CardWidth/2
		y := centerY - arcRadius*math.Cos(angle) - g.cardLiftY[i]

		// If this is the hovered card, save it to draw last (on top)
		if i == g.selectedCard && !g.dragging {
			hoveredCard = &card
			hoveredCardX = x
			hoveredCardY = y
			hoveredAngle = angle
			continue
		}

		DrawCardRotated(screen, card, x, y, angle)
	}

	// Draw hovered card last so it appears on top (z-index)
	if hoveredCard != nil {
		DrawCardRotated(screen, *hoveredCard, hoveredCardX, hoveredCardY, hoveredAngle)
	}

	// Draw dragged card on top (follows mouse, no rotation)
	if g.dragging && g.dragCardIndex >= 0 && g.dragCardIndex < len(player.Hand) {
		DrawCard(screen, player.Hand[g.dragCardIndex], g.dragX, g.dragY, false)
	}

	// Player name and card count (centered above hand)
	label := fmt.Sprintf("%s (%d)", player.Name, len(player.Hand))
	if isMyTurn {
		label = ">> " + label + " <<"
	}
	// Center the label
	labelX := float64(g.screenWidth) / 2
	labelY := float64(g.screenHeight) - CardHeight - 70
	DrawLabel(screen, label, labelX-float64(len(label)*5), labelY, "normal")

	// Draw fan-style UNO! underneath player name if they've called it
	if player.HasCalledUno && player.HandSize() <= 2 {
		unoY := labelY + 30 // Below the name
		DrawFanText(screen, "UNO!", labelX, unoY, 80, 0.8) // Larger radius and angle for spacing
	}

	// Draw action text above cards
	if action := g.GetPlayerAction(g.playerIndex); action != "" {
		actionY := labelY - 25
		DrawLabel(screen, action, labelX-float64(len(action)*5), actionY, "small")
	}
}

func (g *UnoGame) drawOpponents(screen *ebiten.Image) {
	for i, player := range g.state.Players {
		if i == g.playerIndex {
			continue
		}

		animatingCount := g.CountAnimatingCards(i)
		visibleCards := min(player.HandSize()-animatingCount, 10)
		if visibleCards <= 0 {
			continue
		}

		// Fan parameters for opponents
		fanAngle := 0.25 // Smaller spread for opponents
		arcRadius := 300.0

		// Position, base rotation, and label position for each opponent
		var anchorX, anchorY float64
		var baseRotation float64
		var labelX, labelY int

		// Common offsets for symmetric positioning
		sideMargin := 120 // Distance from edge for card anchor
		sideY := float64(g.screenHeight)/2 + float64(PlayAreaOffsetY)

		switch i {
		case 1: // Left side - fan facing right
			anchorX = float64(sideMargin)
			anchorY = sideY
			baseRotation = math.Pi / 2
			labelX = sideMargin + CardHeight/2 + 30 // To the right of cards (towards center)
			labelY = g.screenHeight/2 + PlayAreaOffsetY
		case 2: // Top - fan facing down
			anchorX = float64(g.screenWidth) / 2
			anchorY = 100
			baseRotation = math.Pi
			labelX = g.screenWidth / 2
			labelY = 100 + CardHeight/2 + 30 // Below cards (towards center)
		case 3: // Right side - fan facing left
			anchorX = float64(g.screenWidth - sideMargin)
			anchorY = sideY
			baseRotation = -math.Pi / 2
			labelX = g.screenWidth - sideMargin - CardHeight/2 - 30 // To the left of cards (towards center)
			labelY = g.screenHeight/2 + PlayAreaOffsetY
		}

		// Draw cards in fan arrangement
		for j := range visibleCards {
			var cardAngle float64
			if visibleCards == 1 {
				cardAngle = 0
			} else {
				t := float64(j) / float64(visibleCards-1)
				cardAngle = (t - 0.5) * fanAngle
			}

			totalAngle := baseRotation + cardAngle

			// Position card relative to anchor, fanning outward
			offsetX := arcRadius * math.Sin(cardAngle)
			offsetY := -arcRadius * (1 - math.Cos(cardAngle)) * 0.3 // Slight arc

			// Rotate offset by base rotation
			rotatedOffsetX := offsetX*math.Cos(baseRotation) - offsetY*math.Sin(baseRotation)
			rotatedOffsetY := offsetX*math.Sin(baseRotation) + offsetY*math.Cos(baseRotation)

			cardX := anchorX + rotatedOffsetX - CardWidth/2
			cardY := anchorY + rotatedOffsetY - CardHeight/2

			DrawCardBackRotated(screen, cardX, cardY, totalAngle)
		}

		// Draw rotated label with nice font
		indicator := ""
		if g.state.CurrentPlayer == i {
			indicator = " <<"
		}
		label := fmt.Sprintf("%s (%d)%s", player.Name, player.HandSize(), indicator)
		DrawLabelRotated(screen, label, float64(labelX), float64(labelY), baseRotation)

		// Draw fan-style UNO! text near the player if they've called it
		if player.HasCalledUno && player.HandSize() <= 2 {
			// Position UNO! text relative to label, offset perpendicular to rotation
			unoOffsetDist := 35.0
			unoX := float64(labelX) + unoOffsetDist*math.Cos(baseRotation+math.Pi/2)
			unoY := float64(labelY) + unoOffsetDist*math.Sin(baseRotation+math.Pi/2)
			DrawFanTextRotated(screen, "UNO!", unoX, unoY, 60, 0.6, baseRotation) // Larger radius and angle
		}

		// Draw action text near opponent
		if action := g.GetPlayerAction(i); action != "" {
			actionOffsetDist := -30.0
			actionX := float64(labelX) + actionOffsetDist*math.Cos(baseRotation+math.Pi/2)
			actionY := float64(labelY) + actionOffsetDist*math.Sin(baseRotation+math.Pi/2)
			DrawLabelRotated(screen, action, actionX, actionY, baseRotation)
		}
	}
}

func (g *UnoGame) drawUI(screen *ebiten.Image) {
	// Turn indicator
	currentPlayer := g.state.CurrentPlayerObj()
	if g.state.CurrentPlayer == g.playerIndex {
		// Draw centered "YOUR TURN" pill banner
		bannerWidth := 280
		bannerHeight := 36
		bannerX := (g.screenWidth - bannerWidth) / 2
		bannerY := 15

		// Outer glow (cached)
		if turnBannerGlow == nil {
			turnBannerGlow = ebiten.NewImage(bannerWidth+8, bannerHeight+8)
		}
		turnBannerGlow.Fill(color.RGBA{255, 200, 0, 80})
		glowOp := &ebiten.DrawImageOptions{}
		glowOp.GeoM.Translate(float64(bannerX-4), float64(bannerY-4))
		screen.DrawImage(turnBannerGlow, glowOp)

		// Main banner (cached)
		if turnBanner == nil {
			turnBanner = ebiten.NewImage(bannerWidth, bannerHeight)
		}
		turnBanner.Fill(color.RGBA{200, 30, 30, 240})
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(bannerX), float64(bannerY))
		screen.DrawImage(turnBanner, op)

		// Text centered
		ebitenutil.DebugPrintAt(screen, "YOUR TURN", bannerX+100, bannerY+10)
	} else {
		// Smaller waiting indicator (cached)
		waitText := fmt.Sprintf("Waiting for %s...", currentPlayer.Name)
		bannerX := (g.screenWidth - 200) / 2

		if waitBanner == nil {
			waitBanner = ebiten.NewImage(200, 28)
		}
		waitBanner.Fill(color.RGBA{50, 50, 50, 180})
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(bannerX), 15)
		screen.DrawImage(waitBanner, op)

		ebitenutil.DebugPrintAt(screen, waitText, bannerX+10, 22)
	}

	// Direction arrow indicator (centered above the card piles)
	g.drawDirectionArrow(screen)

	// Debug: Show current card and required color
	topCard := g.state.CurrentCard()
	debugInfo := fmt.Sprintf("Top: %s | Required Color: %s", topCard, g.state.ChosenColor)
	ebitenutil.DebugPrintAt(screen, debugInfo, 10, 50)

	// Debug: Show playable card indices and selected card info
	if g.state.CurrentPlayer == g.playerIndex {
		player := g.state.CurrentPlayerObj()
		playable := player.GetPlayableCards(topCard, g.state.ChosenColor)
		playableStr := fmt.Sprintf("Playable indices: %v", playable)
		ebitenutil.DebugPrintAt(screen, playableStr, 10, 70)

		// Show selected card details
		if g.selectedCard >= 0 && g.selectedCard < len(player.Hand) {
			selectedCard := player.Hand[g.selectedCard]
			canPlay := selectedCard.CanPlayOn(topCard, g.state.ChosenColor)
			selectedStr := fmt.Sprintf("Selected [%d]: %s (can play: %v)", g.selectedCard, selectedCard, canPlay)
			ebitenutil.DebugPrintAt(screen, selectedStr, 10, 110)
		}
	}

	// Message
	if g.message != "" {
		ebitenutil.DebugPrintAt(screen, g.message, 10, 90)
	}

	// Pass button (cached)
	if g.state.CurrentPlayer == g.playerIndex {
		passX := g.screenWidth - 120
		passY := g.screenHeight - 60
		if passButton == nil {
			passButton = ebiten.NewImage(100, 40)
		}
		passButton.Fill(color.RGBA{100, 100, 100, 255})
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(passX), float64(passY))
		screen.DrawImage(passButton, op)
		ebitenutil.DebugPrintAt(screen, "PASS", passX+35, passY+12)
	}

	// Buttons - centered vertically relative to card piles
	discardX := g.screenWidth/2 + 20
	buttonX := discardX + CardWidth + 15
	pileCenterY := g.screenHeight/2 + PlayAreaOffsetY
	buttonGap := 10
	buttonHeight := 40
	totalHeight := buttonHeight*2 + buttonGap // Two buttons stacked

	// UNO button
	unoX := buttonX
	unoY := pileCenterY - totalHeight/2
	if unoButtonRed == nil {
		unoButtonRed = ebiten.NewImage(80, 40)
	}
	unoButtonRed.Fill(color.RGBA{200, 30, 30, 255})
	unoOp := &ebiten.DrawImageOptions{}
	unoOp.GeoM.Translate(float64(unoX), float64(unoY))
	screen.DrawImage(unoButtonRed, unoOp)
	ebitenutil.DebugPrintAt(screen, "UNO!", unoX+25, unoY+12)

	// Challenge button - below UNO button
	chalX := unoX
	chalY := unoY + buttonHeight + buttonGap
	if challengeBtn == nil {
		challengeBtn = ebiten.NewImage(80, 40)
	}
	// Always gray - no visual hint when someone is vulnerable
	challengeBtn.Fill(color.RGBA{80, 80, 80, 255})
	chalOp := &ebiten.DrawImageOptions{}
	chalOp.GeoM.Translate(float64(chalX), float64(chalY))
	screen.DrawImage(challengeBtn, chalOp)
	ebitenutil.DebugPrintAt(screen, "CATCH", chalX+20, chalY+12)
}

func (g *UnoGame) drawColorPicker(screen *ebiten.Image) {
	// Darken background (simple fill is cheaper than overlay image)
	colors := []game.Color{game.ColorRed, game.ColorYellow, game.ColorGreen, game.ColorBlue}
	boxSize := 60
	startX := g.screenWidth/2 - (boxSize*4+30)/2
	startY := g.screenHeight/2 - boxSize/2

	ebitenutil.DebugPrintAt(screen, "Choose a color:", startX, startY-30)

	// Draw color boxes (cached)
	for i, c := range colors {
		if colorBoxes[i] == nil {
			colorBoxes[i] = ebiten.NewImage(boxSize, boxSize)
			colorBoxes[i].Fill(getColorRGBA(c))
		}
		x := startX + i*(boxSize+10)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x), float64(startY))
		screen.DrawImage(colorBoxes[i], op)
	}
}

// Cached popup image
var caughtPopupBg *ebiten.Image

func (g *UnoGame) drawCaughtPopup(screen *ebiten.Image) {
	popupWidth := 350
	popupHeight := 100
	popupX := (g.screenWidth - popupWidth) / 2
	popupY := (g.screenHeight - popupHeight) / 2

	// Draw popup background
	if caughtPopupBg == nil {
		caughtPopupBg = ebiten.NewImage(popupWidth, popupHeight)
	}
	caughtPopupBg.Fill(color.RGBA{200, 50, 50, 240})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(popupX), float64(popupY))
	screen.DrawImage(caughtPopupBg, op)

	// Draw text
	title := "CAUGHT!"
	msg := fmt.Sprintf("%s caught %s!", g.caughtByName, g.caughtPlayerName)
	penalty := fmt.Sprintf("%s draws 2 cards!", g.caughtPlayerName)

	ebitenutil.DebugPrintAt(screen, title, popupX+140, popupY+15)
	ebitenutil.DebugPrintAt(screen, msg, popupX+50, popupY+45)
	ebitenutil.DebugPrintAt(screen, penalty, popupX+70, popupY+70)
}

func (g *UnoGame) drawGameOver(screen *ebiten.Image) {
	overlay := ebiten.NewImage(g.screenWidth, g.screenHeight)
	overlay.Fill(color.RGBA{0, 0, 0, 200})
	screen.DrawImage(overlay, nil)

	msg := fmt.Sprintf("%s WINS!", g.state.Winner.Name)
	ebitenutil.DebugPrintAt(screen, msg, g.screenWidth/2-50, g.screenHeight/2-20)
	ebitenutil.DebugPrintAt(screen, "Click to play again", g.screenWidth/2-70, g.screenHeight/2+10)
}

func getColorRGBA(c game.Color) color.RGBA {
	switch c {
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

// Cache for rotated text images to avoid allocations every frame
var rotatedTextCache = make(map[string]*ebiten.Image)

// drawRotatedText draws text at the given position with rotation
// The text is rendered to an offscreen image first, then drawn with rotation
func drawRotatedText(screen *ebiten.Image, text string, x, y int, rotation float64) {
	// Estimate text size (roughly 6 pixels per character width, 16 height for debug font)
	textWidth := len(text) * 6
	textHeight := 16

	// Check cache for existing text image
	textImg, exists := rotatedTextCache[text]
	if !exists {
		// Create and cache offscreen image for text
		textImg = ebiten.NewImage(textWidth, textHeight)
		ebitenutil.DebugPrintAt(textImg, text, 0, 0)
		rotatedTextCache[text] = textImg
	}

	// Draw with rotation around center
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(textWidth)/2, -float64(textHeight)/2)
	op.GeoM.Rotate(rotation)
	op.GeoM.Translate(float64(x)+float64(textWidth)/2, float64(y)+float64(textHeight)/2)
	screen.DrawImage(textImg, op)
}
