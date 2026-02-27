package render

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mr1hm/go-uno/internal/game"
)

const (
	CardWidth  = 80
	CardHeight = 120
	CardGap    = 30
)

type UnoGame struct {
	state        *game.GameState
	playerIndex  int // Which player is human (0)
	aiPlayers    []int
	selectedCard int
	message      string
	colorPicker  bool // Show color picker for wild cards
	pendingCard  int  // Card index waiting for color choice
	screenWidth  int
	screenHeight int
}

func NewUnoGame(playerNames []string) *UnoGame {
	g := &UnoGame{
		state:        game.NewGame(playerNames),
		playerIndex:  0,
		selectedCard: -1,
		screenWidth:  1280,
		screenHeight: 720,
	}

	// Mark AI Players (everyone except player 0)
	for i := 1; i < len(playerNames); i++ {
		g.aiPlayers = append(g.aiPlayers, i)
	}

	return g
}

func (g *UnoGame) Update() error {
	g.screenWidth, g.screenHeight = ebiten.WindowSize()

	// Handle game over
	if g.state.GameOver {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Restart game
			g.state = game.NewGame([]string{"You", "CPU 1", "CPU 2"})
			g.message = ""
		}
		return nil
	}

	// Color picker active?
	if g.colorPicker {
		g.handleColorPicker()
		return nil
	}

	// Is it human's turn?
	if g.state.CurrentPlayer == g.playerIndex {
		g.handleHumanTurn()
	} else {
		// AI turn
		g.handleAITurn()
	}

	return nil
}

func (g *UnoGame) handleHumanTurn() {
	player := g.state.CurrentPlayerObj()
	mx, my := ebiten.CursorPosition()

	// Calculate hand position
	handY := g.screenHeight - CardHeight - 40
	totalWidth := len(player.Hand)*CardGap + CardWidth
	startX := (g.screenWidth - totalWidth) / 2

	// Check card hover/click
	g.selectedCard = -1
	for i := range player.Hand {
		cardX := startX + i*CardGap
		if mx >= cardX && mx < cardX+CardWidth && my >= handY && my < handY+CardHeight {
			g.selectedCard = i

			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				card := player.Hand[i]

				// Wild card needs color picker
				if card.IsWild() {
					g.colorPicker = true
					g.pendingCard = i
					return
				}

				// Try to play the card
				if err := g.state.PlayCard(player.ID, i, g.state.ChosenColor); err != nil {
					g.message = err.Error()
				} else {
					g.message = fmt.Sprintf("Played %s", card)
				}
				return
			}
		}
	}

	// Check draw pile click
	drawX := g.screenWidth/2 - CardWidth - 20
	drawY := g.screenHeight/2 - CardHeight/2

	if mx >= drawX && mx < drawX+CardWidth && my >= drawY && my < drawY+CardHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			card, err := g.state.DrawCard(player.ID)
			if err != nil {
				g.message = err.Error()
			} else {
				g.message = fmt.Sprintf("Drew %s", card)

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

func (g *UnoGame) handleAITurn() {
	player := g.state.CurrentPlayerObj()

	// Find playable cards
	playable := player.GetPlayableCards(g.state.CurrentCard(), g.state.ChosenColor)

	if len(playable) > 0 {
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
		// Draw a card
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

func (g *UnoGame) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{34, 139, 34, 255}) // Green table

	g.drawDiscardPile(screen)
	g.drawDrawPile(screen)
	g.drawPlayerHand(screen)
	g.drawOpponents(screen)
	g.drawUI(screen)

	if g.colorPicker {
		g.drawColorPicker(screen)
	}

	if g.state.GameOver {
		g.drawGameOver(screen)
	}
}

func (g *UnoGame) drawDiscardPile(screen *ebiten.Image) {
	x := float64(g.screenWidth/2 + 20)
	y := float64(g.screenHeight/2 - CardHeight/2)
	DrawCard(screen, g.state.CurrentCard(), x, y, false)

	// Show chosen color indicator
	colorBox := ebiten.NewImage(20, 20)
	colorBox.Fill(getColorRGBA(g.state.ChosenColor))
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x+CardWidth/2-10, y-25)
	screen.DrawImage(colorBox, op)
}

func (g *UnoGame) drawDrawPile(screen *ebiten.Image) {
	x := float64(g.screenWidth/2 - CardWidth - 20)
	y := float64(g.screenHeight/2 - CardHeight/2)
	DrawCardBack(screen, x, y)
	ebitenutil.DebugPrintAt(screen, "DRAW", int(x)+25, int(y)+CardHeight/2-8)
}

