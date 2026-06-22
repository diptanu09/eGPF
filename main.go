//go:generate goversioninfo -platform=amd64
package main

import (
	"log"

	"fyne.io/fyne/v2/app"

	"egpf-app/db"
	"egpf-app/ui"
)

func main() {
	// 1. Establish data layer connectivity
	if err := db.InitDatabase(); err != nil {
		log.Fatalf("Database initialization terminal breakdown: %v", err)
	}

	// 2. Provision window framing contexts
	myApp := app.New()

	// 3. Set embedded icon globally on the App instance
	if ui.ResourceLogoPng != nil {
		myApp.SetIcon(ui.ResourceLogoPng)
	}

	myWindow := myApp.NewWindow("eGPF Management System Gateway")

	// 4. Force the embedded icon onto the active window manager frame
	if ui.ResourceLogoPng != nil {
		myWindow.SetIcon(ui.ResourceLogoPng)
	}

	// 5. Mount login portal view entrypoint
	ui.RenderLoginView(myApp, myWindow)

	myWindow.ShowAndRun()
}
