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
	colorIndicators [5]*ebiten.Image // One per game.Color
	turnBannerGlow  *ebiten.Image
	turnBanner      *ebiten.Image
	waitBanner      *ebiten.Image
	passButton      *ebiten.Image
	unoButtonRed    *ebiten.Image
	challengeBtn    *ebiten.Image
	colorBoxes      [4]*ebiten.Image
	gameOverOverlay *ebiten.Image
	lastOverlaySize [2]int
)

func (g *UnoGame) drawDiscardPile(screen *ebiten.Image) {
	cardW := g.cardWidthF()
	cardH := g.cardHeightF()
	offsetY := g.playAreaOffsetYF()

	x := float64(g.screenWidth/2) + 20*g.scale()
	y := float64(g.screenHeight/2) - cardH/2 + offsetY

	// Draw a few cards underneath with stagger for depth effect
	pileSize := len(g.state.DiscardPile)
	staggerCards := min(pileSize-1, 4)

	for i := range staggerCards {
		seed := float64((pileSize - staggerCards + i) * 7)
		staggerX := (seed/10 - float64(int(seed/10))) * 8 - 4
		staggerY := (seed/13 - float64(int(seed/13))) * 6 - 3
		rotation := (seed/17 - float64(int(seed/17))) * 0.2 - 0.1

		g.drawCardBackRotatedScaled(screen, x+staggerX*g.scale(), y+staggerY*g.scale(), rotation)
	}

	// Skip drawing top card if there's a play animation (card is still in flight)
	if len(g.playAnims) > 0 {
		return
	}

	// Draw top card with slight rotation
	topRotation := 0.0
	if pileSize > 1 {
		topRotation = (float64(pileSize*13%100) / 100) * 0.15 - 0.075
	}
	g.drawCardRotatedScaled(screen, g.state.CurrentCard(), x, y, topRotation)

	// Show chosen color indicator (cached per color)
	colorIdx := int(g.state.ChosenColor)
	if colorIdx >= 0 && colorIdx < len(colorIndicators) {
		if colorIndicators[colorIdx] == nil {
			colorIndicators[colorIdx] = ebiten.NewImage(20, 20)
			colorIndicators[colorIdx].Fill(getColorRGBA(g.state.ChosenColor))
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(x+cardW/2-10, y-25*g.scale())
		screen.DrawImage(colorIndicators[colorIdx], op)
	}
}

func (g *UnoGame) drawDirectionArrow(screen *ebiten.Image) {
	arrow := GetDirectionArrow(g.state.Direction == game.DirectionClockwise)
	if arrow == nil {
		return
	}

	cardH := g.cardHeightF()
	offsetY := g.playAreaOffsetYF()

	arrowBounds := arrow.Bounds()
	arrowSize := 48.0 * g.scale()
	arrowScale := arrowSize / float64(arrowBounds.Dx())

	x := float64(g.screenWidth/2) - arrowSize/2
	y := float64(g.screenHeight/2) - cardH/2 + offsetY - arrowSize - 20*g.scale()

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(arrowScale, arrowScale)
	op.GeoM.Translate(x, y)
	op.ColorScale.Scale(1.0, 0.85, 0.2, 1.0)
	screen.DrawImage(arrow, op)
}

func (g *UnoGame) drawDrawPile(screen *ebiten.Image) {
	cardW := g.cardWidthF()
	cardH := g.cardHeightF()
	offsetY := g.playAreaOffsetYF()

	x := float64(g.screenWidth/2) - cardW - 20*g.scale()
	y := float64(g.screenHeight/2) - cardH/2 + offsetY

	remaining := g.state.DrawPile.Remaining()
	staggerCards := min(remaining, 5)

	for i := range staggerCards {
		seed := float64(i * 17)
		staggerX := (seed/10 - float64(int(seed/10))) * 6 - 3
		staggerY := (seed/13 - float64(int(seed/13))) * 4 - 2
		rotation := (seed/19 - float64(int(seed/19))) * 0.12 - 0.06

		g.drawCardBackRotatedScaled(screen, x+staggerX*g.scale(), y+staggerY*g.scale(), rotation)
	}

	ebitenutil.DebugPrintAt(screen, "DRAW", int(x+cardW/2-15), int(y+cardH/2-8))
}

func (g *UnoGame) drawPlayerHand(screen *ebiten.Image) {
	player := g.state.Players[g.playerIndex]
	isMyTurn := g.state.CurrentPlayer == g.playerIndex

	animatingCount := g.CountAnimatingCards(g.playerIndex)
	visibleCards := len(player.Hand) - animatingCount

	if visibleCards == 0 {
		return
	}

	cardW := g.cardWidthF()
	cardH := g.cardHeightF()

	// Skip lift animations during drag - cards are static
	if !g.dragging {
		g.updateCardLiftAnimations(visibleCards)
	}

	// Fan parameters
	arcRadius := 800.0 * g.scale()
	centerX := float64(g.screenWidth) / 2
	centerY := float64(g.screenHeight) + arcRadius - cardH - 20*g.scale()

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

	for i := range visibleCards {
		card := player.Hand[i]
		// Skip dragged card (drawn separately)
		if g.dragging && i == g.dragCardIndex {
			continue
		}

		var angle float64
		if visibleCards == 1 {
			angle = 0
		} else {
			t := float64(i) / float64(visibleCards-1)
			angle = (t - 0.5) * actualFanAngle
		}

		x := centerX + arcRadius*math.Sin(angle) - cardW/2
		y := centerY - arcRadius*math.Cos(angle) - g.cardLiftY[i]*g.scale()

		g.drawCardRotatedScaled(screen, card, x, y, angle)
	}

	// Draw labels
	label := fmt.Sprintf("%s (%d)", player.Name, len(player.Hand))
	if isMyTurn {
		label = ">> " + label + " <<"
	}
	labelX := float64(g.screenWidth) / 2
	labelY := float64(g.screenHeight) - cardH - 70*g.scale()
	DrawLabel(screen, label, labelX-float64(len(label)*5), labelY, "normal")

	if player.HasCalledUno && player.HandSize() <= 2 {
		unoY := labelY + 30*g.scale()
		DrawFanText(screen, "UNO!", labelX, unoY, 80, 0.8)
	}

	if action := g.GetPlayerAction(g.playerIndex); action != "" {
		actionY := labelY - 25*g.scale()
		DrawLabel(screen, action, labelX-float64(len(action)*5), actionY, "small")
	}
}

func (g *UnoGame) drawOpponents(screen *ebiten.Image) {
	cardW := g.cardWidthF()
	cardH := g.cardHeightF()
	offsetY := g.playAreaOffsetYF()
	scale := g.scale()

	for i, player := range g.state.Players {
		if i == g.playerIndex {
			continue
		}

		animatingCount := g.CountAnimatingCards(i)
		visibleCards := min(player.HandSize()-animatingCount, 10)
		if visibleCards <= 0 {
			continue
		}

		fanAngle := 0.25
		arcRadius := 300.0 * scale

		var anchorX, anchorY float64
		var baseRotation float64
		var labelX, labelY float64

		sideMargin := 120.0 * scale
		sideY := float64(g.screenHeight)/2 + offsetY

		switch i {
		case 1: // Left side
			anchorX = sideMargin
			anchorY = sideY
			baseRotation = math.Pi / 2
			labelX = sideMargin + cardH/2 + 30*scale
			labelY = float64(g.screenHeight)/2 + offsetY
		case 2: // Top
			anchorX = float64(g.screenWidth) / 2
			anchorY = 100 * scale
			baseRotation = math.Pi
			labelX = float64(g.screenWidth) / 2
			labelY = 100*scale + cardH/2 + 30*scale
		case 3: // Right side
			anchorX = float64(g.screenWidth) - sideMargin
			anchorY = sideY
			baseRotation = -math.Pi / 2
			labelX = float64(g.screenWidth) - sideMargin - cardH/2 - 30*scale
			labelY = float64(g.screenHeight)/2 + offsetY
		}

		for j := range visibleCards {
			var cardAngle float64
			if visibleCards == 1 {
				cardAngle = 0
			} else {
				t := float64(j) / float64(visibleCards-1)
				cardAngle = (t - 0.5) * fanAngle
			}

			totalAngle := baseRotation + cardAngle

			fanOffsetX := arcRadius * math.Sin(cardAngle)
			fanOffsetY := -arcRadius * (1 - math.Cos(cardAngle)) * 0.3

			rotatedOffsetX := fanOffsetX*math.Cos(baseRotation) - fanOffsetY*math.Sin(baseRotation)
			rotatedOffsetY := fanOffsetX*math.Sin(baseRotation) + fanOffsetY*math.Cos(baseRotation)

			cardX := anchorX + rotatedOffsetX - cardW/2
			cardY := anchorY + rotatedOffsetY - cardH/2

			g.drawCardBackRotatedScaled(screen, cardX, cardY, totalAngle)
		}

		indicator := ""
		if g.state.CurrentPlayer == i {
			indicator = " <<"
		}
		label := fmt.Sprintf("%s (%d)%s", player.Name, player.HandSize(), indicator)
		DrawLabelRotated(screen, label, labelX, labelY, baseRotation)

		if player.HasCalledUno && player.HandSize() <= 2 {
			unoOffsetDist := 35.0 * scale
			unoX := labelX + unoOffsetDist*math.Cos(baseRotation+math.Pi/2)
			unoY := labelY + unoOffsetDist*math.Sin(baseRotation+math.Pi/2)
			DrawFanTextRotated(screen, "UNO!", unoX, unoY, 60, 0.6, baseRotation)
		}

		if action := g.GetPlayerAction(i); action != "" {
			actionOffsetDist := -30.0 * scale
			actionX := labelX + actionOffsetDist*math.Cos(baseRotation+math.Pi/2)
			actionY := labelY + actionOffsetDist*math.Sin(baseRotation+math.Pi/2)
			DrawLabelRotated(screen, action, actionX, actionY, baseRotation)
		}
	}
}

// Cached UI dimensions to detect when we need to recreate images
var lastUIScale float64 = -1


func (g *UnoGame) drawUI(screen *ebiten.Image) {
	scale := g.scale()
	cardW := g.cardWidthF()
	offsetY := g.playAreaOffsetYF()

	// Recreate cached UI images only when scale changes
	if scale != lastUIScale {
		lastUIScale = scale
		// Invalidate all cached UI images
		turnBannerGlow = nil
		turnBanner = nil
		waitBanner = nil
		passButton = nil
		unoButtonRed = nil
		challengeBtn = nil
	}

	currentPlayer := g.state.CurrentPlayerObj()
	if g.state.CurrentPlayer == g.playerIndex {
		bannerWidth := int(280 * scale)
		bannerHeight := int(36 * scale)
		bannerX := (g.screenWidth - bannerWidth) / 2
		bannerY := int(15 * scale)

		// Create and fill only once
		if turnBannerGlow == nil {
			turnBannerGlow = ebiten.NewImage(bannerWidth+8, bannerHeight+8)
			turnBannerGlow.Fill(color.RGBA{255, 200, 0, 80})
		}
		glowOp := &ebiten.DrawImageOptions{}
		glowOp.GeoM.Translate(float64(bannerX-4), float64(bannerY-4))
		screen.DrawImage(turnBannerGlow, glowOp)

		if turnBanner == nil {
			turnBanner = ebiten.NewImage(bannerWidth, bannerHeight)
			turnBanner.Fill(color.RGBA{200, 30, 30, 240})
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(bannerX), float64(bannerY))
		screen.DrawImage(turnBanner, op)

		ebitenutil.DebugPrintAt(screen, "YOUR TURN", bannerX+bannerWidth/2-35, bannerY+bannerHeight/3)
	} else {
		waitText := fmt.Sprintf("Waiting for %s...", currentPlayer.Name)
		bannerWidth := int(200 * scale)
		bannerX := (g.screenWidth - bannerWidth) / 2

		if waitBanner == nil {
			waitBanner = ebiten.NewImage(bannerWidth, int(28*scale))
			waitBanner.Fill(color.RGBA{50, 50, 50, 180})
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(bannerX), 15*scale)
		screen.DrawImage(waitBanner, op)

		ebitenutil.DebugPrintAt(screen, waitText, bannerX+10, int(22*scale))
	}

	g.drawDirectionArrow(screen)

	// Pass button
	if g.state.CurrentPlayer == g.playerIndex {
		passX := g.screenWidth - int(120*scale)
		passY := g.screenHeight - int(60*scale)
		btnW := int(100 * scale)
		btnH := int(40 * scale)
		if passButton == nil {
			passButton = ebiten.NewImage(btnW, btnH)
			passButton.Fill(color.RGBA{100, 100, 100, 255})
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(passX), float64(passY))
		screen.DrawImage(passButton, op)
		ebitenutil.DebugPrintAt(screen, "PASS", passX+btnW/3, passY+btnH/3)
	}

	// UNO and Challenge buttons
	discardX := float64(g.screenWidth/2) + 20*scale
	buttonX := int(discardX + cardW + 15*scale)
	pileCenterY := g.screenHeight/2 + int(offsetY)
	buttonGap := int(10 * scale)
	buttonHeight := int(40 * scale)
	buttonWidth := int(80 * scale)
	totalHeight := buttonHeight*2 + buttonGap

	unoX := buttonX
	unoY := pileCenterY - totalHeight/2
	if unoButtonRed == nil {
		unoButtonRed = ebiten.NewImage(buttonWidth, buttonHeight)
		unoButtonRed.Fill(color.RGBA{200, 30, 30, 255})
	}
	unoOp := &ebiten.DrawImageOptions{}
	unoOp.GeoM.Translate(float64(unoX), float64(unoY))
	screen.DrawImage(unoButtonRed, unoOp)
	ebitenutil.DebugPrintAt(screen, "UNO!", unoX+buttonWidth/3, unoY+buttonHeight/3)

	chalX := unoX
	chalY := unoY + buttonHeight + buttonGap
	if challengeBtn == nil {
		challengeBtn = ebiten.NewImage(buttonWidth, buttonHeight)
		challengeBtn.Fill(color.RGBA{80, 80, 80, 255})
	}
	chalOp := &ebiten.DrawImageOptions{}
	chalOp.GeoM.Translate(float64(chalX), float64(chalY))
	screen.DrawImage(challengeBtn, chalOp)
	ebitenutil.DebugPrintAt(screen, "CATCH", chalX+buttonWidth/4, chalY+buttonHeight/3)
}

func (g *UnoGame) drawColorPicker(screen *ebiten.Image) {
	scale := g.scale()

	colors := []game.Color{game.ColorRed, game.ColorYellow, game.ColorGreen, game.ColorBlue}
	boxSize := int(60 * scale)
	gap := int(10 * scale)
	totalWidth := boxSize*4 + gap*3
	startX := g.screenWidth/2 - totalWidth/2
	startY := g.screenHeight/2 - boxSize/2

	ebitenutil.DebugPrintAt(screen, "Choose a color:", startX, startY-int(30*scale))

	// Draw color boxes (recreate if size changed)
	for i, c := range colors {
		if colorBoxes[i] == nil || colorBoxes[i].Bounds().Dx() != boxSize {
			colorBoxes[i] = ebiten.NewImage(boxSize, boxSize)
			colorBoxes[i].Fill(getColorRGBA(c))
		}
		x := startX + i*(boxSize+gap)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x), float64(startY))
		screen.DrawImage(colorBoxes[i], op)
	}
}

