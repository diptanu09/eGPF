package ui

import (
	"fmt"
	"time"

	"egpf-app/db"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func RenderLoginView(app fyne.App, window fyne.Window) {
	window.Resize(fyne.NewSize(450, 600))
	window.CenterOnScreen()

	// Use the embedded static resource instead of an external file path
	var logoImg *canvas.Image
	if ResourceLogoPng != nil {
		logoImg = canvas.NewImageFromResource(ResourceLogoPng)
	} else {
		logoImg = canvas.NewImageFromResource(theme.AccountIcon())
	}
	logoImg.FillMode = canvas.ImageFillContain
	logoImg.SetMinSize(fyne.NewSize(110, 110))

	titleLabel := widget.NewLabelWithStyle("eGPF SYSTEM TERMINAL", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	subtitleLabel := widget.NewLabelWithStyle("Enter credentials to establish secure core session", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})

	usernameBox := widget.NewEntry()
	usernameBox.SetPlaceHolder("Operator Username")

	passwordBox := widget.NewPasswordEntry()
	passwordBox.SetPlaceHolder("Security Access Password")

	formLayout := container.NewVBox(
		widget.NewLabel("System Identity Handle:"),
		usernameBox,
		layout.NewSpacer(),
		widget.NewLabel("Access Authorization PIN/Pass:"),
		passwordBox,
	)

	loginButton := widget.NewButtonWithIcon("Establish Session Connection", theme.LoginIcon(), func() {
		if usernameBox.Text == "" || passwordBox.Text == "" {
			dialog.ShowError(fmt.Errorf("Access Denied:\nInput vector entries cannot be blank."), window)
			return
		}

		role, lastLogin, err := db.AuthenticateUser(usernameBox.Text, passwordBox.Text)
		if err != nil {
			if err.Error() == "AWAITING_ADMIN_APPROVAL" {
				dialog.ShowError(fmt.Errorf("Security Exception:\nYour application handle is currently PENDING administrative approval/role assignment."), window)
			} else if err.Error() == "TERMINAL_LOCK_VIOLATION" {
				dialog.ShowError(fmt.Errorf("Security Lock Violation:\nThis user profile is bound to a different workstation node.\nAccess from this physical system is denied."), window)
			} else {
				dialog.ShowError(fmt.Errorf("Authentication Rejected:\nInvalid system credential pairing provided."), window)
			}
			return
		}

		db.RefreshUserTimestamp(usernameBox.Text)
		LaunchOperationalDashboard(app, window, usernameBox.Text, role, lastLogin.Format("2006-01-02 15:04:05"))
	})
	loginButton.Importance = widget.HighImportance

	registerButton := widget.NewButtonWithIcon("Request New Access Account", theme.DocumentCreateIcon(), func() {
		ShowPublicRegistrationDialog(window)
	})

	footerText := widget.NewLabelWithStyle("Authorized personnel access only. Actions are monitored.", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})

	contentView := container.NewVBox(
		container.NewPadded(logoImg),
		titleLabel,
		subtitleLabel,
		widget.NewSeparator(),
		container.NewPadded(formLayout),
		layout.NewSpacer(),
		container.NewPadded(loginButton),
		container.NewPadded(registerButton),
		layout.NewSpacer(),
		footerText,
	)

	// Tied labels directly to the dashboard version constraints to bypass obfuscation drops
	loginFooterLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("System Registry Core v%s\n%s", CurrentClientVersion, CopyrightInfo),
		fyne.TextAlignCenter,
		fyne.TextStyle{Italic: true},
	)
	loginFooter := container.NewVBox(
		widget.NewSeparator(),
		loginFooterLabel,
	)

	mainLayout := container.NewBorder(
		nil,
		loginFooter,
		nil, nil,
		container.NewPadded(contentView),
	)

	window.SetContent(mainLayout)

	// Automated Version Alignment Control Check Hook Verification
	go func() {
		// Small delay to ensure the parent frame rendering cycles complete smoothly
		time.Sleep(200 * time.Millisecond)

		latestVer, sharedPath, err := db.FetchLatestAppVersion()
		if err == nil && latestVer != CurrentClientVersion {
			window.Canvas().Scale()

			// FIXED: Created the entry widget explicitly to avoid invalid method chaining syntax errors
			pathDisplayEntry := widget.NewEntry()
			pathDisplayEntry.SetText(sharedPath)

			msgContent := container.NewVBox(
				widget.NewLabelWithStyle("⚠️ CRITICAL SYSTEM VERSION MISMATCH", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
				widget.NewSeparator(),
				widget.NewLabel(fmt.Sprintf("Local Executable Version: v%s", CurrentClientVersion)),
				widget.NewLabel(fmt.Sprintf("Required Database Version: v%s", latestVer)),
				widget.NewSeparator(),
				widget.NewLabel("Access Denied: Terminal connection cannot proceed on an outdated build."),
				widget.NewLabelWithStyle("Please update your local executable file immediately.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
				widget.NewSeparator(),
				widget.NewLabel("Shared Distribution Network Path:"),
				pathDisplayEntry, // Embedded cleanly now that it's an independent instance variable
			)

			// Show a blocking custom dialog box that locks interaction actions
			d := dialog.NewCustom("Mandatory System Update Required", "Exit Terminal", msgContent, window)
			d.SetOnClosed(func() {
				app.Quit()
			})

			// Disable operational center interactives to prevent unauthorized login bypasses
			loginButton.Disable()
			registerButton.Disable()
			usernameBox.Disable()
			passwordBox.Disable()

			d.Show()
		}
	}()
}

func ShowPublicRegistrationDialog(window fyne.Window) {
	regUserEntry := widget.NewEntry()
	regUserEntry.SetPlaceHolder("Choose unique operator name")

	regPassEntry := widget.NewPasswordEntry()
	regPassEntry.SetPlaceHolder("Establish secure session key string")

	formItems := []*widget.FormItem{
		widget.NewFormItem("Requested Username", regUserEntry),
		widget.NewFormItem("Account Password", regPassEntry),
	}

	dialog.ShowForm("Submit Access Registration Request", "Submit Request", "Cancel", formItems, func(confirmed bool) {
		if confirmed {
			if regUserEntry.Text == "" || regPassEntry.Text == "" {
				dialog.ShowError(fmt.Errorf("Submission Error: Fields cannot be left empty"), window)
				return
			}

			err := db.ExecuteRegisterSelf(regUserEntry.Text, regPassEntry.Text)
			if err == nil {
				dialog.ShowInformation("Registration Submitted", "Success: Your request was successfully queued for processing.\nAn administrator must review and activate your workspace profile before first login.", window)
			} else {
				dialog.ShowError(err, window)
			}
		}
	}, window)
}
