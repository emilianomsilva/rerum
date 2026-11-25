package main

import (
	"context"
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--import-legacy-data" {
		dir, err := userDataDir()
		if err == nil {
			err = importLegacyData(context.Background(), os.Args[2], dir)
		}
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "Rerum — Gestão patrimonial e de aluguéis",
		Width:            1440,
		Height:           900,
		MinWidth:         980,
		MinHeight:        640,
		WindowStartState: options.Maximised,
		BackgroundColour: &options.RGBA{R: 243, G: 245, B: 247, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			Icon:             appIcon,
			ProgramName:      "rerum",
			WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