func (g *UnoGame) drawPlayerHand(screen *ebiten.Image) {
	player := g.state.Players[g.playerIndex]
	handY := g.screenHeight - CardHeight - 40
	totalWidth := len(player.Hand)*CardGap + CardWidth
	startX := (g.screenWidth - totalWidth) / 2

	for i, card := range player.Hand {
		x := float64(startX + i*CardGap)
		y := float64(handY)
		if i == g.selectedCard {
			y -= 20 // Lift selected card
		}
		DrawCard(screen, card, x, y, g.state.CurrentPlayer == g.playerIndex)
	}

	// Player name and card count
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s (%d cards)", player.Name, len(player.Hand)), startX, handY-20)
}

func (g *UnoGame) drawOpponents(screen *ebiten.Image) {
	for i, player := range g.state.Players {
		if i == g.playerIndex {
			continue
		}

		// Position opponents around the table
		var x, y int
		switch i {
		case 1:
			x = 50
			y = g.screenHeight / 2
		case 2:
			x = g.screenWidth/2 - 50
			y = 50
		case 3:
			x = g.screenWidth - 150
			y = g.screenHeight / 2
		}

		// Draw card backs to represent hand
		for j := 0; j < min(player.HandSize(), 7); j++ {
			DrawCardBack(screen, float64(x+j*15), float64(y))
		}

		indicator := ""
		if g.state.CurrentPlayer == i {
			indicator = " <--"
		}
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s (%d)%s", player.Name, player.HandSize(), indicator), x, y-20)
	}
}

func (g *UnoGame) drawUI(screen *ebiten.Image) {
	// Turn indicator
	currentPlayer := g.state.CurrentPlayerObj()
	turnText := fmt.Sprintf("Turn: %s", currentPlayer.Name)
	if g.state.CurrentPlayer == g.playerIndex {
		turnText += " (Your turn!)"
	}
	ebitenutil.DebugPrintAt(screen, turnText, 10, 10)

	// Direction indicator
	dirText := "Direction: Clockwise"
	if g.state.Direction == game.DirectionCounterClockwise {
		dirText = "Direction: Counter-Clockwise"
	}
	ebitenutil.DebugPrintAt(screen, dirText, 10, 30)

	// Message
	if g.message != "" {
		ebitenutil.DebugPrintAt(screen, g.message, 10, 50)
	}

	// Pass button
	if g.state.CurrentPlayer == g.playerIndex {
		passX := g.screenWidth - 120
		passY := g.screenHeight - 60
		passBtn := ebiten.NewImage(100, 40)
		passBtn.Fill(color.RGBA{100, 100, 100, 255})
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(passX), float64(passY))
		screen.DrawImage(passBtn, op)
		ebitenutil.DebugPrintAt(screen, "PASS", passX+35, passY+12)
	}
}

func (g *UnoGame) drawColorPicker(screen *ebiten.Image) {
	// Darken background
	overlay := ebiten.NewImage(g.screenWidth, g.screenHeight)
	overlay.Fill(color.RGBA{0, 0, 0, 150})
	screen.DrawImage(overlay, nil)

	// Draw color boxes
	colors := []game.Color{game.ColorRed, game.ColorYellow, game.ColorGreen, game.ColorBlue}
	boxSize := 60
	startX := g.screenWidth/2 - (boxSize*4+30)/2
	startY := g.screenHeight/2 - boxSize/2

	ebitenutil.DebugPrintAt(screen, "Choose a color:", startX, startY-30)

	for i, c := range colors {
		x := startX + i*(boxSize+10)
		box := ebiten.NewImage(boxSize, boxSize)
		box.Fill(getColorRGBA(c))
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x), float64(startY))
		screen.DrawImage(box, op)
	}
}

func (g *UnoGame) drawGameOver(screen *ebiten.Image) {
	overlay := ebiten.NewImage(g.screenWidth, g.screenHeight)
	overlay.Fill(color.RGBA{0, 0, 0, 200})
	screen.DrawImage(overlay, nil)

	msg := fmt.Sprintf("%s WINS!", g.state.Winner.Name)
	ebitenutil.DebugPrintAt(screen, msg, g.screenWidth/2-50, g.screenHeight/2-20)
	ebitenutil.DebugPrintAt(screen, "Click to play again", g.screenWidth/2-70, g.screenHeight/2+10)
}

func (g *UnoGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
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
