package ui

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"egpf-app/db"
)

const (
	CurrentClientVersion   = "2.3.4"
	CopyrightInfo          = "© 2026 O/o the Accountant General (A&E), Tripura. \nAll Rights Reserved."
	SessionInactivityLimit = 5 * time.Minute
)

var selectedRowIndex int = -1

func generateRandom4DigitPIN() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("%04d", r.Intn(10000))
}

func LaunchOperationalDashboard(app fyne.App, window fyne.Window, username string, role string, lastLogin string) {
	window.Resize(fyne.NewSize(1280, 720))
	window.CenterOnScreen()

	gridData := [][]string{{
		"Series ID & Name", "Account Number", "Subscriber Name",
		"Employee Code", "Beneficiary Code", "Designation",
		"Mobile No", "Date of Birth", "DDO Code", "PIN",
	}}
	selectedRowIndex = -1

	canWrite := db.EvaluateUserPermission(username, role, "can_write", role == "admin" || role == "operator")
	canDelete := (role == "admin")
	canManageUsers := (role == "admin")
	canAssignSubscriberPin := db.EvaluateUserPermission(username, role, "can_assign_pin", role == "admin")
	canViewDdoData := db.EvaluateUserPermission(username, role, "can_view_ddo", true)

	var activeDialogs []dialog.Dialog
	var dialogsLock sync.Mutex

	trackDialog := func(d dialog.Dialog) {
		dialogsLock.Lock()
		activeDialogs = append(activeDialogs, d)
		dialogsLock.Unlock()
	}

	clearAllActiveDialogs := func() {
		dialogsLock.Lock()
		for _, d := range activeDialogs {
			if d != nil {
				d.Hide()
			}
		}
		activeDialogs = nil
		dialogsLock.Unlock()
	}

	var activityLock sync.Mutex
	lastActivityTime := time.Now()
	isSessionActive := true

	resetInactivityTimer := func() {
		activityLock.Lock()
		lastActivityTime = time.Now()
		activityLock.Unlock()
	}

	window.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		resetInactivityTimer()
	})

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			activityLock.Lock()
			elapsed := time.Since(lastActivityTime)
			activeStatus := isSessionActive
			activityLock.Unlock()

			if !activeStatus {
				return
			}

			if elapsed >= SessionInactivityLimit {
				activityLock.Lock()
				isSessionActive = false
				activityLock.Unlock()

				clearAllActiveDialogs()

				dialog.ShowInformation("🛡️ Security Timeout", "Your terminal session context was terminated automatically due to inactivity.", window)
				gridData = [][]string{{
					"Series ID & Name", "Account Number", "Subscriber Name",
					"Employee Code", "Beneficiary Code", "Designation",
					"Mobile No", "Date of Birth", "DDO Code", "PIN",
				}}
				selectedRowIndex = -1
				RenderLoginView(app, window)
				return
			}
		}
	}()

	seriesOptions, seriesMap, err := db.FetchSeriesDropdownOptions()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to populate system series definitions: %v", err), window)
	} else if len(seriesOptions) > 0 {
		sort.Slice(seriesOptions, func(i, j int) bool {
			var id1, id2 int
			fmt.Sscanf(seriesOptions[i], "%d", &id1)
			fmt.Sscanf(seriesOptions[j], "%d", &id2)
			return id1 < id2
		})
	}

	dataTable := widget.NewTable(
		func() (int, int) { return len(gridData), len(gridData[0]) },
		func() fyne.CanvasObject {
			lbl := widget.NewLabel("")
			lbl.Wrapping = fyne.TextTruncate
			return lbl
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			if id.Row < len(gridData) && id.Col < len(gridData[0]) {
				label := cell.(*widget.Label)
				label.SetText(gridData[id.Row][id.Col])
				if id.Row == 0 {
					label.TextStyle = fyne.TextStyle{Bold: true}
				} else {
					label.TextStyle = fyne.TextStyle{}
				}
			}
		},
	)

	dataTable.SetColumnWidth(0, 160)
	dataTable.SetColumnWidth(1, 140)
	dataTable.SetColumnWidth(2, 180)
	dataTable.SetColumnWidth(3, 110)
	dataTable.SetColumnWidth(4, 120)
	dataTable.SetColumnWidth(5, 150)
	dataTable.SetColumnWidth(6, 120)
	dataTable.SetColumnWidth(7, 110)
	dataTable.SetColumnWidth(8, 100)
	dataTable.SetColumnWidth(9, 90)

	syncUiViewGrid := func(seriesTerm, accountTerm, nameTerm string, forceAll bool) {
		resetInactivityTimer()
		var err error
		gridData, err = db.FetchSubscriberRegistry(seriesTerm, accountTerm, nameTerm, forceAll)
		if err != nil {
			dialog.ShowError(err, window)
		}
		selectedRowIndex = -1
		dataTable.Refresh()
	}

	dataTable.OnSelected = func(id widget.TableCellID) {
		resetInactivityTimer()
		if id.Row == 0 {
			dataTable.Unselect(id)
			return
		}
		selectedRowIndex = id.Row
	}

	var selectedSearchSeriesID string
	searchSeriesSelect := widget.NewSelect(append([]string{"[All Series Categories]"}, seriesOptions...), func(selected string) {
		resetInactivityTimer()
		if selected == "[All Series Categories]" || selected == "" {
			selectedSearchSeriesID = ""
		} else {
			selectedSearchSeriesID = seriesMap[selected]
		}
	})
	searchSeriesSelect.SetSelected("[All Series Categories]")

	searchAccountBox := widget.NewEntry()
	searchAccountBox.SetPlaceHolder("Enter Account No...")

	searchNameBox := widget.NewEntry()
	searchNameBox.SetPlaceHolder("Enter Subscriber Name...")

	seriesSizer := container.NewGridWrap(fyne.NewSize(260, 36), searchSeriesSelect)
	accountSizer := container.NewGridWrap(fyne.NewSize(180, 36), searchAccountBox)
	nameSizer := container.NewGridWrap(fyne.NewSize(200, 36), searchNameBox)

	searchButton := widget.NewButtonWithIcon("Search", theme.SearchIcon(), func() {
		syncUiViewGrid(selectedSearchSeriesID, searchAccountBox.Text, searchNameBox.Text, false)
	})

	clearButton := widget.NewButtonWithIcon("Reset", theme.ViewRefreshIcon(), func() {
		resetInactivityTimer()
		searchSeriesSelect.SetSelected("[All Series Categories]")
		searchAccountBox.SetText("")
		searchNameBox.SetText("")
		gridData = [][]string{{
			"Series ID & Name", "Account Number", "Subscriber Name",
			"Employee Code", "Beneficiary Code", "Designation",
			"Mobile No", "Date of Birth", "DDO Code", "PIN",
		}}
		selectedRowIndex = -1
		dataTable.Refresh()
	})

	searchBarLayout := container.NewHBox(
		widget.NewLabelWithStyle("🔍 Search Subscriber:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		seriesSizer,
		accountSizer,
		nameSizer,
		searchButton,
		clearButton,
	)

	// UPDATED: Dynamic Avatar lookup evaluation supporting static assignments
	resolveProfileAvatar := func(avatarSelection string, targetRole string, width, height float32) fyne.CanvasObject {
		var avatarRes fyne.Resource

		switch avatarSelection {
		case "avatar_1":
			avatarRes = ResourceAvatar1Png
		case "avatar_2":
			avatarRes = ResourceAvatar2Png
		case "avatar_3":
			avatarRes = ResourceAvatar3Png
		case "avatar_4":
			avatarRes = ResourceAvatar4Png
		default:
			switch targetRole {
			case "admin":
				// FIXED: Match lowercase if generated that way by the bundler tool
				if ResourceRoleAdminPng != nil {
					avatarRes = ResourceRoleAdminPng
				}
			case "operator":
				// FIXED: Match lowercase if generated that way by the bundler tool
				if ResourceRoleOperatorPng != nil {
					avatarRes = ResourceRoleOperatorPng
				}
			}
		}

		if avatarRes == nil {
			avatarRes = theme.AccountIcon()
		}

		img := canvas.NewImageFromResource(avatarRes)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(width, height))
		return img
	}

	executeViewDdo := func() {
		if selectedRowIndex == -1 {
			return
		}
		ddoCodeTarget := gridData[selectedRowIndex][8]
		if ddoCodeTarget == "" || ddoCodeTarget == "N/A" {
			dialog.ShowError(fmt.Errorf("No DDO mapping assigned to this subscriber row space"), window)
			return
		}
		ddoInfo, err := db.FetchDDODetails(ddoCodeTarget)
		if err != nil {
			dialog.ShowError(err, window)
			return
		}

		displayPin := ddoInfo["pin"]
		if role != "admin" {
			displayPin = "[REDACTED]"
		}

		infoContent := container.NewVBox(
			widget.NewLabelWithStyle(fmt.Sprintf("DDO Master Code: %s", ddoInfo["ddo_code"]), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			widget.NewLabel(fmt.Sprintf("Designation: %s", ddoInfo["ddo_desg"])),
			widget.NewLabel(fmt.Sprintf("Phone No: %s", ddoInfo["ddo_phone"])),
			widget.NewLabel(fmt.Sprintf("Email: %s", ddoInfo["ddo_email"])),
			widget.NewLabel(fmt.Sprintf("Treasury Code: %s", ddoInfo["ddo_tres_code"])),
			widget.NewLabel(fmt.Sprintf("VLC Code: %s", ddoInfo["vlc_ddo_code"])),
			widget.NewLabel(fmt.Sprintf("Gate Access PIN: %s", displayPin)),
		)
		d := dialog.NewCustom("DDO Details", "Close", infoContent, window)
		trackDialog(d)
		d.Show()
	}

	executeAssignPin := func() {
		if selectedRowIndex == -1 {
			newAccountNoEntry := widget.NewEntry()
			newAccountNoEntry.SetPlaceHolder("Account Number")
			seriesSelectDropdown := widget.NewSelect(seriesOptions, nil)
			seriesSelectDropdown.PlaceHolder = "[Select Series]"
			initialPinField := widget.NewEntry()
			initialPinField.SetPlaceHolder("Initial Access PIN")
			autoGenBtn := widget.NewButtonWithIcon("Auto-Generate", theme.ViewRefreshIcon(), func() {
				initialPinField.SetText(generateRandom4DigitPIN())
			})

			formItems := []*widget.FormItem{
				widget.NewFormItem("Series", seriesSelectDropdown),
				widget.NewFormItem("Account No", newAccountNoEntry),
				widget.NewFormItem("PIN Key", initialPinField),
				widget.NewFormItem("Action", autoGenBtn),
			}

			d := dialog.NewForm("Register New Subscriber Profile", "Create Account", "Cancel", formItems, func(confirmed bool) {
				if confirmed {
					if seriesSelectDropdown.Selected == "" || newAccountNoEntry.Text == "" || initialPinField.Text == "" {
						return
					}
					resolvedSeriesID := seriesMap[seriesSelectDropdown.Selected]
					if err := db.ExecuteCreateNewSubscriber(username, resolvedSeriesID, newAccountNoEntry.Text, initialPinField.Text); err == nil {
						syncUiViewGrid(resolvedSeriesID, newAccountNoEntry.Text, "", false)
					}
				}
			}, window)
			trackDialog(d)
			d.Show()
			return
		}

		rawSeriesStr := gridData[selectedRowIndex][0]
		var parsedSeriesID string
		fmt.Sscanf(rawSeriesStr, "%s", &parsedSeriesID)
		targetAccountNo := gridData[selectedRowIndex][1]
		targetName := gridData[selectedRowIndex][2]

		pinEntryField := widget.NewEntry()
		pinEntryField.SetPlaceHolder("New PIN")
		autoGenResetBtn := widget.NewButtonWithIcon("Auto-Generate", theme.ViewRefreshIcon(), func() {
			pinEntryField.SetText(generateRandom4DigitPIN())
		})

		formItems := []*widget.FormItem{
			widget.NewFormItem("Subscriber", widget.NewLabel(targetName)),
			widget.NewFormItem("Account No", widget.NewLabel(targetAccountNo)),
			widget.NewFormItem("New PIN", pinEntryField),
			widget.NewFormItem("Action", autoGenResetBtn),
		}

		d := dialog.NewForm("Modify Subscriber Portal PIN", "Save PIN", "Cancel", formItems, func(confirmed bool) {
			if confirmed && pinEntryField.Text != "" {
				if err := db.ExecuteResetSubscriberPIN(username, parsedSeriesID, targetAccountNo, pinEntryField.Text); err == nil {
					syncUiViewGrid(parsedSeriesID, targetAccountNo, "", false)
				}
			}
		}, window)
		trackDialog(d)
		d.Show()
	}

	executeUpdateRow := func() {
		if selectedRowIndex == -1 {
			return
		}
		rawSeriesStr := gridData[selectedRowIndex][0]
		var oldSerID string
		fmt.Sscanf(rawSeriesStr, "%s", &oldSerID)
		oldAccNo := gridData[selectedRowIndex][1]

		designationEntry := widget.NewEntry()
		designationEntry.SetText(gridData[selectedRowIndex][5])
		mobileEntry := widget.NewEntry()
		mobileEntry.SetText(gridData[selectedRowIndex][6])
		pinEntry := widget.NewEntry()
		pinEntry.SetText(gridData[selectedRowIndex][9])

		items := []*widget.FormItem{
			widget.NewFormItem("Series", widget.NewLabel(rawSeriesStr)),
			widget.NewFormItem("Account No", widget.NewLabel(oldAccNo)),
			widget.NewFormItem("Designation", designationEntry),
			widget.NewFormItem("Mobile No.", mobileEntry),
			widget.NewFormItem("PIN Code", pinEntry),
		}
		d := dialog.NewForm("Modify Subscriber Data", "Apply", "Cancel", items, func(confirmed bool) {
			if confirmed {
				if err := db.ExecuteUpdateRecord(username, oldSerID, oldAccNo, pinEntry.Text, mobileEntry.Text, designationEntry.Text, oldSerID, oldAccNo); err == nil {
					syncUiViewGrid(selectedSearchSeriesID, searchAccountBox.Text, searchNameBox.Text, false)
				}
			}
		}, window)
		trackDialog(d)
		d.Show()
	}

	executeDeleteRow := func() {
		if selectedRowIndex == -1 {
			return
		}
		rawSeriesStr := gridData[selectedRowIndex][0]
		var targetSerID string
		fmt.Sscanf(rawSeriesStr, "%s", &targetSerID)
		targetAccNo := gridData[selectedRowIndex][1]

		d := dialog.NewConfirm("Confirm Drop Action", "Permanently delete subscriber record?", func(confirmed bool) {
			if confirmed {
				if err := db.ExecuteDeleteRecord(username, targetSerID, targetAccNo); err == nil {
					syncUiViewGrid(selectedSearchSeriesID, searchAccountBox.Text, searchNameBox.Text, false)
				}
			}
		}, window)
		trackDialog(d)
		d.Show()
	}

	actionMenuButton := widget.NewSelect([]string{"[Choose Row Operation]", "View DDO Info", "Assign/Reset PIN", "Update Profile Data", "Delete Record Space"}, func(chosen string) {
		resetInactivityTimer()
		if selectedRowIndex == -1 && chosen != "Assign/Reset PIN" {
			dialog.ShowInformation("Selection Required", "Please click a row in the registry grid below before choosing an action item option.", window)
			return
		}
		switch chosen {
		case "View DDO Info":
			if canViewDdoData {
				executeViewDdo()
			} else {
				dialog.ShowError(fmt.Errorf("Security Exception: Access Denied"), window)
			}
		case "Assign/Reset PIN":
			if canAssignSubscriberPin {
				executeAssignPin()
			} else {
				dialog.ShowError(fmt.Errorf("Security Exception: Access Denied"), window)
			}
		case "Update Profile Data":
			if canWrite {
				executeUpdateRow()
			} else {
				dialog.ShowError(fmt.Errorf("Security Exception: Access Denied"), window)
			}
		case "Delete Record Space":
			if canDelete {
				executeDeleteRow()
			} else {
				dialog.ShowError(fmt.Errorf("Security Exception: Access Denied"), window)
			}
		}
	})
	actionMenuButton.Selected = "[Choose Row Operation]"
	actionMenuSizer := container.NewGridWrap(fyne.NewSize(240, 36), actionMenuButton)

	systemToolsMenu := widget.NewSelect([]string{"[System Tools Portal Menu]", "DDO Master Directory Registry", "Add New Fund Series Category", "Manage Local User Profiles", "Pending Registration Requests", "Feature Permissions Overrides", "App Distribution Version Setup"}, func(tool string) {
		resetInactivityTimer()
		if role != "admin" && tool != "DDO Master Directory Registry" {
			dialog.ShowError(fmt.Errorf("Security Constraint: Administrative levels required."), window)
			return
		}
		switch tool {
		case "DDO Master Directory Registry":
			if !canViewDdoData {
				dialog.ShowError(fmt.Errorf("Access Denied"), window)
				return
			}

			var ddoGridData [][]string
			var selectedDdoRowIndex int = -1

			ddoSearchBox := widget.NewEntry()
			ddoSearchBox.SetPlaceHolder("Type DDO Code to filter directory matrix on-the-fly...")

			ddoTable := widget.NewTable(
				func() (int, int) { return len(ddoGridData), 5 },
				func() fyne.CanvasObject {
					lbl := widget.NewLabel("")
					lbl.Wrapping = fyne.TextTruncate
					return lbl
				},
				func(id widget.TableCellID, cell fyne.CanvasObject) {
					if id.Row < len(ddoGridData) && id.Col < 5 {
						label := cell.(*widget.Label)
						label.SetText(ddoGridData[id.Row][id.Col])
						if id.Row == 0 {
							label.TextStyle = fyne.TextStyle{Bold: true}
						} else {
							label.TextStyle = fyne.TextStyle{}
						}
					}
				},
			)

			ddoTable.SetColumnWidth(0, 100)
			ddoTable.SetColumnWidth(1, 250)
			ddoTable.SetColumnWidth(2, 130)
			ddoTable.SetColumnWidth(3, 240)
			ddoTable.SetColumnWidth(4, 100)

			syncDdoGrid := func(filter string) {
				profiles, _ := db.FetchAllDDOMasterProfiles(filter)
				matrix := [][]string{{
					"DDO Code", "Official Designation", "Phone No", "Email Address", "Gate Access PIN",
				}}
				for _, p := range profiles {
					pinText := p[6]
					if role != "admin" {
						pinText = "****"
					}
					matrix = append(matrix, []string{p[0], p[1], p[2], p[3], pinText})
				}
				ddoGridData = matrix
				selectedDdoRowIndex = -1
				ddoTable.UnselectAll()
				ddoTable.Refresh()
			}

			ddoTable.OnSelected = func(id widget.TableCellID) {
				resetInactivityTimer()
				if id.Row == 0 {
					ddoTable.Unselect(id)
					return
				}
				selectedDdoRowIndex = id.Row
			}

			ddoSearchBox.OnChanged = syncDdoGrid

			managePinBtn := widget.NewButtonWithIcon("Update DDO Profile Details", theme.DocumentCreateIcon(), func() {
				resetInactivityTimer()
				if selectedDdoRowIndex == -1 {
					dialog.ShowInformation("Selection Required", "Please click on a row within the DDO matrix table grid first.", window)
					return
				}
				if role != "admin" {
					dialog.ShowError(fmt.Errorf("Security Constraint: Administrative authorization limits required."), window)
					return
				}

				targetDdoCode := ddoGridData[selectedDdoRowIndex][0]
				currentDesg := ddoGridData[selectedDdoRowIndex][1]
				currentPhone := ddoGridData[selectedDdoRowIndex][2]
				currentEmail := ddoGridData[selectedDdoRowIndex][3]
				currentPin := ddoGridData[selectedDdoRowIndex][4]
				if currentPin == "****" {
					currentPin = ""
				}

				desgInputField := widget.NewEntry()
				desgInputField.SetText(currentDesg)
				phoneInputField := widget.NewEntry()
				phoneInputField.SetText(currentPhone)
				emailInputField := widget.NewEntry()
				emailInputField.SetText(currentEmail)
				pinInputField := widget.NewEntry()
				pinInputField.SetText(currentPin)
				pinInputField.SetPlaceHolder("Enter or generate 4-digit PIN")

				autoGenerateBtn := widget.NewButtonWithIcon("Auto-Generate", theme.ViewRefreshIcon(), func() {
					pinInputField.SetText(generateRandom4DigitPIN())
				})

				formItems := []*widget.FormItem{
					widget.NewFormItem("Target DDO Code", widget.NewLabelWithStyle(targetDdoCode, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
					widget.NewFormItem("Official Designation", desgInputField),
					widget.NewFormItem("Phone Number", phoneInputField),
					widget.NewFormItem("Email Address", emailInputField),
					widget.NewFormItem("Security Access PIN", pinInputField),
					widget.NewFormItem("Action Control", autoGenerateBtn),
				}

				d := dialog.NewForm("Modify DDO System Master Profile Records", "Save Changes", "Cancel", formItems, func(confirmed bool) {
					if confirmed {
						err := db.ExecuteUpdateDDOProfile(username, targetDdoCode, desgInputField.Text, phoneInputField.Text, emailInputField.Text, pinInputField.Text)
						if err == nil {
							dialog.ShowInformation("Profile Synced", "DDO master data fields and security bounds saved successfully.", window)
							syncDdoGrid(ddoSearchBox.Text)
						} else {
							dialog.ShowError(err, window)
						}
					}
				}, window)
				trackDialog(d)
				d.Show()
			})

			syncDdoGrid("")

			topPanel := container.NewVBox(ddoSearchBox)
			bottomPanel := container.NewHBox(managePinBtn)
			tableSizer := container.NewPadded(ddoTable)
			mainBorderLayout := container.NewBorder(topPanel, bottomPanel, nil, nil, tableSizer)

			d := dialog.NewCustom("DDO Master Directory Registry Console", "Close Matrix Portal", mainBorderLayout, window)
			d.Resize(fyne.NewSize(880, 500))
			trackDialog(d)
			d.Show()

		case "Add New Fund Series Category":
			idEnt := widget.NewEntry()
			idEnt.SetPlaceHolder("ID")
			nmEnt := widget.NewEntry()
			nmEnt.SetPlaceHolder("Name")
			items := []*widget.FormItem{widget.NewFormItem("Series ID", idEnt), widget.NewFormItem("Series Name", nmEnt)}
			d := dialog.NewForm("Configure Series", "Save", "Cancel", items, func(confirmed bool) {
				if confirmed && idEnt.Text != "" && nmEnt.Text != "" {
					_ = db.ExecuteInsertSeries(idEnt.Text, nmEnt.Text)
				}
			}, window)
			trackDialog(d)
			d.Show()

		case "Manage Local User Profiles":
			if !canManageUsers {
				dialog.ShowError(fmt.Errorf("Security Access Rejected"), window)
				return
			}
			userListContainer := container.NewVBox()
			userSearchBox := widget.NewEntry()
			userSearchBox.SetPlaceHolder("Search system operators directory...")

			var renderDirectoryRows func(string)
			renderDirectoryRows = func(filter string) {
				userListContainer.Objects = nil
				users, err := db.FetchSystemUsers(filter)
				if err != nil {
					userListContainer.Add(widget.NewLabel("Failed to load user records."))
					return
				}

				userListContainer.Add(container.NewHBox(
					container.NewGridWrap(fyne.NewSize(40, 32), widget.NewLabel("")),
					container.NewGridWrap(fyne.NewSize(140, 32), widget.NewLabelWithStyle("Username", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
					container.NewGridWrap(fyne.NewSize(110, 32), widget.NewLabelWithStyle("Clearance", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
					container.NewGridWrap(fyne.NewSize(110, 32), widget.NewLabelWithStyle("Status", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
					container.NewGridWrap(fyne.NewSize(200, 32), widget.NewLabelWithStyle("Session Footprint", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
					container.NewGridWrap(fyne.NewSize(140, 32), widget.NewLabelWithStyle("Action Controls", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
				))
				userListContainer.Add(widget.NewSeparator())

				for _, uData := range users {
					uNameLocal := uData[0]
					currentRoleLocal := uData[1]
					lastLoginTextLocal := uData[2]
					currentStatusLocal := uData[3]
					currentAvatarLocal := uData[5]

					manageAccountBtn := widget.NewButtonWithIcon("Manage Account", theme.SettingsIcon(), func() {
						resetInactivityTimer()
						var subConsoleModal dialog.Dialog

						roleDropdown := widget.NewSelect([]string{"user", "operator", "admin"}, nil)
						roleDropdown.SetSelected(currentRoleLocal)

						// 1. Initialize a dynamic image holder canvas component for preview rendering
						previewAvatarObject := resolveProfileAvatar(currentAvatarLocal, currentRoleLocal, 64, 64)
						previewWrapper := container.NewCenter(previewAvatarObject)

						avatarDropdown := widget.NewSelect([]string{"default", "avatar_1", "avatar_2", "avatar_3", "avatar_4"}, nil)
						avatarDropdown.SetSelected(currentAvatarLocal)

						// 2. Real-time update tracker callback hook for interactive preview transformations
						avatarDropdown.OnChanged = func(newSelection string) {
							// Generate new asset canvas layout and swap current graphic constraints dynamically
							updatedAvatarImage := resolveProfileAvatar(newSelection, roleDropdown.Selected, 64, 64)
							previewWrapper.Objects = []fyne.CanvasObject{updatedAvatarImage}
							previewWrapper.Refresh()
						}

						// Sync avatar choices if role changes while selection remains set to "default"
						roleDropdown.OnChanged = func(newRole string) {
							if avatarDropdown.Selected == "default" {
								updatedAvatarImage := resolveProfileAvatar("default", newRole, 64, 64)
								previewWrapper.Objects = []fyne.CanvasObject{updatedAvatarImage}
								previewWrapper.Refresh()
							}
						}

						saveRoleBtn := widget.NewButtonWithIcon("Save Profile Configuration", theme.DocumentSaveIcon(), func() {
							if uNameLocal == username && roleDropdown.Selected != "admin" {
								dialog.ShowError(fmt.Errorf("You cannot alter your own admin role clearance level"), window)
								return
							}

							err1 := db.ExecuteUpdateUserRole(username, uNameLocal, roleDropdown.Selected)
							err2 := db.ExecuteUpdateUserAvatar(username, uNameLocal, avatarDropdown.Selected)

							if err1 == nil && err2 == nil {
								if subConsoleModal != nil {
									subConsoleModal.Hide()
								}
								renderDirectoryRows(userSearchBox.Text)
							}
						})

						suspendBtnText := "Suspend User Account"
						if currentStatusLocal == "suspended" {
							suspendBtnText = "Reactivate User Account"
						}
						toggleSuspendBtn := widget.NewButtonWithIcon(suspendBtnText, theme.WarningIcon(), func() {
							if uNameLocal == username {
								dialog.ShowError(fmt.Errorf("You cannot alter your own access states"), window)
								return
							}
							isSuspending := (currentStatusLocal == "approved")
							if err := db.ExecuteToggleUserSuspend(username, uNameLocal, isSuspending); err == nil {
								if subConsoleModal != nil {
									subConsoleModal.Hide()
								}
								renderDirectoryRows(userSearchBox.Text)
							}
						})

						newPassEntry := widget.NewPasswordEntry()
						newPassEntry.SetPlaceHolder("Enter unique override password...")
						resetPassBtn := widget.NewButtonWithIcon("Apply Password Override", theme.DocumentCreateIcon(), func() {
							if newPassEntry.Text != "" {
								if err := db.ExecuteResetUserPassword(username, uNameLocal, newPassEntry.Text); err == nil {
									dialog.ShowInformation("Reset Done", "Password hash has been overwritten.", window)
									newPassEntry.SetText("")
								}
							}
						})

						terminateSessionBtn := widget.NewButtonWithIcon("Terminate Session & Clear Lock", theme.CancelIcon(), func() {
							if err := db.ExecuteTerminateUserSession(username, uNameLocal); err == nil {
								if subConsoleModal != nil {
									subConsoleModal.Hide()
								}
								renderDirectoryRows(userSearchBox.Text)
							}
						})

						modalLayout := container.NewVBox(
							container.NewHBox(
								previewWrapper, // Live interactive picture slot
								widget.NewLabelWithStyle(fmt.Sprintf("Target User Account: %s", uNameLocal), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
							),
							widget.NewSeparator(),
							widget.NewLabel("Modify Security Authorization Level:"),
							roleDropdown,
							widget.NewLabel("Assign Profile Avatar Concept:"),
							avatarDropdown,
							saveRoleBtn,
							widget.NewSeparator(),
							widget.NewLabel("Account Access State Administration:"),
							toggleSuspendBtn,
							widget.NewSeparator(),
							widget.NewLabel("Administrative Password Overwrite:"),
							newPassEntry, resetPassBtn,
							widget.NewSeparator(),
							widget.NewLabel("Live Concurrent Session Terminations:"),
							terminateSessionBtn,
						)
						subConsoleModal = dialog.NewCustom("Operator Context Panel", "Close", modalLayout, window)
						subConsoleModal.Resize(fyne.NewSize(520, 560))
						subConsoleModal.Show()
					})

					userListContainer.Add(container.NewHBox(
						resolveProfileAvatar(currentAvatarLocal, currentRoleLocal, 32, 32),
						container.NewGridWrap(fyne.NewSize(140, 36), widget.NewLabel(uNameLocal)),
						container.NewGridWrap(fyne.NewSize(110, 36), widget.NewLabel(currentRoleLocal)),
						container.NewGridWrap(fyne.NewSize(110, 36), widget.NewLabel(currentStatusLocal)),
						container.NewGridWrap(fyne.NewSize(200, 36), widget.NewLabel(lastLoginTextLocal)),
						container.NewGridWrap(fyne.NewSize(140, 36), manageAccountBtn),
					))
				}
			}
			userSearchBox.OnChanged = func(t string) { renderDirectoryRows(t) }
			renderDirectoryRows("")
			scrollPanel := container.NewScroll(userListContainer)
			scrollPanel.SetMinSize(fyne.NewSize(780, 360))
			d := dialog.NewCustom("System Operator Registry Directory", "Close Profile Portal", container.NewBorder(userSearchBox, nil, nil, nil, scrollPanel), window)
			trackDialog(d)
			d.Show()

		case "Pending Registration Requests":
			if !canManageUsers {
				dialog.ShowError(fmt.Errorf("Administrative boundaries constraint exception."), window)
				return
			}
			pendingContainer := container.NewVBox()

			var refreshPendingQueue func()
			refreshPendingQueue = func() {
				pendingContainer.Objects = nil
				totalPending := db.FetchPendingUserCount()
				pendingContainer.Add(widget.NewLabelWithStyle(fmt.Sprintf("📋 Total Pending Registrations: %d", totalPending), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
				pendingContainer.Add(widget.NewSeparator())

				list, err := db.FetchPendingUsers()
				if err != nil || len(list) == 0 {
					pendingContainer.Add(widget.NewLabel("No registration requests currently awaiting terminal verification."))
					return
				}

				for _, targetAccount := range list {
					targetAccountLocal := targetAccount
					roleSelection := widget.NewSelect([]string{"user", "operator", "admin"}, nil)
					roleSelection.SetSelected("user")

					approveBtn := widget.NewButtonWithIcon("Approve Access", theme.ConfirmIcon(), func() {
						if err := db.ExecuteProcessApproval(username, targetAccountLocal, roleSelection.Selected, true); err == nil {
							refreshPendingQueue()
						}
					})
					approveBtn.Importance = widget.HighImportance

					rejectBtn := widget.NewButtonWithIcon("Reject/Purge", theme.CancelIcon(), func() {
						if err := db.ExecuteProcessApproval(username, targetAccountLocal, "", false); err == nil {
							refreshPendingQueue()
						}
					})

					pendingContainer.Add(container.NewHBox(
						container.NewGridWrap(fyne.NewSize(160, 36), widget.NewLabel(targetAccountLocal)),
						widget.NewLabel("Assign Role:"),
						container.NewGridWrap(fyne.NewSize(110, 36), roleSelection),
						approveBtn,
						rejectBtn,
					))
				}
			}
			refreshPendingQueue()
			scroll := container.NewScroll(pendingContainer)
			scroll.SetMinSize(fyne.NewSize(650, 360))
			d := dialog.NewCustom("Pending Authorization Registration Queue", "Close Queue View", scroll, window)
			trackDialog(d)
			d.Show()

		case "Feature Permissions Overrides":
			targetScopeEntry := widget.NewEntry()
			targetScopeEntry.SetPlaceHolder("Enter target role or unique username...")
			scopeSelector := widget.NewSelect([]string{"Role Default: operator", "Role Default: user", "Custom Target Username Override"}, func(sel string) {
				if sel == "Role Default: operator" {
					targetScopeEntry.SetText("operator")
					targetScopeEntry.Disable()
				}
				if sel == "Role Default: user" {
					targetScopeEntry.SetText("user")
					targetScopeEntry.Disable()
				}
				if sel == "Custom Target Username Override" {
					targetScopeEntry.SetText("")
					targetScopeEntry.Enable()
				}
			})
			scopeSelector.SetSelected("Role Default: operator")

			wCheck := widget.NewCheck("Enable database operational Write/Update profiles access capabilities", nil)
			pCheck := widget.NewCheck("Enable subscriber registration and system security access PIN creations", nil)
			vCheck := widget.NewCheck("Enable DDO Information Lookup Directories panels access bounds", nil)

			loadTargetRulesBtn := widget.NewButtonWithIcon("Query Scope Profile Rules", theme.SearchIcon(), func() {
				scope := targetScopeEntry.Text
				if scope == "" {
					return
				}
				wCheck.SetChecked(db.EvaluateUserPermission(scope, scope, "can_write", scope == "operator"))
				pCheck.SetChecked(db.EvaluateUserPermission(scope, scope, "can_assign_pin", false))
				vCheck.SetChecked(db.EvaluateUserPermission(scope, scope, "can_view_ddo", true))
			})

			var permFormDialog dialog.Dialog

			savePermsBtn := widget.NewButtonWithIcon("Save Authorization Bounds Rules", theme.DocumentSaveIcon(), func() {
				scope := targetScopeEntry.Text
				if scope == "" {
					return
				}
				_ = db.ExecuteSavePermissionToggle(username, scope, "can_write", wCheck.Checked)
				_ = db.ExecuteSavePermissionToggle(username, scope, "can_assign_pin", pCheck.Checked)
				_ = db.ExecuteSavePermissionToggle(username, scope, "can_view_ddo", vCheck.Checked)

				if permFormDialog != nil {
					permFormDialog.Hide()
				}
				dialog.ShowInformation("Matrix Updated", "Granular application credentials overwrites deployed successfully.", window)
			})
			savePermsBtn.Importance = widget.HighImportance

			modalContent := container.NewVBox(
				widget.NewLabelWithStyle("⚙️ Dynamic Granular Matrix Rule Setup Portal Context", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: true}),
				widget.NewLabel("Define permissions targeting roles universally, or specify isolated individual usernames for dynamic custom overrides:"),
				scopeSelector, targetScopeEntry, loadTargetRulesBtn, widget.NewSeparator(),
				wCheck, pCheck, vCheck, widget.NewSeparator(), savePermsBtn,
			)
			permFormDialog = dialog.NewCustom("Permissions Administration", "Close Console", modalContent, window)
			permFormDialog.Resize(fyne.NewSize(580, 440))
			trackDialog(permFormDialog)
			permFormDialog.Show()

		case "App Distribution Version Setup":
			currentVer, currentPath, err := db.FetchLatestAppVersion()
			if err != nil {
				dialog.ShowError(err, window)
				return
			}

			verEntry := widget.NewEntry()
			verEntry.SetText(currentVer)
			pathEntry := widget.NewEntry()
			pathEntry.SetText(currentPath)

			formItems := []*widget.FormItem{
				widget.NewFormItem("Global Target Version String", verEntry),
				widget.NewFormItem("Deployment Download/Shared Path", pathEntry),
			}

			var versionDialog dialog.Dialog
			versionDialog = dialog.NewForm("Modify Distribution Setup Metadata", "Publish Update Rules", "Cancel", formItems, func(confirmed bool) {
				if confirmed {
					if verEntry.Text == "" || pathEntry.Text == "" {
						return
					}
					if err := db.ExecuteUpdateAppConfig(verEntry.Text, pathEntry.Text); err == nil {
						dialog.ShowInformation("Success", "Application deployment rules published successfully.", window)
					}
				}
			}, window)
			trackDialog(versionDialog)
			versionDialog.Show()
		}
	})
	systemToolsMenu.Selected = "[System Tools Portal Menu]"
	systemToolsSizer := container.NewGridWrap(fyne.NewSize(260, 36), systemToolsMenu)

	controlButtonsBar := container.NewHBox(
		widget.NewLabelWithStyle("🕹️ Control Operations Center:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		actionMenuSizer,
		systemToolsSizer,
	)

	logoutButton := widget.NewButtonWithIcon("Logout Session", theme.LogoutIcon(), func() {
		resetInactivityTimer()
		dialog.ShowConfirm("End Active Session?", "Are you sure you want to log out?", func(confirmed bool) {
			if confirmed {
				activityLock.Lock()
				isSessionActive = false
				activityLock.Unlock()
				clearAllActiveDialogs()
				gridData = [][]string{gridData[0]}
				selectedRowIndex = -1
				RenderLoginView(app, window)
			}
		}, window)
	})

	dashLogo := canvas.NewImageFromFile("assets/logo.png")
	dashLogo.FillMode = canvas.ImageFillContain
	dashLogo.SetMinSize(fyne.NewSize(75, 75))

	// Fetch current user's profile avatar selection to render the correct header card item
	myAvatarSelection := "default"
	if operators, err := db.FetchSystemUsers(username); err == nil && len(operators) > 0 {
		myAvatarSelection = operators[0][5]
	}

	profileText := fmt.Sprintf("Operator ID: %s\nClearance: Secure %s\nSession Start: %s", username, role, lastLogin)

	profileHeaderLayout := container.NewHBox(
		resolveProfileAvatar(myAvatarSelection, role, 52, 52),
		widget.NewLabel(profileText),
	)
	profileCard := widget.NewCard("eGPF Operational Core Enterprise Dashboard", "Secure Profile Scope", profileHeaderLayout)

	headerLayout := container.NewBorder(nil, nil, nil, container.NewHBox(container.NewPadded(dashLogo), logoutButton), profileCard)

	dashboardFooter := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle(fmt.Sprintf("System Registry Core v%s\n%s", CurrentClientVersion, CopyrightInfo), fyne.TextAlignCenter, fyne.TextStyle{Italic: true}),
	)

	dashboardView := container.NewBorder(
		container.NewVBox(headerLayout, controlButtonsBar, widget.NewSeparator(), searchBarLayout),
		dashboardFooter,
		nil, nil,
		container.NewPadded(dataTable),
	)

	window.SetContent(dashboardView)
}