// Cached popup image (created once and reused)
var caughtPopupBg *ebiten.Image

func (g *UnoGame) drawCaughtPopup(screen *ebiten.Image) {
	popupWidth := 350
	popupHeight := 100
	popupX := (g.screenWidth - popupWidth) / 2
	popupY := (g.screenHeight - popupHeight) / 2

	// Draw popup background (create and fill only once)
	if caughtPopupBg == nil {
		caughtPopupBg = ebiten.NewImage(popupWidth, popupHeight)
		caughtPopupBg.Fill(color.RGBA{200, 50, 50, 240})
	}
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
	// Recreate overlay only when screen size changes
	if gameOverOverlay == nil || lastOverlaySize[0] != g.screenWidth || lastOverlaySize[1] != g.screenHeight {
		gameOverOverlay = ebiten.NewImage(g.screenWidth, g.screenHeight)
		gameOverOverlay.Fill(color.RGBA{0, 0, 0, 200})
		lastOverlaySize[0] = g.screenWidth
		lastOverlaySize[1] = g.screenHeight
	}
	screen.DrawImage(gameOverOverlay, nil)

	msg := fmt.Sprintf("%s WINS!", g.state.Winner.Name)
	ebitenutil.DebugPrintAt(screen, msg, g.screenWidth/2-50, g.screenHeight/2-20)
	ebitenutil.DebugPrintAt(screen, "Click to play again", g.screenWidth/2-70, g.screenHeight/2+10)
}

