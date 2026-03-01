package render

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mr1hm/go-uno/internal/game"
)

// handleCardHover detects which card the mouse is over (for lift animation)
func (g *UnoGame) handleCardHover() {
	player := g.state.Players[g.playerIndex]
	mx, my := ebiten.CursorPosition()

	if g.dragging {
		return
	}

	animatingCount := g.CountAnimatingCards(g.playerIndex)
	visibleCards := len(player.Hand) - animatingCount
	if visibleCards == 0 {
		g.selectedCard = -1
		return
	}

	cardW := g.cardWidthF()
	cardH := g.cardHeightF()
	scale := g.scale()

	arcRadius := 800.0 * scale
	centerX := float64(g.screenWidth) / 2
	centerY := float64(g.screenHeight) + arcRadius - cardH - 20*scale

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

	g.selectedCard = -1
	for i := visibleCards - 1; i >= 0; i-- {
		var angle float64
		if visibleCards == 1 {
			angle = 0
		} else {
			t := float64(i) / float64(visibleCards-1)
			angle = (t - 0.5) * actualFanAngle
		}

		cardCenterX := centerX + arcRadius*math.Sin(angle)
		cardCenterY := centerY - arcRadius*math.Cos(angle)

		dx := float64(mx) - cardCenterX
		dy := float64(my) - cardCenterY

		cos := math.Cos(-angle)
		sin := math.Sin(-angle)
		localX := dx*cos - dy*sin
		localY := dx*sin + dy*cos

		if localX >= -cardW/2 && localX <= cardW/2 &&
			localY >= -cardH/2 && localY <= cardH/2+30*scale {
			g.selectedCard = i
			break
		}
	}
}

// handleGlobalButtons handles UNO and Challenge buttons (clickable anytime)
func (g *UnoGame) handleGlobalButtons() {
	mx, my := ebiten.CursorPosition()
	player := g.state.Players[g.playerIndex]
	scale := g.scale()
	cardW := g.cardWidthF()
	offsetY := g.playAreaOffsetYF()

	discardX := float64(g.screenWidth/2) + 20*scale
	buttonX := int(discardX + cardW + 15*scale)
	pileCenterY := g.screenHeight/2 + int(offsetY)
	buttonGap := int(10 * scale)
	buttonHeight := int(40 * scale)
	buttonWidth := int(80 * scale)
	totalHeight := buttonHeight*2 + buttonGap

	unoX := buttonX
	unoY := pileCenterY - totalHeight/2
	if mx >= unoX && mx < unoX+buttonWidth && my >= unoY && my < unoY+buttonHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !g.unoClickedThisTurn && !player.HasCalledUno {
			g.unoClickedThisTurn = true
			if err := g.state.CallUno(player.ID); err != nil {
				g.message = "False UNO! +2 cards"
				g.ShowAnnouncement(AnnouncementUNO, g.playerIndex, true)
			} else if player.HandSize() == 1 {
				g.message = "Called UNO! (just in time)"
				g.ShowAnnouncement(AnnouncementUNO, g.playerIndex, false)
			} else {
				g.message = "Called UNO!"
				g.ShowAnnouncement(AnnouncementUNO, g.playerIndex, false)
			}
		}
	}

	chalX := unoX
	chalY := unoY + buttonHeight + buttonGap
	if mx >= chalX && mx < chalX+buttonWidth && my >= chalY && my < chalY+buttonHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			var vulnerableTarget *game.Player
			for _, p := range g.state.Players {
				if p.ID != player.ID && p.HandSize() == 1 && !p.HasCalledUno {
					vulnerableTarget = p
					break
				}
			}

			if vulnerableTarget != nil {
				if err := g.state.ChallengeUno(player.ID, vulnerableTarget.ID); err == nil {
					g.message = fmt.Sprintf("Caught %s! +2 cards", vulnerableTarget.Name)
					g.ShowAnnouncement(AnnouncementFalseCatch, g.playerIndex, false)
					g.caughtPopup = 120
					g.caughtPlayerName = vulnerableTarget.Name
					g.caughtByName = player.Name
				}
			} else {
				g.state.PenalizePlayer(player.ID, 2)
				g.message = "False challenge! +2 cards"
				g.ShowAnnouncement(AnnouncementFalseCatch, g.playerIndex, true)
			}
			g.challengeWindow = 0
			g.challengeTargetID = ""
		}
	}
}

