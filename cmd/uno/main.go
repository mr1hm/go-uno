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

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
