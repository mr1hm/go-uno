package render

import (
	"fmt"
	"image/color"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mr1hm/go-uno/internal/game"
)

func randFloat() float64 {
	return rand.Float64()
}

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
	aiDelay      int       // Frames to wait before AI acts
	cardLiftY    []float64 // Current lift offset for each card (for smooth animation)
	// Drag state
	dragging      bool
	dragCardIndex int
	dragX, dragY  float64
	// UNO challenge state
	challengeWindow   int    // Frames remaining to challenge
	challengeTargetID string // Player who can be challenged
	lastHandSizes     []int  // Track hand sizes to detect UNO violations
	// Draw animation state
	drawAnims []drawAnimation
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

	// Decrement challenge window
	if g.challengeWindow > 0 {
		g.challengeWindow--
		if g.challengeWindow == 0 {
			g.challengeTargetID = ""
		}
	}

	// Initialize hand size tracking
	if g.lastHandSizes == nil {
		g.lastHandSizes = make([]int, len(g.state.Players))
		for i, p := range g.state.Players {
			g.lastHandSizes[i] = p.HandSize()
		}
	}

	// Handle game over
	if g.state.GameOver {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Restart game
			g.state = game.NewGame([]string{"You", "CPU 1", "CPU 2"})
			g.message = ""
			g.lastHandSizes = nil
		}
		return nil
	}

	// Color picker active?
	if g.colorPicker {
		g.handleColorPicker()
		return nil
	}

	// Always handle card hover (for lift animation)
	g.handleCardHover()

	// Is it human's turn?
	if g.state.CurrentPlayer == g.playerIndex {
		g.aiDelay = 0
		g.handleHumanTurn()
	} else {
		// AI turn with delay for readability
		if g.aiDelay < 90 { // ~1.5 seconds at 60fps
			g.aiDelay++
			return nil
		}
		g.aiDelay = 0
		g.handleAITurn()
	}

	// Check for UNO violations AFTER turns are processed
	for i, p := range g.state.Players {
		if i < len(g.lastHandSizes) && g.lastHandSizes[i] > 1 && p.HandSize() == 1 && !p.HasCalledUno {
			// Player just went to 1 card without calling UNO - vulnerable!
			g.challengeWindow = 180 // ~3 seconds to challenge at 60fps
			g.challengeTargetID = p.ID
		}
	}

	// Detect and animate draws, then update hand sizes
	g.detectAndTriggerDrawAnimations()
	for i, p := range g.state.Players {
		if i < len(g.lastHandSizes) {
			g.lastHandSizes[i] = p.HandSize()
		}
	}

	// Update draw animations
	g.updateDrawAnimations()

	return nil
}

// handleCardHover detects which card the mouse is over (for lift animation)
func (g *UnoGame) handleCardHover() {
	player := g.state.Players[g.playerIndex]
	mx, my := ebiten.CursorPosition()

	handY := g.screenHeight - CardHeight - 40
	totalWidth := len(player.Hand)*CardGap + CardWidth
	startX := (g.screenWidth - totalWidth) / 2
	liftAmount := 30

	// Don't update hover during drag
	if g.dragging {
		return
	}

	g.selectedCard = -1
	for i := len(player.Hand) - 1; i >= 0; i-- {
		cardX := startX + i*CardGap
		if mx >= cardX && mx < cardX+CardWidth && my >= handY-liftAmount && my < handY+CardHeight {
			g.selectedCard = i
			break
		}
	}
}

