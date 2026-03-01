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
	needsRedraw  bool // Only redraw when this is true
	// Multiplayer
	network      *NetworkClient
	lobbyPlayers []string // Players in lobby
	selectedCard int
	message      string
	colorPicker  bool // Show color picker for wild cards
	pendingCard  int  // Card index waiting for color choice
	screenWidth  int
	screenHeight int
	aiDelay   int       // Frames to wait before AI acts
	cardLiftY []float64 // Current lift offset for each card (for smooth animation)
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

// Cached scale values - updated once per frame in Layout()
var (
	cachedScale        float64 = 1.0
	cachedCardWidth    float64 = baseCardWidth
	cachedCardHeight   float64 = baseCardHeight
	cachedCardGap      float64 = baseCardGap
	cachedPlayAreaOffsetY float64 = basePlayAreaOffsetY
)

// updateScaleCache recalculates cached scale values (called once per frame in Layout)
func (g *UnoGame) updateScaleCache() {
	scaleX := float64(g.screenWidth) / baseWidth
	scaleY := float64(g.screenHeight) / baseHeight
	if scaleX < scaleY {
		cachedScale = scaleX
	} else {
		cachedScale = scaleY
	}
	cachedCardWidth = baseCardWidth * cachedScale
	cachedCardHeight = baseCardHeight * cachedScale
	cachedCardGap = baseCardGap * cachedScale
	cachedPlayAreaOffsetY = basePlayAreaOffsetY * cachedScale
}

// scale returns the cached scale factor
func (g *UnoGame) scale() float64 { return cachedScale }

// Scaled dimension helpers (float for precise positioning)
func (g *UnoGame) cardWidthF() float64   { return cachedCardWidth }
func (g *UnoGame) cardHeightF() float64  { return cachedCardHeight }
func (g *UnoGame) cardGapF() float64     { return cachedCardGap }
func (g *UnoGame) playAreaOffsetYF() float64 { return cachedPlayAreaOffsetY }

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
		g.needsRedraw = true
		lastFrameValid = false
		baseFrameValid = false

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
	prevSelectedCard := g.selectedCard

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

	// Detect game state changes that require redraw
	discardSize := len(g.state.DiscardPile)
	if g.state.CurrentPlayer != cachedCurrentPlayer || discardSize != cachedDiscardSize {
		baseFrameValid = false
		g.needsRedraw = true
		cachedCurrentPlayer = g.state.CurrentPlayer
		cachedDiscardSize = discardSize
	}

	// Mark redraw needed if selected card changed (hover effect) or mouse clicked
	if g.selectedCard != prevSelectedCard ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		g.needsRedraw = true
	}

	return nil
}

// Cached scaled background
var (
	scaledBackground     *ebiten.Image
	lastBackgroundWidth  int
	lastBackgroundHeight int
)

// Skip-draw optimization state
var lastFrameValid bool

// Cached base frame (everything except player hand - changes rarely)
var baseFrame *ebiten.Image
var baseFrameValid bool

// Track state for cache invalidation
var cachedCurrentPlayer int = -1
var cachedDiscardSize int = -1

// hasActiveAnimations returns true if any animations are currently running
func (g *UnoGame) hasActiveAnimations() bool {
	return len(g.drawAnims) > 0 || len(g.playAnims) > 0 ||
		g.announcementTimer > 0 || g.caughtPopup > 0
}

// isHoverAnimationComplete returns true if hover animation is done (can skip redraw)
func (g *UnoGame) isHoverAnimationComplete() bool {
	// Check ALL cards are at their target position
	for i, lift := range g.cardLiftY {
		if i == g.selectedCard {
			// Selected card should be fully lifted
			if lift < cardLiftTarget {
				return false
			}
		} else {
			// Other cards should be fully down
			if lift > 0 {
				return false
			}
		}
	}
	return true
}

func (g *UnoGame) Draw(screen *ebiten.Image) {
	// Check if we can skip drawing entirely
	// Game over screen is static - always skip after first draw
	if g.state != nil && g.state.GameOver && lastFrameValid && !g.needsRedraw {
		return
	}
	if lastFrameValid && !g.needsRedraw && !g.hasActiveAnimations() &&
		!g.dragging && g.isHoverAnimationComplete() {
		return
	}

	// Draw menu
	if g.mode == ModeMenu {
		g.drawMenu(screen)
		lastFrameValid = true
		g.needsRedraw = false
		return
	}

	// Draw lobby
	if g.mode == ModeLobby {
		g.drawLobby(screen)
		lastFrameValid = true
		g.needsRedraw = false
		return
	}

	// Create/resize base frame cache if needed
	if baseFrame == nil || baseFrame.Bounds().Dx() != g.screenWidth {
		baseFrame = ebiten.NewImage(g.screenWidth, g.screenHeight)
		baseFrameValid = false
	}

	// Regenerate base frame only when needed (expensive elements that rarely change)
	if !baseFrameValid {
		baseFrame.Clear()

		// Draw background
		if bg := GetBackgroundSprite(); bg != nil {
			if scaledBackground == nil || lastBackgroundWidth != g.screenWidth || lastBackgroundHeight != g.screenHeight {
				scaledBackground = ebiten.NewImage(g.screenWidth, g.screenHeight)
				bgBounds := bg.Bounds()
				scaleX := float64(g.screenWidth) / float64(bgBounds.Dx())
				scaleY := float64(g.screenHeight) / float64(bgBounds.Dy())
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(scaleX, scaleY)
				scaledBackground.DrawImage(bg, op)
				lastBackgroundWidth = g.screenWidth
				lastBackgroundHeight = g.screenHeight
			}
			baseFrame.DrawImage(scaledBackground, nil)
		} else {
			baseFrame.Fill(color.RGBA{34, 139, 34, 255})
		}

		g.drawDiscardPile(baseFrame)
		g.drawDrawPile(baseFrame)
		g.drawOpponents(baseFrame)
		g.drawUI(baseFrame)
		baseFrameValid = true
	}

	// Draw directly to screen (no intermediate buffer)
	screen.DrawImage(baseFrame, nil)

	// Draw dynamic elements
	g.drawPlayerHand(screen)
	g.drawDrawAnimations(screen)
	g.drawPlayAnimations(screen)

	if g.colorPicker {
		g.drawColorPicker(screen)
	}

	if g.caughtPopup > 0 {
		g.drawCaughtPopup(screen)
	}

	g.drawAnnouncement(screen)

	if g.state.GameOver {
		g.drawGameOver(screen)
	}

	// Draw dragged card on top if dragging
	if g.dragging {
		g.drawDraggedCard(screen)
	}

	// Cache state for skip optimization
	lastFrameValid = true
	g.needsRedraw = false
}

// drawDraggedCard draws only the card being dragged
func (g *UnoGame) drawDraggedCard(screen *ebiten.Image) {
	if !g.dragging || g.dragCardIndex < 0 {
		return
	}
	player := g.state.Players[g.playerIndex]
	if g.dragCardIndex >= len(player.Hand) {
		return
	}
	card := player.Hand[g.dragCardIndex]
	g.drawCardScaled(screen, card, g.dragX, g.dragY)
}

func (g *UnoGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	// Render at fixed internal resolution for performance
	// GPU will scale to actual window size
	g.screenWidth = baseWidth
	g.screenHeight = baseHeight
	g.updateScaleCache()
	return baseWidth, baseHeight
}
