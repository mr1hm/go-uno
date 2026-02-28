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
	CardWidth  = 116
	CardHeight = 168
	CardGap    = 40
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
	aiDelay      int // Frames to wait before AI acts
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
		g.aiDelay = 0
		g.handleHumanTurn()
	} else {
		// AI turn with delay for readability
		if g.aiDelay < 45 { // ~0.75 seconds at 60fps
			g.aiDelay++
			return nil
		}
		g.aiDelay = 0
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

	// Check card hover/click - iterate in REVERSE so topmost (rightmost) card is checked first
	liftAmount := 30
	g.selectedCard = -1
	for i := len(player.Hand) - 1; i >= 0; i-- {
		cardX := startX + i*CardGap
		// Hitbox extends upward by lift amount so clicked lifted cards register
		if mx >= cardX && mx < cardX+CardWidth && my >= handY-liftAmount && my < handY+CardHeight {
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
			break // Stop after first (topmost) match for hover
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
	// Draw background
	if bg := GetBackgroundSprite(); bg != nil {
		// Scale background to fit screen
		bgBounds := bg.Bounds()
		scaleX := float64(g.screenWidth) / float64(bgBounds.Dx())
		scaleY := float64(g.screenHeight) / float64(bgBounds.Dy())
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(scaleX, scaleY)
		screen.DrawImage(bg, op)
	} else {
		screen.Fill(color.RGBA{34, 139, 34, 255}) // Fallback green table
	}

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
	isMyTurn := g.state.CurrentPlayer == g.playerIndex

	// Draw subtle glow behind hand when it's your turn
	if isMyTurn {
		highlightPadding := 10
		highlight := ebiten.NewImage(totalWidth+highlightPadding*2, CardHeight+highlightPadding*2+30)
		highlight.Fill(color.RGBA{255, 255, 255, 25})
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(startX-highlightPadding), float64(handY-highlightPadding-30))
		screen.DrawImage(highlight, op)
	}

	for i, card := range player.Hand {
		x := float64(startX + i*CardGap)
		y := float64(handY)
		isSelected := i == g.selectedCard
		if isSelected {
			y -= 30 // Lift selected card more
		}
		DrawCard(screen, card, x, y, isMyTurn && isSelected)
	}

	// Player name and card count
	label := fmt.Sprintf("%s (%d cards)", player.Name, len(player.Hand))
	if isMyTurn {
		label = ">> " + label + " <<"
	}
	ebitenutil.DebugPrintAt(screen, label, startX, handY-25)
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
	// Turn indicator - prominent banner when it's your turn
	currentPlayer := g.state.CurrentPlayerObj()
	if g.state.CurrentPlayer == g.playerIndex {
		// Draw "YOUR TURN" banner
		bannerHeight := 40
		banner := ebiten.NewImage(g.screenWidth, bannerHeight)
		banner.Fill(color.RGBA{0, 100, 0, 200})
		op := &ebiten.DrawImageOptions{}
		screen.DrawImage(banner, op)
		ebitenutil.DebugPrintAt(screen, ">>> YOUR TURN - Click a card to play! <<<", g.screenWidth/2-150, 12)
	} else {
		turnText := fmt.Sprintf("Waiting for %s...", currentPlayer.Name)
		ebitenutil.DebugPrintAt(screen, turnText, 10, 10)
	}

	// Direction indicator
	dirText := "Direction: Clockwise"
	if g.state.Direction == game.DirectionCounterClockwise {
		dirText = "Direction: Counter-Clockwise"
	}
	ebitenutil.DebugPrintAt(screen, dirText, 10, 30)

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
