//go:generate goversioninfo -platform=amd64
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"egpf-app/db"
	"egpf-app/ui"
)

func main() {
	// 1. Provision window framing contexts first
	myApp := app.New()
	myWindow := myApp.NewWindow("eGPF Management System Gateway")
	myWindow.Resize(fyne.NewSize(500, 500))
	myWindow.CenterOnScreen()

	// 2. Set embedded icon globally on the App and Window instance
	if ui.ResourceLogoPng != nil {
		myApp.SetIcon(ui.ResourceLogoPng)
		myWindow.SetIcon(ui.ResourceLogoPng)
	}

	// 3. Establish data layer connectivity & verify root CA certificate
	_, err := db.InitDatabase()
	if err != nil {
		// Prepare custom error layout
		msgContent := container.NewVBox(
			widget.NewLabelWithStyle("⚠️ CRITICAL SECURITY / DATABASE ERROR", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			widget.NewLabel("Failed to establish secure connection to the central database server."),
			widget.NewSeparator(),
			widget.NewLabelWithStyle("Possible Cause:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabel("1. The 'certs/root.crt' security certificate file is missing."),
			widget.NewLabel("2. Network connection to database server timed out."),
			widget.NewSeparator(),
			widget.NewLabelWithStyle("Action Required:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: true}),
			widget.NewLabel("Please verify that 'certs/root.crt' exists in your application directory,\nor contact your Administrator / IT Cell immediately."),
		)

		// Create blocking custom modal dialog
		d := dialog.NewCustom("System Security Notice", "Exit Terminal", msgContent, myWindow)
		d.SetOnClosed(func() {
			myApp.Quit()
		})

		myWindow.SetContent(container.NewPadded(msgContent))
		myWindow.Show()
		d.Show()

		myApp.Run()
		return
	}

	// 4. Mount login portal view entrypoint
	ui.RenderLoginView(myApp, myWindow)

	myWindow.ShowAndRun()
}
