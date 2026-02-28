package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mr1hm/go-uno/internal/game"
)

// Base dimensions designed for 1280x720
const (
	baseWidth       = 1280
	baseHeight      = 720
	baseCardWidth   = 116
	baseCardHeight  = 168
	baseCardGap     = 40
	basePlayAreaOffsetY = 30
)

// Legacy constants for compatibility - will be replaced by scaled methods
const (
	CardWidth       = baseCardWidth
	CardHeight      = baseCardHeight
	CardGap         = baseCardGap
	PlayAreaOffsetY = basePlayAreaOffsetY
)

type UnoGame struct {
	mode         GameMode // Menu, Lobby, AI, or Players
	state        *game.GameState
	playerIndex  int // Which player is human (0)
	aiPlayers    []int
	// Multiplayer
	network      *NetworkClient
	lobbyPlayers []string // Players in lobby
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
	// Play animation state (card moving to discard pile)
	playAnims []playAnimation
	// Player action text (shown above their cards)
	playerActions      []string // Action text per player
	playerActionTimers []int    // Frames remaining to show action
	// Caught popup state
	caughtPopup       int    // Frames remaining to show popup
	caughtPlayerName  string // Name of player who got caught
	caughtByName      string // Name of player who caught them
	// Announcement state
	announcement        string  // Current announcement text
	announcementTimer   int     // Frames remaining
	announcementFade    float64 // Current opacity (0-1)
	announcementPhase   int     // 0=fade in, 1=hold, 2=fade out
	announcementPlayer  int     // Index of player who triggered announcement (-1 for self/current viewer)
	announcementFailure bool    // True if this is a failure/penalty announcement
}

func NewUnoGame(playerNames []string) *UnoGame {
	g := &UnoGame{
		mode:         ModeMenu,
		playerIndex:  0,
		selectedCard: -1,
		screenWidth:  1280,
		screenHeight: 720,
	}
	return g
}

// scale returns the current scale factor based on screen size
func (g *UnoGame) scale() float64 {
	scaleX := float64(g.screenWidth) / baseWidth
	scaleY := float64(g.screenHeight) / baseHeight
	if scaleX < scaleY {
		return scaleX
	}
	return scaleY
}

// Scaled dimension helpers
func (g *UnoGame) cardWidth() int   { return int(baseCardWidth * g.scale()) }
func (g *UnoGame) cardHeight() int  { return int(baseCardHeight * g.scale()) }
func (g *UnoGame) cardGap() int     { return int(baseCardGap * g.scale()) }
func (g *UnoGame) playAreaOffsetY() int { return int(basePlayAreaOffsetY * g.scale()) }

// Float versions for precise positioning
func (g *UnoGame) cardWidthF() float64   { return baseCardWidth * g.scale() }
func (g *UnoGame) cardHeightF() float64  { return baseCardHeight * g.scale() }
func (g *UnoGame) cardGapF() float64     { return baseCardGap * g.scale() }
func (g *UnoGame) playAreaOffsetYF() float64 { return basePlayAreaOffsetY * g.scale() }

func (g *UnoGame) startGame(mode GameMode) {
	if mode == ModeAI {
		g.mode = ModeAI
		playerNames := []string{"You", "CPU 1", "CPU 2", "CPU 3"}
		g.state = game.NewGame(playerNames)
		g.selectedCard = -1
		g.aiPlayers = nil
		g.lastHandSizes = nil
		g.drawAnims = nil
		g.playAnims = nil
		g.playerActions = nil
		g.playerActionTimers = nil

		for i := 1; i < len(playerNames); i++ {
			g.aiPlayers = append(g.aiPlayers, i)
		}
	} else {
		// Multiplayer - go to lobby
		g.mode = ModeLobby
		g.network = NewNetworkClient()

		// Get Discord state for room ID and player name
		discord := GetDiscordState()
		roomID := discord.InstanceID // All players in same Activity share instanceId
		playerName := discord.Username
		if playerName == "" {
			playerName = "Player"
		}

		g.lobbyPlayers = []string{playerName}

		// Connect to server
		wsURL := "ws://" + getHost() + "/ws"
		if discord.UserID != "" {
			wsURL += "?id=" + discord.UserID // Use Discord user ID as player ID
		}
		g.network.Connect(wsURL)
		g.network.JoinRoom(roomID, playerName)
	}
}

func (g *UnoGame) returnToMenu() {
	g.mode = ModeMenu
	if g.network != nil {
		g.network.Close()
		g.network = nil
	}
	g.state = nil
	g.lobbyPlayers = nil
}

func (g *UnoGame) Update() error {
	// Screen size is set in Layout()

	// Handle menu
	if g.mode == ModeMenu {
		g.updateMenu()
		return nil
	}

	// Handle lobby
	if g.mode == ModeLobby {
		g.updateLobby()
		return nil
	}

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

	// Update animations
	g.updateDrawAnimations()
	g.updatePlayAnimations()
	g.updatePlayerActions()

	// Update announcement fade
	g.updateAnnouncement()

	return nil
}

func (g *UnoGame) Draw(screen *ebiten.Image) {
	// Draw menu
	if g.mode == ModeMenu {
		g.drawMenu(screen)
		return
	}

	// Draw lobby
	if g.mode == ModeLobby {
		g.drawLobby(screen)
		return
	}

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
	g.drawPlayAnimations(screen)
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
	g.screenWidth = outsideWidth
	g.screenHeight = outsideHeight
	return outsideWidth, outsideHeight
}
