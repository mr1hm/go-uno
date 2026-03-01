package render

import (
	"fmt"
	"image/color"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/mr1hm/go-uno/internal/server"
)

type GameMode int

const (
	ModeMenu GameMode = iota
	ModeLobby // Waiting for players
	ModeAI
	ModePlayers
)

const (
	menuButtonWidth  = 300
	menuButtonHeight = 60
	menuButtonGap    = 20
)

type MenuButton struct {
	Label string
	Mode  GameMode
}

// Cached button images to avoid per-frame allocations
// Key format: "label:width:height:hovered"
var buttonCache = make(map[string]*ebiten.Image)

// Shared 1x1 pixel for drawing borders efficiently
var borderPixel *ebiten.Image

var menuButtons = []MenuButton{
	{"New Game (AI)", ModeAI},
	{"New Game (Multiplayer)", ModePlayers},
}

var lastHoveredButton = -1 // Track hover changes for skip-draw

func (g *UnoGame) updateMenu() {
	mx, my := ebiten.CursorPosition()

	// Calculate button dimensions (scale for small screens)
	btnWidth := menuButtonWidth
	btnHeight := menuButtonHeight
	btnGap := menuButtonGap
	if g.screenWidth < 600 {
		btnWidth = g.screenWidth - 40
		btnHeight = 50
		btnGap = 15
	}

	// Calculate layout (must match drawMenu)
	totalBtnHeight := len(menuButtons)*btnHeight + (len(menuButtons)-1)*btnGap
	titleHeight := 100
	titleGap := 40
	totalContentHeight := titleHeight + titleGap + totalBtnHeight

	contentStartY := (g.screenHeight - totalContentHeight) / 2
	if contentStartY < 20 {
		contentStartY = 20
	}
	startY := contentStartY + titleHeight + titleGap

	// Track which button is hovered
	hoveredButton := -1
	for i, btn := range menuButtons {
		btnX := (g.screenWidth - btnWidth) / 2
		btnY := startY + i*(btnHeight+btnGap)

		if mx >= btnX && mx < btnX+btnWidth &&
			my >= btnY && my < btnY+btnHeight {
			hoveredButton = i
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				g.startGame(btn.Mode)
			}
		}
	}

	// Only redraw if hover state changed
	if hoveredButton != lastHoveredButton {
		lastHoveredButton = hoveredButton
		g.needsRedraw = true
	}
}

func (g *UnoGame) drawMenu(screen *ebiten.Image) {
	// Dark background
	screen.Fill(color.RGBA{20, 20, 30, 255})

	// Calculate button dimensions (scale for small screens)
	btnWidth := menuButtonWidth
	btnHeight := menuButtonHeight
	btnGap := menuButtonGap
	if g.screenWidth < 600 {
		btnWidth = g.screenWidth - 40
		btnHeight = 50
		btnGap = 15
	}

	// Calculate total content height (title + gap + buttons)
	totalBtnHeight := len(menuButtons)*btnHeight + (len(menuButtons)-1)*btnGap
	titleHeight := 100 // approximate title height
	titleGap := 40
	totalContentHeight := titleHeight + titleGap + totalBtnHeight

	// Start Y so content is vertically centered
	contentStartY := (g.screenHeight - totalContentHeight) / 2
	if contentStartY < 20 {
		contentStartY = 20
	}

	// Title
	title := "UNO"
	titleFace := getMenuTitleFace()
	if titleFace != nil {
		tw, th := text.Measure(title, titleFace, 0)
		titleX := (float64(g.screenWidth) - tw) / 2
		titleY := float64(contentStartY)

		// Draw title with gold color
		op := &text.DrawOptions{}
		op.GeoM.Translate(titleX, titleY)
		op.ColorScale.ScaleWithColor(color.RGBA{255, 215, 0, 255})
		text.Draw(screen, title, titleFace, op)

		titleHeight = int(th)
	}

	// Buttons (positioned below title)
	mx, my := ebiten.CursorPosition()
	startY := contentStartY + titleHeight + titleGap

	for i, btn := range menuButtons {
		btnX := (g.screenWidth - btnWidth) / 2
		btnY := startY + i*(btnHeight+btnGap)

		// Check hover
		hovered := mx >= btnX && mx < btnX+btnWidth &&
			my >= btnY && my < btnY+btnHeight

		drawMenuButton(screen, btnX, btnY, btnWidth, btnHeight, btn.Label, hovered)
	}
}

func drawMenuButton(screen *ebiten.Image, x, y, w, h int, label string, hovered bool) {
	// Build cache key including dimensions and hover state
	hoverKey := "0"
	if hovered {
		hoverKey = "1"
	}
	cacheKey := fmt.Sprintf("%s:%d:%d:%s", label, w, h, hoverKey)

	// Check cache
	btnImg, exists := buttonCache[cacheKey]
	if !exists {
		// Button background
		var bgColor color.RGBA
		if hovered {
			bgColor = color.RGBA{80, 80, 100, 255}
		} else {
			bgColor = color.RGBA{50, 50, 70, 255}
		}

		// Border color
		borderColor := color.RGBA{100, 100, 130, 255}
		if hovered {
			borderColor = color.RGBA{255, 215, 0, 255} // Gold on hover
		}

		// Create and cache button image
		btnImg = ebiten.NewImage(w, h)
		btnImg.Fill(bgColor)
		drawRectBorder(btnImg, w, h, borderColor)
		buttonCache[cacheKey] = btnImg
	}

	// Draw cached button
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(btnImg, op)

	// Button text
	face := getMenuButtonFace()
	if face != nil {
		tw, th := text.Measure(label, face, 0)
		textX := float64(x) + (float64(w)-tw)/2
		textY := float64(y) + (float64(h)-th)/2

		textOp := &text.DrawOptions{}
		textOp.GeoM.Translate(textX, textY)
		textOp.ColorScale.ScaleWithColor(color.White)
		text.Draw(screen, label, face, textOp)
	}
}

