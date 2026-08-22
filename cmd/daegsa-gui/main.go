package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/unit"
	"github.com/charleszardd/daegsa/internal/gui"
	"github.com/charleszardd/daegsa/internal/gui/views"
)

func main() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("DAEGSA Studio - REST API Load & Capacity Testing"),
			app.Size(unit.Dp(1150), unit.Dp(750)),
			app.MinSize(unit.Dp(900), unit.Dp(600)),
		)

		application := gui.NewApp(w)
		builderView := views.NewBuilderView(application.State)
		monitorView := views.NewMonitorView(application.State)
		compareView := views.NewCompareView(application.State)
		doctorView := views.NewDoctorView(application.State)

		application.SetViews(builderView, monitorView, compareView, doctorView)

		if err := application.Run(); err != nil {
			log.Printf("DAEGSA Studio error: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	app.Main()
}
