package main

import (
	"flag"
	"log"
	"os"
	"runtime/pprof"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mr1hm/go-uno/internal/render"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	game := render.NewUnoGame([]string{"You", "CPU 1", "CPU 2", "CPU 3"})

	ebiten.SetWindowSize(1280, 720)
	ebiten.SetWindowTitle("Go Uno")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	// For WASM, the game will use the canvas size from Layout()
	// Don't call SetFullscreen - browsers require user gesture

	// Limit updates to 60 per second
	ebiten.SetTPS(60)

	// Enable VSync to prevent uncapped frame rate
	ebiten.SetVsyncEnabled(true)

	// Don't clear screen every frame - allows true skip-draw
	ebiten.SetScreenClearedEveryFrame(false)

	// Use single-thread mode to reduce synchronization overhead
	ebiten.SetRunnableOnUnfocused(true)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
