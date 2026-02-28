package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mr1hm/go-uno/internal/game"
)

const (
	CardWidth        = 116
	CardHeight       = 168
	CardGap          = 40
	PlayAreaOffsetY  = 30 // Shift play area down to make room for direction arrow
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
	// UNO button state (one click per turn)
	unoClickedThisTurn bool
	lastCurrentPlayer  int
	// Draw animation state
	drawAnims []drawAnimation
	// Caught popup state
	caughtPopup       int    // Frames remaining to show popup
	caughtPlayerName  string // Name of player who got caught
	caughtByName      string // Name of player who caught them
	// Announcement state
	announcement      string  // Current announcement text
	announcementTimer int     // Frames remaining
	announcementFade  float64 // Current opacity (0-1)
	announcementPhase int     // 0=fade in, 1=hold, 2=fade out
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

	// Decrement caught popup
	if g.caughtPopup > 0 {
		g.caughtPopup--
	}

	// Reset UNO button when turn changes
	if g.state.CurrentPlayer != g.lastCurrentPlayer {
		g.unoClickedThisTurn = false
		g.lastCurrentPlayer = g.state.CurrentPlayer
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
			g.state = game.NewGame([]string{"You", "CPU 1", "CPU 2", "CPU 3"})
			g.message = ""
			g.lastHandSizes = nil
		}
		return nil
	}

	// Color picker active?
	if g.colorPicker {
		g.handleColorPicker()
		// Still update animations while color picker is shown
		g.updateDrawAnimations()
		return nil
	}

	// Always handle card hover (for lift animation)
	g.handleCardHover()

	// Handle UNO and Challenge buttons (can be clicked anytime)
	g.handleGlobalButtons()

	// Is it player's turn?
	if g.state.CurrentPlayer == g.playerIndex {
		g.aiDelay = 0
		g.handlePlayerTurn()
	} else {
		// AI turn with delay for readability
		if g.aiDelay < 90 { // ~1.5 seconds at 60fps
			g.aiDelay++
			// Don't process AI turn yet, but still update animations below
		} else {
			g.aiDelay = 0
			g.handleAITurn()
		}
	}

	// Check for UNO violations AFTER turns are processed
	for i, p := range g.state.Players {
		if i == g.playerIndex {
			continue
		}
		// Detect when player just went to 1 card without calling UNO
		if i < len(g.lastHandSizes) && g.lastHandSizes[i] > 1 && p.HandSize() == 1 && !p.HasCalledUno {
			g.challengeWindow = 180 // ~3 seconds to challenge at 60fps
			g.challengeTargetID = p.ID
		}
		// Also check if someone already has 1 card and no active challenge (for missed frames)
		if p.HandSize() == 1 && !p.HasCalledUno && g.challengeWindow == 0 && g.challengeTargetID == "" {
			g.challengeWindow = 180
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

	// Update announcement fade
	g.updateAnnouncement()

	return nil
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

	if g.caughtPopup > 0 {
		g.drawCaughtPopup(screen)
	}

	// Draw centered announcement with fade
	g.drawAnnouncement(screen)

	if g.state.GameOver {
		g.drawGameOver(screen)
	}
}

func (g *UnoGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}
