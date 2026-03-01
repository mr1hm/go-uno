package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mr1hm/go-uno/internal/game"
)

// drawAnimation represents a card being drawn and moving to a player
type drawAnimation struct {
	targetPlayer int     // Which player is receiving the card
	x, y         float64 // Current position
	targetX      float64 // Destination X
	targetY      float64 // Destination Y
	progress     float64 // 0 to 1 (negative = delayed start)
}

// playAnimation represents a card being played and moving to the discard pile
type playAnimation struct {
	fromPlayer int         // Which player played the card
	card       game.Card   // The card being played
	x, y       float64     // Current position
	startX     float64     // Starting X
	startY     float64     // Starting Y
	targetX    float64     // Destination X (discard pile)
	targetY    float64     // Destination Y (discard pile)
	progress   float64     // 0 to 1
	rotation   float64     // Current rotation
}

// CountAnimatingCards returns how many cards are currently animating to a player
func (g *UnoGame) CountAnimatingCards(playerIndex int) int {
	count := 0
	for _, anim := range g.drawAnims {
		if anim.targetPlayer == playerIndex {
			count++
		}
	}
	return count
}

const (
	drawAnimSpeed    = 0.08
	drawAnimDelay    = -1.1 // Delay between cards (negative = wait for previous to finish)
	maxDrawAnims     = 8
	cardLiftSpeed    = 4.0
	cardLiftTarget   = 30.0
)

// updateDrawAnimations advances all draw animations and removes completed ones
func (g *UnoGame) updateDrawAnimations() {
	if len(g.drawAnims) == 0 {
		return
	}

	scale := g.scale()
	cardW := g.cardWidthF()
	cardH := g.cardHeightF()
	offsetY := g.playAreaOffsetYF()

	drawPileX := float64(g.screenWidth/2) - cardW - 20*scale
	drawPileY := float64(g.screenHeight/2) - cardH/2 + offsetY

	// Filter in place to avoid allocation
	writeIdx := 0
	for i := range g.drawAnims {
		g.drawAnims[i].progress += drawAnimSpeed
		if g.drawAnims[i].progress >= 1 {
			continue // Animation complete, skip it
		}
		// Lerp position (only when progress > 0)
		if g.drawAnims[i].progress > 0 {
			g.drawAnims[i].x = drawPileX + (g.drawAnims[i].targetX-drawPileX)*g.drawAnims[i].progress
			g.drawAnims[i].y = drawPileY + (g.drawAnims[i].targetY-drawPileY)*g.drawAnims[i].progress
		} else {
			g.drawAnims[i].x = drawPileX
			g.drawAnims[i].y = drawPileY
		}
		g.drawAnims[writeIdx] = g.drawAnims[i]
		writeIdx++
	}
	g.drawAnims = g.drawAnims[:writeIdx]
}

// startDrawAnimation starts a draw animation for a player
func (g *UnoGame) startDrawAnimation(playerIndex int) {
	g.startDrawAnimationWithDelay(playerIndex, 0)
}

// startDrawAnimationWithDelay starts a draw animation with optional delay (negative startProgress)
func (g *UnoGame) startDrawAnimationWithDelay(playerIndex int, startProgress float64) {
	var targetX, targetY float64
	scale := g.scale()
	cardW := g.cardWidthF()
	cardH := g.cardHeightF()
	cardGap := g.cardGapF()
	offsetY := g.playAreaOffsetYF()

	if playerIndex == g.playerIndex {
		// Animate to player's hand
		player := g.state.Players[g.playerIndex]
		handY := float64(g.screenHeight) - cardH - 40*scale
		totalWidth := float64(len(player.Hand))*cardGap + cardW
		startX := (float64(g.screenWidth) - totalWidth) / 2
		targetX = startX + float64(len(player.Hand)-1)*cardGap
		targetY = handY
	} else {
		// Animate to opponent's position
		switch playerIndex {
		case 1:
			targetX, targetY = 50*scale, float64(g.screenHeight/2)
		case 2:
			targetX, targetY = float64(g.screenWidth/2)-50*scale, 50*scale
		case 3:
			targetX, targetY = float64(g.screenWidth)-150*scale, float64(g.screenHeight/2)
		}
	}

	drawPileX := float64(g.screenWidth/2) - cardW - 20*scale
	drawPileY := float64(g.screenHeight/2) - cardH/2 + offsetY

	g.drawAnims = append(g.drawAnims, drawAnimation{
		targetPlayer: playerIndex,
		x:            drawPileX,
		y:            drawPileY,
		targetX:      targetX,
		targetY:      targetY,
		progress:     startProgress,
	})
}

// drawDrawAnimations renders all active draw animations
func (g *UnoGame) drawDrawAnimations(screen *ebiten.Image) {
	for _, anim := range g.drawAnims {
		if anim.progress > 0 { // Only draw when animation has started
			g.drawCardBackScaled(screen, anim.x, anim.y)
		}
	}
}

