package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/idarciooliveira/safe-past/ui/backend"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	app := backend.New()

	err := wails.Run(&options.App{
		Title:  "safe-past",
		Width:  1120,
		Height: 720,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
