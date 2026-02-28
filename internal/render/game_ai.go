package render

import (
	"fmt"
	"math/rand"

	"github.com/mr1hm/go-uno/internal/game"
)

func randFloat() float64 {
	return rand.Float64()
}

func (g *UnoGame) handleAITurn() {
	player := g.state.CurrentPlayerObj()

	// AI checks if any player forgot to call UNO (70% chance to notice)
	challenged := false
	if randFloat() < 0.7 {
		for _, p := range g.state.Players {
			if p.ID != player.ID && p.HandSize() == 1 && !p.HasCalledUno {
				if err := g.state.ChallengeUno(player.ID, p.ID); err == nil {
					g.message = fmt.Sprintf("%s caught %s! +2 cards", player.Name, p.Name)
					g.ShowAnnouncement(AnnouncementFalseCatch, g.state.CurrentPlayer, false)
					// Show caught popup
					g.caughtPopup = 120 // ~2 seconds at 60fps
					g.caughtPlayerName = p.Name
					g.caughtByName = player.Name
					challenged = true
					// Clear challenge window and return - challenge is the action for this turn
					g.challengeWindow = 0
					g.challengeTargetID = ""
					return
				}
				// Clear challenge window either way - AI took action
				g.challengeWindow = 0
				g.challengeTargetID = ""
				break
			}
		}
	}

	// Close challenge window if AI didn't challenge - they're taking their turn
	if !challenged {
		g.challengeWindow = 0
		g.challengeTargetID = ""
	}

	// Find playable cards
	playable := player.GetPlayableCards(g.state.CurrentCard(), g.state.ChosenColor)

	if len(playable) > 0 {
		// AI calls UNO if they have 2 cards (60% chance - sometimes forgets)
		if player.HandSize() == 2 && !player.HasCalledUno {
			if randFloat() < 0.6 {
				g.state.CallUno(player.ID)
				g.message = fmt.Sprintf("%s called UNO!", player.Name)
				g.ShowAnnouncement(AnnouncementUNO, g.state.CurrentPlayer, false)
			}
		}

		// Play first playable card
		cardIdx := playable[0]
		card := player.Hand[cardIdx]

		// Choose color for wild
		chosenColor := g.state.ChosenColor
		if card.IsWild() {
			chosenColor = g.pickAIColor(player)
		}

		g.state.PlayCard(player.ID, cardIdx, chosenColor)
		g.message = fmt.Sprintf("%s played %s", player.Name, card)
	} else {
		// Draw a card (animation triggered by hand size change detection)
		card, _ := g.state.DrawCard(player.ID)

		if card.CanPlayOn(g.state.CurrentCard(), g.state.ChosenColor) {
			chosenColor := g.state.ChosenColor
			if card.IsWild() {
				chosenColor = g.pickAIColor(player)
			}
			// Find the card we just drew (last in hand)
			g.state.PlayCard(player.ID, len(player.Hand)-1, chosenColor)
			g.message = fmt.Sprintf("%s drew and played %s", player.Name, card)
		} else {
			g.state.PassTurn(player.ID)
			g.message = fmt.Sprintf("%s drew and passed", player.Name)
		}
	}
}

func (g *UnoGame) pickAIColor(player *game.Player) game.Color {
	// Count colors in hand, pick most common
	counts := make(map[game.Color]int)
	for _, card := range player.Hand {
		if !card.IsWild() {
			counts[card.Color]++
		}
	}

	bestColor := game.ColorRed
	bestCount := 0
	for c, count := range counts {
		if count > bestCount {
			bestColor = c
			bestCount = count
		}
	}
	return bestColor
}