// Scaled card drawing methods

func (g *UnoGame) drawCardScaled(screen *ebiten.Image, card game.Card, x, y float64) {
	sprite := GetCardSprite(card)
	if sprite == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(g.scale(), g.scale())
	op.GeoM.Translate(x, y)
	screen.DrawImage(sprite, op)
}

func (g *UnoGame) drawCardRotatedScaled(screen *ebiten.Image, card game.Card, x, y, rotation float64) {
	sprite := GetCardSprite(card)
	if sprite == nil {
		return
	}

	cardW := g.cardWidthF()
	cardH := g.cardHeightF()

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(g.scale(), g.scale())
	op.GeoM.Translate(-cardW/2, -cardH/2)
	op.GeoM.Rotate(rotation)
	op.GeoM.Translate(x+cardW/2, y+cardH/2)
	screen.DrawImage(sprite, op)
}

func (g *UnoGame) drawCardBackScaled(screen *ebiten.Image, x, y float64) {
	g.drawCardBackRotatedScaled(screen, x, y, 0)
}

func (g *UnoGame) drawCardBackRotatedScaled(screen *ebiten.Image, x, y, rotation float64) {
	sprite := GetCardBackSprite()
	if sprite == nil {
		return
	}

	cardW := g.cardWidthF()
	cardH := g.cardHeightF()

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(g.scale(), g.scale())
	op.GeoM.Translate(-cardW/2, -cardH/2)
	op.GeoM.Rotate(rotation)
	op.GeoM.Translate(x+cardW/2, y+cardH/2)
	screen.DrawImage(sprite, op)
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

