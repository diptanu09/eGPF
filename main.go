//go:generate goversioninfo -platform=amd64
package main

import (
	"log"

	"fyne.io/fyne/v2" // Added for Resource loading
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

	// Load and set the application icon for the runtime title bar
	// Ensure assets/logo.png exists in your project directory
	icon, err := fyne.LoadResourceFromPath("assets/logo.png")
	if err == nil {
		myApp.SetIcon(icon)
	}

	myWindow := myApp.NewWindow("eGPF Management System Gateway")

	// 3. Mount login portal view entrypoint
	ui.RenderLoginView(myApp, myWindow)

	myWindow.ShowAndRun()
}