// drawRectBorder draws a 1px border using scaled pixel draws (much faster than Set per pixel)
func drawRectBorder(img *ebiten.Image, w, h int, c color.Color) {
	// Initialize shared pixel if needed
	if borderPixel == nil {
		borderPixel = ebiten.NewImage(1, 1)
	}
	borderPixel.Fill(c)

	// Top border
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(w), 1)
	img.DrawImage(borderPixel, op)

	// Bottom border
	op = &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(w), 1)
	op.GeoM.Translate(0, float64(h-1))
	img.DrawImage(borderPixel, op)

	// Left border
	op = &ebiten.DrawImageOptions{}
	op.GeoM.Scale(1, float64(h))
	img.DrawImage(borderPixel, op)

	// Right border
	op = &ebiten.DrawImageOptions{}
	op.GeoM.Scale(1, float64(h))
	op.GeoM.Translate(float64(w-1), 0)
	img.DrawImage(borderPixel, op)
}

var menuTitleFace *text.GoTextFace
var menuButtonFace *text.GoTextFace

// getHost returns the current host for WebSocket connection
func getHost() string {
	if runtime.GOARCH == "wasm" {
		return getHostWASM()
	}
	return "localhost:3000"
}

// Lobby functions

func (g *UnoGame) updateLobby() {
	// Poll for network messages
	if g.network != nil {
		for {
			msg := g.network.Poll()
			if msg == nil {
				break
			}
			g.handleNetworkMessage(msg)
		}
	}

	// Back button
	mx, my := ebiten.CursorPosition()
	backX, backY := 20, 20
	if mx >= backX && mx < backX+100 && my >= backY && my < backY+40 {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.returnToMenu()
		}
	}
}

func (g *UnoGame) handleNetworkMessage(msg *server.ServerMessage) {
	switch msg.Type {
	case server.MsgRoomInfo:
		// Got room info
	case server.MsgPlayerJoined:
		g.lobbyPlayers = append(g.lobbyPlayers, msg.PlayerName)
	case server.MsgPlayerLeft:
		// Remove player from lobby
		for i, name := range g.lobbyPlayers {
			if name == msg.PlayerName {
				g.lobbyPlayers = append(g.lobbyPlayers[:i], g.lobbyPlayers[i+1:]...)
				break
			}
		}
	case server.MsgGameState:
		// Game started! Transition to playing
		g.mode = ModePlayers
		// TODO: sync state from server
	}
}

func (g *UnoGame) drawLobby(screen *ebiten.Image) {
	screen.Fill(color.RGBA{20, 20, 30, 255})

	// Title
	title := "Waiting for Players..."
	face := getMenuButtonFace()
	if face != nil {
		tw, _ := text.Measure(title, face, 0)
		titleX := (float64(g.screenWidth) - tw) / 2
		titleY := float64(g.screenHeight) / 4

		op := &text.DrawOptions{}
		op.GeoM.Translate(titleX, titleY)
		op.ColorScale.ScaleWithColor(color.White)
		text.Draw(screen, title, face, op)
	}

	// Player list
	startY := g.screenHeight / 3
	for i, name := range g.lobbyPlayers {
		playerText := fmt.Sprintf("%d. %s", i+1, name)
		if face != nil {
			tw, _ := text.Measure(playerText, face, 0)
			x := (float64(g.screenWidth) - tw) / 2
			y := float64(startY + i*40)

			op := &text.DrawOptions{}
			op.GeoM.Translate(x, y)
			op.ColorScale.ScaleWithColor(color.RGBA{200, 200, 200, 255})
			text.Draw(screen, playerText, face, op)
		}
	}

	// Connection status
	status := "Connecting..."
	if g.network != nil && g.network.IsConnected() {
		if IsDiscordActivity() {
			status = "Connected - Friends in voice channel will auto-join"
		} else {
			status = "Connected - Waiting for players..."
		}
	}
	if face != nil {
		tw, _ := text.Measure(status, face, 0)
		x := (float64(g.screenWidth) - tw) / 2
		y := float64(g.screenHeight) * 0.7

		op := &text.DrawOptions{}
		op.GeoM.Translate(x, y)
		op.ColorScale.ScaleWithColor(color.RGBA{150, 150, 150, 255})
		text.Draw(screen, status, face, op)
	}

	// Back button
	drawMenuButton(screen, 20, 20, 100, 40, "Back", false)
}

func getMenuTitleFace() *text.GoTextFace {
	if menuTitleFace == nil && fontSource != nil {
		menuTitleFace = &text.GoTextFace{
			Source: fontSource,
			Size:   96,
		}
	}
	return menuTitleFace
}

func getMenuButtonFace() *text.GoTextFace {
	if menuButtonFace == nil && fontSource != nil {
		menuButtonFace = &text.GoTextFace{
			Source: fontSource,
			Size:   28,
		}
	}
	return menuButtonFace
}