func (g *UnoGame) handlePlayerTurn() {
	player := g.state.CurrentPlayerObj()
	mx, my := ebiten.CursorPosition()
	scale := g.scale()
	cardW := g.cardWidthF()
	cardH := g.cardHeightF()
	offsetY := g.playAreaOffsetYF()

	discardX := int(float64(g.screenWidth/2) + 20*scale)
	discardY := int(float64(g.screenHeight/2) - cardH/2 + offsetY)
	cardWi := int(cardW)
	cardHi := int(cardH)

	if g.dragging {
		g.dragX = float64(mx) - cardW/2
		g.dragY = float64(my) - cardH/2
		g.needsRedraw = true // Redraw while dragging

		if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
			g.dragging = false

			dropMargin := int(30 * scale)
			if mx >= discardX-dropMargin && mx < discardX+cardWi+dropMargin &&
				my >= discardY-dropMargin && my < discardY+cardHi+dropMargin {

				card := player.Hand[g.dragCardIndex]

				if card.IsWild() {
					g.colorPicker = true
					g.pendingCard = g.dragCardIndex
					return
				}

				if err := g.state.PlayCard(player.ID, g.dragCardIndex, g.state.ChosenColor); err != nil {
					g.message = err.Error()
				} else {
					g.message = fmt.Sprintf("Played %s", card)
					g.challengeWindow = 0
					g.challengeTargetID = ""
				}
			}
			g.dragCardIndex = -1
		}
		return
	}

	if g.selectedCard >= 0 && g.selectedCard < len(player.Hand) {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.dragging = true
			g.dragCardIndex = g.selectedCard
			g.dragX = float64(mx) - cardW/2
			g.dragY = float64(my) - cardH/2
			g.needsRedraw = true
			return
		}
	}

	drawX := int(float64(g.screenWidth/2) - cardW - 20*scale)
	drawY := int(float64(g.screenHeight/2) - cardH/2 + offsetY)

	if mx >= drawX && mx < drawX+cardWi && my >= drawY && my < drawY+cardHi {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.challengeWindow = 0
			g.challengeTargetID = ""

			card, err := g.state.DrawCard(player.ID)
			if err != nil {
				g.message = err.Error()
			} else {
				g.message = fmt.Sprintf("Drew %s", card)
				g.SetPlayerAction(g.playerIndex, "Draw 1")

				if !card.CanPlayOn(g.state.CurrentCard(), g.state.ChosenColor) {
					g.state.PassTurn(player.ID)
				}
			}
		}
	}

	passX := g.screenWidth - int(120*scale)
	passY := g.screenHeight - int(60*scale)
	passBtnW := int(100 * scale)
	passBtnH := int(40 * scale)
	if mx >= passX && mx < passX+passBtnW && my >= passY && my < passY+passBtnH {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.state.PassTurn(player.ID)
			g.message = "Passed"
			g.SetPlayerAction(g.playerIndex, "Pass")
		}
	}
}

func (g *UnoGame) handleColorPicker() {
	mx, my := ebiten.CursorPosition()
	scale := g.scale()

	colors := []game.Color{game.ColorRed, game.ColorYellow, game.ColorGreen, game.ColorBlue}
	boxSize := int(60 * scale)
	gap := int(10 * scale)
	totalWidth := boxSize*4 + gap*3
	startX := g.screenWidth/2 - totalWidth/2
	startY := g.screenHeight/2 - boxSize/2

	for i, c := range colors {
		x := startX + i*(boxSize+gap)
		if mx >= x && mx < x+boxSize && my >= startY && my < startY+boxSize {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				player := g.state.CurrentPlayerObj()
				err := g.state.PlayCard(player.ID, g.pendingCard, c)
				if err != nil {
					g.message = err.Error()
				} else {
					g.message = fmt.Sprintf("Played Wild, chose %s", c)
				}
				// Trigger draw animations and update hand sizes immediately
				g.detectAndTriggerDrawAnimations()
				for j, p := range g.state.Players {
					if j < len(g.lastHandSizes) {
						g.lastHandSizes[j] = p.HandSize()
					}
				}
				g.colorPicker = false
				return
			}
		}
	}

	// Cancel with right click
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		g.colorPicker = false
	}
}
