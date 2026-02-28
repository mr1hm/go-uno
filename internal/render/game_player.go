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

	// Don't update hover during drag
	if g.dragging {
		return
	}

	// Fan parameters (must match drawPlayerHand)
	animatingCount := g.CountAnimatingCards(g.playerIndex)
	visibleCards := len(player.Hand) - animatingCount
	if visibleCards == 0 {
		g.selectedCard = -1
		return
	}

	arcRadius := 800.0
	centerX := float64(g.screenWidth) / 2
	centerY := float64(g.screenHeight) + arcRadius - CardHeight - 20 // Must match drawPlayerHand

	// Adjust fan angle based on number of cards (must match drawPlayerHand)
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

	// Find card closest to mouse (checking from top card to bottom for overlap priority)
	g.selectedCard = -1
	for i := visibleCards - 1; i >= 0; i-- {
		var angle float64
		if visibleCards == 1 {
			angle = 0
		} else {
			t := float64(i) / float64(visibleCards-1)
			angle = (t - 0.5) * actualFanAngle
		}

		// Calculate card center position
		cardCenterX := centerX + arcRadius*math.Sin(angle)
		cardCenterY := centerY - arcRadius*math.Cos(angle)

		// Check if mouse is within card bounds (rough rectangular check)
		dx := float64(mx) - cardCenterX
		dy := float64(my) - cardCenterY

		// Rotate mouse position to card's local space
		cos := math.Cos(-angle)
		sin := math.Sin(-angle)
		localX := dx*cos - dy*sin
		localY := dx*sin + dy*cos

		// Check if within card bounds
		if localX >= -CardWidth/2 && localX <= CardWidth/2 &&
			localY >= -CardHeight/2 && localY <= CardHeight/2+30 { // +30 for lift area
			g.selectedCard = i
			break
		}
	}
}

// handleGlobalButtons handles UNO and Challenge buttons (clickable anytime)
func (g *UnoGame) handleGlobalButtons() {
	mx, my := ebiten.CursorPosition()
	player := g.state.Players[g.playerIndex]

	// UNO button - next to discard pile
	discardX := g.screenWidth/2 + 20
	discardY := g.screenHeight/2 - CardHeight/2 + PlayAreaOffsetY
	unoX := discardX + CardWidth + 15
	unoY := discardY + CardHeight/2 - 20
	if mx >= unoX && mx < unoX+80 && my >= unoY && my < unoY+40 {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !g.unoClickedThisTurn && !player.HasCalledUno {
			g.unoClickedThisTurn = true
			// Always attempt to call UNO - game logic handles penalties
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

	// Challenge button - centered below player's hand
	buttonY := g.screenHeight - 45
	centerX := g.screenWidth / 2
	chalX := centerX - 50
	if mx >= chalX && mx < chalX+100 && my >= buttonY && my < buttonY+40 {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Find anyone with 1 card who hasn't called UNO
			var vulnerableTarget *game.Player
			for _, p := range g.state.Players {
				if p.ID != player.ID && p.HandSize() == 1 && !p.HasCalledUno {
					vulnerableTarget = p
					break
				}
			}

			if vulnerableTarget != nil {
				// Valid challenge - target draws 2
				if err := g.state.ChallengeUno(player.ID, vulnerableTarget.ID); err == nil {
					g.message = fmt.Sprintf("Caught %s! +2 cards", vulnerableTarget.Name)
					g.ShowAnnouncement(AnnouncementFalseCatch, g.playerIndex, false)
					g.caughtPopup = 120
					g.caughtPlayerName = vulnerableTarget.Name
					g.caughtByName = player.Name
				}
			} else {
				// False challenge - challenger draws 2
				g.state.PenalizePlayer(player.ID, 2)
				g.message = "False challenge! +2 cards"
				g.ShowAnnouncement(AnnouncementFalseCatch, g.playerIndex, true)
			}
			// Clear any active challenge window
			g.challengeWindow = 0
			g.challengeTargetID = ""
		}
	}
}

func (g *UnoGame) handlePlayerTurn() {
	player := g.state.CurrentPlayerObj()
	mx, my := ebiten.CursorPosition()

	// Discard pile position (drop target)
	discardX := g.screenWidth/2 + 20
	discardY := g.screenHeight/2 - CardHeight/2 + PlayAreaOffsetY

	// Handle dragging
	if g.dragging {
		g.dragX = float64(mx) - CardWidth/2
		g.dragY = float64(my) - CardHeight/2

		// Check for mouse release
		if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
			g.dragging = false

			// Check if dropped on discard pile
			if mx >= discardX-30 && mx < discardX+CardWidth+30 &&
				my >= discardY-30 && my < discardY+CardHeight+30 {

				card := player.Hand[g.dragCardIndex]

				// Wild card needs color picker
				if card.IsWild() {
					g.colorPicker = true
					g.pendingCard = g.dragCardIndex
					return
				}

				// Try to play the card
				if err := g.state.PlayCard(player.ID, g.dragCardIndex, g.state.ChosenColor); err != nil {
					g.message = err.Error()
				} else {
					g.message = fmt.Sprintf("Played %s", card)
					// Close challenge window - player took their turn
					g.challengeWindow = 0
					g.challengeTargetID = ""
				}
			}
			g.dragCardIndex = -1
		}
		return
	}

	// Check for drag start on selected card (hover already handled by handleCardHover)
	if g.selectedCard >= 0 && g.selectedCard < len(player.Hand) {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.dragging = true
			g.dragCardIndex = g.selectedCard
			g.dragX = float64(mx) - CardWidth/2
			g.dragY = float64(my) - CardHeight/2
			return
		}
	}

	// Check draw pile click
	drawX := g.screenWidth/2 - CardWidth - 20
	drawY := g.screenHeight/2 - CardHeight/2 + PlayAreaOffsetY

	if mx >= drawX && mx < drawX+CardWidth && my >= drawY && my < drawY+CardHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Close challenge window - player took their turn
			g.challengeWindow = 0
			g.challengeTargetID = ""

			card, err := g.state.DrawCard(player.ID)
			if err != nil {
				g.message = err.Error()
			} else {
				g.message = fmt.Sprintf("Drew %s", card)
				// Animation triggered by hand size change detection

				// Check if drawn card is playable, if not pass turn
				if !card.CanPlayOn(g.state.CurrentCard(), g.state.ChosenColor) {
					g.state.PassTurn(player.ID)
				}
			}
		}
	}

	// Check pass button (bottom right)
	passX := g.screenWidth - 120
	passY := g.screenHeight - 60
	if mx >= passX && mx < passX+100 && my >= passY && my < passY+40 {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.state.PassTurn(player.ID)
			g.message = "Passed"
		}
	}

	// UNO and Challenge buttons are handled in handleGlobalButtons()
}

func (g *UnoGame) handleColorPicker() {
	mx, my := ebiten.CursorPosition()
	colors := []game.Color{game.ColorRed, game.ColorYellow, game.ColorGreen, game.ColorBlue}
	boxSize := 60
	startX := g.screenWidth/2 - (boxSize*4+30)/2
	startY := g.screenHeight/2 - boxSize/2

	for i, c := range colors {
		x := startX + i*(boxSize+10)
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
