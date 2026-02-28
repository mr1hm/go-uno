package render

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// drawAnimation represents a card being drawn and moving to a player
type drawAnimation struct {
	targetPlayer int     // Which player is receiving the card
	x, y         float64 // Current position
	targetX      float64 // Destination X
	targetY      float64 // Destination Y
	progress     float64 // 0 to 1 (negative = delayed start)
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
	remaining := make([]drawAnimation, 0, len(g.drawAnims))
	drawPileX := float64(g.screenWidth/2 - CardWidth - 20)
	drawPileY := float64(g.screenHeight/2 - CardHeight/2)

	for _, anim := range g.drawAnims {
		anim.progress += drawAnimSpeed
		if anim.progress >= 1 {
			continue // Animation complete
		}
		// Lerp position (only when progress > 0)
		if anim.progress > 0 {
			anim.x = drawPileX + (anim.targetX-drawPileX)*anim.progress
			anim.y = drawPileY + (anim.targetY-drawPileY)*anim.progress
		} else {
			anim.x = drawPileX
			anim.y = drawPileY
		}
		remaining = append(remaining, anim)
	}
	g.drawAnims = remaining
}

// startDrawAnimation starts a draw animation for a player
func (g *UnoGame) startDrawAnimation(playerIndex int) {
	g.startDrawAnimationWithDelay(playerIndex, 0)
}

// startDrawAnimationWithDelay starts a draw animation with optional delay (negative startProgress)
func (g *UnoGame) startDrawAnimationWithDelay(playerIndex int, startProgress float64) {
	var targetX, targetY float64

	if playerIndex == g.playerIndex {
		// Animate to player's hand
		player := g.state.Players[g.playerIndex]
		handY := g.screenHeight - CardHeight - 40
		totalWidth := len(player.Hand)*CardGap + CardWidth
		startX := (g.screenWidth - totalWidth) / 2
		targetX = float64(startX + (len(player.Hand)-1)*CardGap)
		targetY = float64(handY)
	} else {
		// Animate to opponent's position
		switch playerIndex {
		case 1:
			targetX, targetY = 50, float64(g.screenHeight/2)
		case 2:
			targetX, targetY = float64(g.screenWidth/2-50), 50
		case 3:
			targetX, targetY = float64(g.screenWidth-150), float64(g.screenHeight/2)
		}
	}

	drawPileX := float64(g.screenWidth/2 - CardWidth - 20)
	drawPileY := float64(g.screenHeight/2 - CardHeight/2)

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
			DrawCardBack(screen, anim.x, anim.y)
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
