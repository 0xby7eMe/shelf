package main

import (
	"context"
	"embed"
	"net/http"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"shelf/internal/library"
)

// osArgs is what Shelf was started with, apart from the program itself.
func osArgs() []string { return os.Args[1:] }

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	library.EnterAppImage()

	app := NewApp()

	covers, err := library.NewCovers()
	if err != nil {
		println("Error:", err.Error())
		return
	}

	ds := app.desk.Get()
	closeToTray := ds.Tray && ds.CloseToTray

	err = wails.Run(&options.App{
		Title:     "Shelf",
		Width:     1424,
		Height:    820,
		MinWidth:  720,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Middleware: func(next http.Handler) http.Handler {
				return covers.Middleware(library.Sounds(next))
			},
		},
		Linux: &linux.Options{
			Icon:             icon,
			WebviewGpuPolicy: linux.WebviewGpuPolicyAlways,
		},
		// One Shelf at a time: a shelf:// link or menu entry reaches the running one.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "dev.0xby7eme.shelf",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				app.onSecondInstance(data.Args)
			},
		},
		// With the tray on, closing the window can keep Shelf running there.
		BackgroundColour: &options.RGBA{R: 10, G: 10, B: 10, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose: func(ctx context.Context) bool {
			return app.beforeClose(closeToTray)
		},
		Bind: []interface{}{
			app,
		},
		Frameless: true,
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
