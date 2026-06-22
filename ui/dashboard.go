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
	CurrentClientVersion   = "2.2.0.2"
	CopyrightInfo          = "© 2026 O/o the Accountant General (A&E), Tripura. \nAll Rights Reserved."
	SessionInactivityLimit = 5 * time.Minute
)

var selectedRowIndex int = -1

func LaunchOperationalDashboard(app fyne.App, window fyne.Window, username string, role string, lastLogin string) {
	window.Resize(fyne.NewSize(1280, 720))
	window.CenterOnScreen()

	gridData := [][]string{{
		"Series ID & Name", "Account Number", "Subscriber Name",
		"Employee Code", "Beneficiary Code", "Designation",
		"Mobile No", "Date of Birth", "DDO Code", "PIN",
	}}
	selectedRowIndex = -1

	// DYNAMIC PER-USER & PER-ROLE PERMISSION EVALUATIONS
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
	} else {
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

	generateRandom4DigitPIN := func() string {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		return fmt.Sprintf("%04d", r.Intn(10000))
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

	systemToolsMenu := widget.NewSelect([]string{"[System Tools Portal Menu]", "DDO Master Directory Registry", "Add New Fund Series Category", "Manage Local User Profiles", "Feature Permissions Overrides", "App Distribution Version Setup"}, func(tool string) {
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
			ddoListContainer := container.NewVBox()
			ddoSearchBox := widget.NewEntry()
			ddoSearchBox.SetPlaceHolder("Filter DDO Master...")
			createCell := func(text string, bold bool, w float32) fyne.CanvasObject {
				lbl := widget.NewLabel(text)
				lbl.Wrapping = fyne.TextWrapWord
				if bold {
					lbl.TextStyle = fyne.TextStyle{Bold: true}
				}
				return container.NewGridWrap(fyne.NewSize(w, 42), lbl)
			}
			renderDdoRows := func(filter string) {
				ddoListContainer.Objects = nil
				profiles, _ := db.FetchAllDDOMasterProfiles(filter)
				ddoListContainer.Add(container.NewHBox(createCell("DDO Code", true, 90), createCell("Designation", true, 220), createCell("Phone", true, 120), createCell("Email", true, 220), createCell("Gate PIN", true, 90)))
				for _, p := range profiles {
					pinText := p[6]
					if role != "admin" {
						pinText = "****"
					}
					ddoListContainer.Add(container.NewHBox(createCell(p[0], false, 90), createCell(p[1], false, 220), createCell(p[2], false, 120), createCell(p[3], false, 220), createCell(pinText, false, 90)))
				}
			}
			ddoSearchBox.OnChanged = renderDdoRows
			renderDdoRows("")
			scroll := container.NewScroll(ddoListContainer)
			scroll.SetMinSize(fyne.NewSize(800, 400))
			d := dialog.NewCustom("DDO Master Registry", "Close", container.NewBorder(ddoSearchBox, nil, nil, nil, scroll), window)
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
					container.NewGridWrap(fyne.NewSize(140, 32), widget.NewLabelWithStyle("Username", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
					container.NewGridWrap(fyne.NewSize(110, 32), widget.NewLabelWithStyle("Clearance", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
					container.NewGridWrap(fyne.NewSize(110, 32), widget.NewLabelWithStyle("Status", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
					container.NewGridWrap(fyne.NewSize(150, 32), widget.NewLabelWithStyle("Session Footprint", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
					container.NewGridWrap(fyne.NewSize(140, 32), widget.NewLabelWithStyle("Action Controls", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
				))
				userListContainer.Add(widget.NewSeparator())

				for _, uData := range users {
					uNameLocal := uData[0]
					currentRoleLocal := uData[1]
					lastLoginTextLocal := uData[2]
					currentStatusLocal := uData[3]

					// ATTACHED OPERATIONAL ACCOUNT WORKFLOW MENUS
					manageAccountBtn := widget.NewButtonWithIcon("Manage Account", theme.SettingsIcon(), func() {
						resetInactivityTimer()
						var subConsoleModal dialog.Dialog

						roleDropdown := widget.NewSelect([]string{"user", "operator", "admin"}, nil)
						roleDropdown.SetSelected(currentRoleLocal)

						saveRoleBtn := widget.NewButtonWithIcon("Save Role Changes", theme.DocumentSaveIcon(), func() {
							if uNameLocal == username {
								dialog.ShowError(fmt.Errorf("You cannot alter your own role clearance level"), window)
								return
							}
							if err := db.ExecuteUpdateUserRole(username, uNameLocal, roleDropdown.Selected); err == nil {
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
							widget.NewLabelWithStyle(fmt.Sprintf("Target User Account: %s", uNameLocal), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
							widget.NewSeparator(),
							widget.NewLabel("Modify Security Authorization Level:"),
							container.NewHBox(roleDropdown, saveRoleBtn),
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
						subConsoleModal.Resize(fyne.NewSize(520, 480))
						subConsoleModal.Show()
					})

					userListContainer.Add(container.NewHBox(
						container.NewGridWrap(fyne.NewSize(140, 36), widget.NewLabel(uNameLocal)),
						container.NewGridWrap(fyne.NewSize(110, 36), widget.NewLabel(currentRoleLocal)),
						container.NewGridWrap(fyne.NewSize(110, 36), widget.NewLabel(currentStatusLocal)),
						container.NewGridWrap(fyne.NewSize(150, 36), widget.NewLabel(lastLoginTextLocal)),
						container.NewGridWrap(fyne.NewSize(140, 36), manageAccountBtn),
					))
				}
			}
			userSearchBox.OnChanged = func(t string) { renderDirectoryRows(t) }
			renderDirectoryRows("")
			scrollPanel := container.NewScroll(userListContainer)
			scrollPanel.SetMinSize(fyne.NewSize(720, 360))
			d := dialog.NewCustom("System Operator Registry Directory", "Close Profile Portal", container.NewBorder(userSearchBox, nil, nil, nil, scrollPanel), window)
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

	profileText := fmt.Sprintf("Operator ID: %s\nClearance: Secure %s\nSession Start: %s", username, role, lastLogin)
	profileCard := widget.NewCard("eGPF Operational Core Enterprise Dashboard", "Secure Profile Scope", widget.NewLabel(profileText))

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