func (g *UnoGame) handleHumanTurn() {
	player := g.state.CurrentPlayerObj()
	mx, my := ebiten.CursorPosition()

	// Discard pile position (drop target)
	discardX := g.screenWidth/2 + 20
	discardY := g.screenHeight/2 - CardHeight/2

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
	drawY := g.screenHeight/2 - CardHeight/2

	if mx >= drawX && mx < drawX+CardWidth && my >= drawY && my < drawY+CardHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
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

	// Check UNO button (bottom left) - always there, player must remember
	unoX := 20
	unoY := g.screenHeight - 60
	if mx >= unoX && mx < unoX+80 && my >= unoY && my < unoY+40 {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if err := g.state.CallUno(player.ID); err != nil {
				g.message = err.Error()
			} else {
				g.message = "Called UNO!"
			}
		}
	}

	// Check challenge button (appears briefly when opponent vulnerable)
	if g.challengeWindow > 0 && g.challengeTargetID != "" {
		chalX := g.screenWidth/2 - 60
		chalY := 60
		if mx >= chalX && mx < chalX+120 && my >= chalY && my < chalY+40 {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				if err := g.state.ChallengeUno(player.ID, g.challengeTargetID); err != nil {
					g.message = err.Error()
				} else {
					target := g.state.GetPlayerByID(g.challengeTargetID)
					g.message = fmt.Sprintf("Caught %s! +2 cards", target.Name)
				}
				g.challengeWindow = 0
				g.challengeTargetID = ""
			}
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
		// AI calls UNO if they have 2 cards (60% chance - sometimes forgets)
		if player.HandSize() == 2 && !player.HasCalledUno {
			if randFloat() < 0.6 {
				g.state.CallUno(player.ID)
				g.message = fmt.Sprintf("%s called UNO!", player.Name)
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
	g.drawDrawAnimations(screen)
	g.drawUI(screen)

	if g.colorPicker {
		g.drawColorPicker(screen)
	}

	if g.state.GameOver {
		g.drawGameOver(screen)
	}
}

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
	y := float64(g.screenHeight/2 - CardHeight/2)

	// Draw a few cards underneath with stagger for depth effect
	// Use deterministic offsets based on discard pile size
	pileSize := len(g.state.DiscardPile)
	staggerCards := min(pileSize-1, 4) // Show up to 4 cards underneath

	for i := range staggerCards {
		// Deterministic "random" offset based on card index
		seed := float64((pileSize - staggerCards + i) * 7)
		offsetX := (seed/10 - float64(int(seed/10))) * 8 - 4   // -4 to 4
		offsetY := (seed/13 - float64(int(seed/13))) * 6 - 3   // -3 to 3
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

func (g *UnoGame) drawDrawPile(screen *ebiten.Image) {
	x := float64(g.screenWidth/2 - CardWidth - 20)
	y := float64(g.screenHeight/2 - CardHeight/2)

	// Draw staggered cards for depth effect
	remaining := g.state.DrawPile.Remaining()
	staggerCards := min(remaining, 5) // Show up to 5 cards in stack

	for i := range staggerCards {
		// Deterministic offset based on index
		seed := float64(i * 17)
		offsetX := (seed/10 - float64(int(seed/10))) * 6 - 3   // -3 to 3
		offsetY := (seed/13 - float64(int(seed/13))) * 4 - 2   // -2 to 2
		rotation := (seed/19 - float64(int(seed/19))) * 0.12 - 0.06 // -0.06 to 0.06 radians

		DrawCardBackRotated(screen, x+offsetX, y+offsetY, rotation)
	}

	ebitenutil.DebugPrintAt(screen, "DRAW", int(x)+25, int(y)+CardHeight/2-8)
}

func (g *UnoGame) drawPlayerHand(screen *ebiten.Image) {
	player := g.state.Players[g.playerIndex]
	handY := g.screenHeight - CardHeight - 40
	totalWidth := len(player.Hand)*CardGap + CardWidth
	startX := (g.screenWidth - totalWidth) / 2
	isMyTurn := g.state.CurrentPlayer == g.playerIndex

	// Hide cards that are still animating (they're at the end of the hand)
	animatingCount := g.CountAnimatingCards(g.playerIndex)
	visibleCards := len(player.Hand) - animatingCount

	// Update card lift animations
	g.updateCardLiftAnimations(visibleCards)

	for i := range visibleCards {
		card := player.Hand[i]
		// Skip drawing card in hand if it's being dragged
		if g.dragging && i == g.dragCardIndex {
			continue
		}
		x := float64(startX + i*CardGap)
		y := float64(handY) - g.cardLiftY[i]
		DrawCard(screen, card, x, y, false)
	}

	// Draw dragged card on top (follows mouse)
	if g.dragging && g.dragCardIndex >= 0 && g.dragCardIndex < len(player.Hand) {
		DrawCard(screen, player.Hand[g.dragCardIndex], g.dragX, g.dragY, false)
	}

	// Player name and card count
	label := fmt.Sprintf("%s (%d cards)", player.Name, len(player.Hand))
	if isMyTurn {
		label = ">> " + label + " <<"
	}
	if player.HasCalledUno && player.HandSize() <= 2 {
		label += " - UNO!"
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

		// Draw card backs to represent hand (hide animating cards)
		animatingCount := g.CountAnimatingCards(i)
		visibleCards := player.HandSize() - animatingCount
		for j := range min(visibleCards, 7) {
			DrawCardBack(screen, float64(x+j*15), float64(y))
		}

		indicator := ""
		if g.state.CurrentPlayer == i {
			indicator = " <--"
		}
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s (%d)%s", player.Name, player.HandSize(), indicator), x, y-20)

		// Show UNO indicator if player has called UNO
		if player.HasCalledUno && player.HandSize() <= 2 {
			ebitenutil.DebugPrintAt(screen, "UNO!", x+50, y+CardHeight+10)
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

	// UNO button (cached)
	unoX := 20
	unoY := g.screenHeight - 60
	if unoButtonRed == nil {
		unoButtonRed = ebiten.NewImage(80, 40)
	}
	unoButtonRed.Fill(color.RGBA{200, 30, 30, 255})
	unoOp := &ebiten.DrawImageOptions{}
	unoOp.GeoM.Translate(float64(unoX), float64(unoY))
	screen.DrawImage(unoButtonRed, unoOp)
	ebitenutil.DebugPrintAt(screen, "UNO!", unoX+25, unoY+12)

	// Challenge button (cached)
	if g.challengeWindow > 0 && g.challengeTargetID != "" {
		target := g.state.GetPlayerByID(g.challengeTargetID)
		chalX := g.screenWidth/2 - 70
		chalY := 70
		if challengeBtn == nil {
			challengeBtn = ebiten.NewImage(140, 40)
		}
		challengeBtn.Fill(color.RGBA{255, 150, 0, 255})
		chalOp := &ebiten.DrawImageOptions{}
		chalOp.GeoM.Translate(float64(chalX), float64(chalY))
		screen.DrawImage(challengeBtn, chalOp)
		chalText := fmt.Sprintf("CATCH %s!", target.Name)
		ebitenutil.DebugPrintAt(screen, chalText, chalX+20, chalY+12)
	}
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