// updateCardLiftAnimations updates the Y offset for card hover lift effect
func (g *UnoGame) updateCardLiftAnimations(handSize int) {
	// Ensure cardLiftY slice matches hand size
	for len(g.cardLiftY) < handSize {
		g.cardLiftY = append(g.cardLiftY, 0)
	}
	if len(g.cardLiftY) > handSize {
		g.cardLiftY = g.cardLiftY[:handSize]
	}

	// Animate card lift
	for i := range handSize {
		if i == g.selectedCard {
			// Lift up
			if g.cardLiftY[i] < cardLiftTarget {
				g.cardLiftY[i] += cardLiftSpeed
				if g.cardLiftY[i] > cardLiftTarget {
					g.cardLiftY[i] = cardLiftTarget
				}
			}
		} else {
			// Lower down
			if g.cardLiftY[i] > 0 {
				g.cardLiftY[i] -= cardLiftSpeed
				if g.cardLiftY[i] < 0 {
					g.cardLiftY[i] = 0
				}
			}
		}
	}
}

// detectAndTriggerDrawAnimations checks for hand size changes and triggers animations
func (g *UnoGame) detectAndTriggerDrawAnimations() {
	for i, p := range g.state.Players {
		if i < len(g.lastHandSizes) && len(g.drawAnims) < maxDrawAnims {
			diff := p.HandSize() - g.lastHandSizes[i]
			for j := 0; j < diff && len(g.drawAnims) < maxDrawAnims; j++ {
				g.startDrawAnimationWithDelay(i, float64(j)*drawAnimDelay)
			}
		}
	}
}

const (
	playAnimSpeed     = 0.12
	actionTextFrames  = 90 // ~1.5 seconds at 60fps
)

// startPlayAnimation starts a card play animation from player to discard pile
func (g *UnoGame) startPlayAnimation(playerIndex int, card game.Card) {
	var startX, startY float64
	scale := g.scale()
	cardH := g.cardHeightF()
	offsetY := g.playAreaOffsetYF()

	// Get start position based on player
	if playerIndex == g.playerIndex {
		startX = float64(g.screenWidth) / 2
		startY = float64(g.screenHeight) - cardH - 40*scale
	} else {
		switch playerIndex {
		case 1: // Left
			startX = 120 * scale
			startY = float64(g.screenHeight)/2 + offsetY
		case 2: // Top
			startX = float64(g.screenWidth) / 2
			startY = 100 * scale
		case 3: // Right
			startX = float64(g.screenWidth) - 120*scale
			startY = float64(g.screenHeight)/2 + offsetY
		}
	}

	// Target is the discard pile
	targetX := float64(g.screenWidth/2) + 20*scale
	targetY := float64(g.screenHeight/2) - cardH/2 + offsetY

	g.playAnims = append(g.playAnims, playAnimation{
		fromPlayer: playerIndex,
		card:       card,
		x:          startX,
		y:          startY,
		startX:     startX,
		startY:     startY,
		targetX:    targetX,
		targetY:    targetY,
		progress:   0,
		rotation:   0,
	})
}

// updatePlayAnimations advances all play animations
func (g *UnoGame) updatePlayAnimations() {
	if len(g.playAnims) == 0 {
		return
	}

	// Filter in place to avoid allocation
	writeIdx := 0
	for i := range g.playAnims {
		g.playAnims[i].progress += playAnimSpeed
		if g.playAnims[i].progress >= 1 {
			continue // Animation complete, skip it
		}
		// Lerp position
		g.playAnims[i].x = g.playAnims[i].startX + (g.playAnims[i].targetX-g.playAnims[i].startX)*g.playAnims[i].progress
		g.playAnims[i].y = g.playAnims[i].startY + (g.playAnims[i].targetY-g.playAnims[i].startY)*g.playAnims[i].progress
		g.playAnims[writeIdx] = g.playAnims[i]
		writeIdx++
	}
	g.playAnims = g.playAnims[:writeIdx]
}

// drawPlayAnimations renders all active play animations
func (g *UnoGame) drawPlayAnimations(screen *ebiten.Image) {
	for _, anim := range g.playAnims {
		g.drawCardScaled(screen, anim.card, anim.x, anim.y, false)
	}
}

// SetPlayerAction sets action text for a player (Pass, Draw 1, etc.)
func (g *UnoGame) SetPlayerAction(playerIndex int, action string) {
	// Initialize slices if needed
	for len(g.playerActions) <= playerIndex {
		g.playerActions = append(g.playerActions, "")
	}
	for len(g.playerActionTimers) <= playerIndex {
		g.playerActionTimers = append(g.playerActionTimers, 0)
	}
	g.playerActions[playerIndex] = action
	g.playerActionTimers[playerIndex] = actionTextFrames
}

// updatePlayerActions decrements action timers
func (g *UnoGame) updatePlayerActions() {
	for i := range g.playerActionTimers {
		if g.playerActionTimers[i] > 0 {
			g.playerActionTimers[i]--
		}
	}
}

// GetPlayerAction returns the current action text for a player (empty if expired)
func (g *UnoGame) GetPlayerAction(playerIndex int) string {
	if playerIndex >= len(g.playerActions) || playerIndex >= len(g.playerActionTimers) {
		return ""
	}
	if g.playerActionTimers[playerIndex] <= 0 {
		return ""
	}
	return g.playerActions[playerIndex]
}
