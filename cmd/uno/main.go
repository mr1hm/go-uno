package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mr1hm/go-uno/internal/render"
)

func main() {
	game := render.NewUnoGame([]string{"You", "CPU 1", "CPU 2", "CPU 3"})

	ebiten.SetWindowSize(1280, 720)
	ebiten.SetWindowTitle("Go Uno")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	// For WASM, the game will use the canvas size from Layout()
	// Don't call SetFullscreen - browsers require user gesture

	// 60 TPS for smooth animations
	ebiten.SetTPS(60)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
