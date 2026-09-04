package backend

import (
	"io/fs"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Run configures and starts the desktop application.
func Run(assets fs.FS) error {
	app := application.New(application.Options{
		Name:        "SARIF Viewer",
		Description: "Review and annotate findings from SARIF files",
		Services: []application.Service{
			application.NewService(NewSARIFService()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "SARIF Viewer",
		Width:     1440,
		Height:    900,
		MinWidth:  980,
		MinHeight: 640,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	return app.Run()
}
