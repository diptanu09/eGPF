package ui

import (
	"fmt"
	"math/rand"
	"net/url"
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
	SessionInactivityLimit = 5 * time.Minute // ⏱️ AUTOMATIC SESSION TIMEOUT BOUNDARY DEFINITION
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

	// Fetch database state dynamic rule set definitions configuration parameters map
	sysPerms, _ := db.FetchApplicationPermissions()

	// Dynamic validation evaluations mapping security credentials variables
	canWrite := (role == "admin" || (role == "operator" && sysPerms["operator_can_write"]))
	canDelete := (role == "admin")
	canManageUsers := (role == "admin")
	canAssignSubscriberPin := (role == "admin" || (role == "operator" && sysPerms["operator_can_assign_pin"]))
	canViewDdoData := (role == "admin" || role == "operator" || (role == "user" && sysPerms["user_can_view_ddo"]))

	// 🔒 Thread-safe tracking of open overlay components to clear windows completely on timeout
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

	// Background lifecycle routine mapping active operational execution timelines
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

	go func() {
		srvVersion, dlPath, err := db.FetchLatestAppVersion()
		if err == nil && srvVersion != CurrentClientVersion {
			dialog.ShowConfirm(
				"⚠️ System Update Available",
				fmt.Sprintf("A newer version of this application is available!\n\nInstalled Version: %s\nLatest Version: %s\n\nWould you like to download the update now?", CurrentClientVersion, srvVersion),
				func(confirm bool) {
					if confirm {
						parsedURL, err := url.Parse(dlPath)
						if err == nil {
							_ = app.OpenURL(parsedURL)
						}
						window.Close()
					} else {
						reqDialog := dialog.NewInformation(
							"🚫 Mandatory Update Required",
							"This update is required to maintain secure synchronization with the network registry.\n\nYou cannot continue using this terminal session on an outdated version.\n\nYou will be logged out automatically.",
							window,
						)
						reqDialog.SetOnClosed(func() {
							activityLock.Lock()
							isSessionActive = false
							activityLock.Unlock()

							clearAllActiveDialogs()

							gridData = [][]string{{
								"Series ID & Name", "Account Number", "Subscriber Name",
								"Employee Code", "Beneficiary Code", "Designation",
								"Mobile No", "Date of Birth", "DDO Code", "PIN",
							}}
							selectedRowIndex = -1
							RenderLoginView(app, window)
						})
						reqDialog.Show()
					}
				},
				window,
			)
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
		widget.NewLabelWithStyle("🔍 Registry Search:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(1),
		widget.NewLabel("Series:"),
		seriesSizer,
		widget.NewLabel("Account No:"),
		accountSizer,
		widget.NewLabel("Name:"),
		nameSizer,
		searchButton,
		clearButton,
	)

	actionPanel := container.NewHBox()

	if canViewDdoData {
		actionPanel.Add(widget.NewButtonWithIcon("View DDO Info", theme.InfoIcon(), func() {
			resetInactivityTimer()
			if selectedRowIndex == -1 {
				dialog.ShowInformation("Selection Required", "Please click an active subscriber row in the data grid to resolve DDO metadata details.", window)
				return
			}

			ddoCodeTarget := gridData[selectedRowIndex][8]
			if ddoCodeTarget == "" || ddoCodeTarget == "N/A" || ddoCodeTarget == "0" {
				dialog.ShowError(fmt.Errorf("Identity Mapping Exception:\nThe highlighted subscriber record space does not hold an active DDO assignment filter link"), window)
				return
			}

			ddoInfo, err := db.FetchDDODetails(ddoCodeTarget)
			if err != nil {
				dialog.ShowError(err, window)
				return
			}

			displayPin := ddoInfo["pin"]
			if role != "admin" {
				displayPin = "[REDACTED - ADMIN CLEARANCE REQUIRED]"
			}

			infoContent := container.NewVBox(
				widget.NewLabelWithStyle(fmt.Sprintf("DDO Master Code: %s", ddoInfo["ddo_code"]), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewSeparator(),
				widget.NewLabel(fmt.Sprintf("Official Designation: %s", ddoInfo["ddo_desg"])),
				widget.NewLabel(fmt.Sprintf("Contact Phone No: %s", ddoInfo["ddo_phone"])),
				widget.NewLabel(fmt.Sprintf("Secure Network Email: %s", ddoInfo["ddo_email"])),
				widget.NewLabel(fmt.Sprintf("Treasury Node Office Code: %s", ddoInfo["ddo_tres_code"])),
				widget.NewLabel(fmt.Sprintf("VLC Assigned Code Matrix: %s", ddoInfo["vlc_ddo_code"])),
				widget.NewLabelWithStyle(fmt.Sprintf("Gate Authorization PIN String: %s", displayPin), fyne.TextAlignLeading, fyne.TextStyle{Italic: true, Bold: role == "admin"}),
			)

			ddoModal := dialog.NewCustom("Drawing and Disbursing Officer (DDO) Master Reference", "Close View", infoContent, window)
			ddoModal.Resize(fyne.NewSize(480, 320))
			trackDialog(ddoModal)
			ddoModal.Show()
		}))

		actionPanel.Add(widget.NewButtonWithIcon("DDO Master Directory", theme.SearchIcon(), func() {
			resetInactivityTimer()
			ddoListContainer := container.NewVBox()
			ddoSearchBox := widget.NewEntry()
			ddoSearchBox.SetPlaceHolder("Enter criteria to search DDO Master by Code...")

			createCustomCell := func(text string, style fyne.TextStyle, minWidth float32) fyne.CanvasObject {
				lbl := widget.NewLabelWithStyle(text, fyne.TextAlignLeading, style)
				lbl.Wrapping = fyne.TextWrapWord
				return container.NewGridWrap(fyne.NewSize(minWidth, 48), lbl)
			}

			renderDdoDirectoryRows := func(filter string) {
				ddoListContainer.Objects = nil
				ddoProfiles, err := db.FetchAllDDOMasterProfiles(filter)
				if err != nil {
					ddoListContainer.Add(widget.NewLabel("System error loading DDO Master records registry."))
					return
				}

				if len(ddoProfiles) == 0 {
					ddoListContainer.Add(widget.NewLabel("No DDO Master registry configurations match search criteria."))
					ddoListContainer.Refresh()
					return
				}

				headerRow := container.NewHBox(
					createCustomCell("DDO Code", fyne.TextStyle{Bold: true}, 90),
					createCustomCell("Official Designation", fyne.TextStyle{Bold: true}, 260),
					createCustomCell("Phone Number", fyne.TextStyle{Bold: true}, 130),
					createCustomCell("Network Email", fyne.TextStyle{Bold: true}, 260),
					createCustomCell("Treasury Code", fyne.TextStyle{Bold: true}, 120),
					createCustomCell("VLC Code", fyne.TextStyle{Bold: true}, 110),
					createCustomCell("Gate PIN", fyne.TextStyle{Bold: true}, 90),
				)
				ddoListContainer.Add(headerRow)
				ddoListContainer.Add(widget.NewSeparator())

				for _, p := range ddoProfiles {
					rowPin := p[6]
					if role != "admin" {
						rowPin = "****"
					}

					rowLayout := container.NewHBox(
						createCustomCell(p[0], fyne.TextStyle{}, 90),
						createCustomCell(p[1], fyne.TextStyle{}, 260),
						createCustomCell(p[2], fyne.TextStyle{}, 130),
						createCustomCell(p[3], fyne.TextStyle{}, 260),
						createCustomCell(p[4], fyne.TextStyle{}, 120),
						createCustomCell(p[5], fyne.TextStyle{}, 110),
						createCustomCell(rowPin, fyne.TextStyle{Bold: role == "admin"}, 90),
					)
					ddoListContainer.Add(rowLayout)
				}
				ddoListContainer.Refresh()
			}

			ddoSearchBox.OnChanged = func(term string) {
				resetInactivityTimer()
				renderDdoDirectoryRows(term)
			}
			renderDdoDirectoryRows("")

			scrollPanel := container.NewScroll(ddoListContainer)
			scrollPanel.SetMinSize(fyne.NewSize(1100, 420))

			portalContent := container.NewBorder(
				container.NewVBox(
					widget.NewLabelWithStyle("🏢 Drawing & Disbursing Officer (DDO) Master Registry Console", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: true}),
					ddoSearchBox,
					widget.NewSeparator(),
				),
				nil, nil, nil,
				scrollPanel,
			)
			ddoDirModal := dialog.NewCustom("Enterprise DDO Master Information Portal", "Close Directory View", portalContent, window)
			ddoDirModal.Resize(fyne.NewSize(1150, 540))
			trackDialog(ddoDirModal)
			ddoDirModal.Show()
		}))
	}

	if canAssignSubscriberPin {
		actionPanel.Add(widget.NewButtonWithIcon("Assign Subscriber PIN", theme.SettingsIcon(), func() {
			resetInactivityTimer()

			generateRandom4DigitPIN := func() string {
				r := rand.New(rand.NewSource(time.Now().UnixNano()))
				return fmt.Sprintf("%04d", r.Intn(10000))
			}

			if selectedRowIndex == -1 {
				newAccountNoEntry := widget.NewEntry()
				newAccountNoEntry.SetPlaceHolder("Enter numeric subscriber account no... (e.g. 54321)")

				seriesSelectDropdown := widget.NewSelect(seriesOptions, nil)
				seriesSelectDropdown.PlaceHolder = "[Select Master Series Category]"

				initialPinField := widget.NewEntry()
				initialPinField.SetPlaceHolder("Assign or auto-generate initial access PIN...")

				autoGenBtn := widget.NewButtonWithIcon("Auto-Generate 4-Digit PIN", theme.ViewRefreshIcon(), func() {
					resetInactivityTimer()
					initialPinField.SetText(generateRandom4DigitPIN())
				})

				formItems := []*widget.FormItem{
					widget.NewFormItem("Choose Master Fund Series", seriesSelectDropdown),
					widget.NewFormItem("New Account Number Input", newAccountNoEntry),
					widget.NewFormItem("Initial Access Gate PIN Key", initialPinField),
					widget.NewFormItem("Automated Provisioning", autoGenBtn),
				}

				createSubscriberDialog := dialog.NewForm("Register New System Subscriber Profile", "Create Subscriber Account", "Cancel", formItems, func(confirmed bool) {
					resetInactivityTimer()
					if confirmed {
						if seriesSelectDropdown.Selected == "" || newAccountNoEntry.Text == "" || initialPinField.Text == "" {
							dialog.ShowError(fmt.Errorf("Validation Failure Exception:\nAll registration payload parameters must be filled completely"), window)
							return
						}

						resolvedSeriesID := seriesMap[seriesSelectDropdown.Selected]
						err := db.ExecuteCreateNewSubscriber(username, resolvedSeriesID, newAccountNoEntry.Text, initialPinField.Text)
						if err == nil {
							dialog.ShowInformation("Registration Complete", fmt.Sprintf("Successfully registered new system subscriber account profile!\nSeries ID: %s\nAccount No: %s", resolvedSeriesID, newAccountNoEntry.Text), window)
							syncUiViewGrid(resolvedSeriesID, newAccountNoEntry.Text, "", false)
						} else {
							dialog.ShowError(err, window)
						}
					}
				}, window)

				createSubscriberDialog.Resize(fyne.NewSize(500, 310))
				trackDialog(createSubscriberDialog)
				createSubscriberDialog.Show()
				return
			}

			rawSeriesStr := gridData[selectedRowIndex][0]
			var parsedSeriesID string
			fmt.Sscanf(rawSeriesStr, "%s", &parsedSeriesID)
			targetAccountNo := gridData[selectedRowIndex][1]
			targetName := gridData[selectedRowIndex][2]

			pinEntryField := widget.NewEntry()
			pinEntryField.SetPlaceHolder("Enter or generate new alphanumeric or numeric access PIN...")

			autoGenResetBtn := widget.NewButtonWithIcon("Auto-Generate 4-Digit PIN", theme.ViewRefreshIcon(), func() {
				resetInactivityTimer()
				pinEntryField.SetText(generateRandom4DigitPIN())
			})

			formItems := []*widget.FormItem{
				widget.NewFormItem("Subscriber Name [Locked]", widget.NewLabel(targetName)),
				widget.NewFormItem("Account No [Locked]", widget.NewLabel(targetAccountNo)),
				widget.NewFormItem("New Gate PIN String", pinEntryField),
				widget.NewFormItem("Automated Provisioning", autoGenResetBtn),
			}

			pinResetDialog := dialog.NewForm("Modify Subscriber Portal Gate PIN", "Assign New PIN", "Cancel", formItems, func(confirmed bool) {
				resetInactivityTimer()
				if confirmed {
					if pinEntryField.Text == "" {
						dialog.ShowError(fmt.Errorf("Validation Error: Subscriber PIN payload string constraint cannot be left empty"), window)
						return
					}

					err := db.ExecuteResetSubscriberPIN(username, parsedSeriesID, targetAccountNo, pinEntryField.Text)
					if err == nil {
						dialog.ShowInformation("Execution Complete", fmt.Sprintf("Successfully assigned new synchronization gate access token credentials for: %s", targetName), window)
						syncUiViewGrid(parsedSeriesID, targetAccountNo, "", false)
					} else {
						dialog.ShowError(fmt.Errorf("Database Rejection Error: %v", err), window)
					}
				}
			}, window)

			pinResetDialog.Resize(fyne.NewSize(480, 290))
			trackDialog(pinResetDialog)
			pinResetDialog.Show()
		}))
	}

	if role == "admin" {
		actionPanel.Add(widget.NewButtonWithIcon("Add New Series", theme.ContentAddIcon(), func() {
			resetInactivityTimer()
			seriesIDEntry := widget.NewEntry()
			seriesIDEntry.SetPlaceHolder("e.g., 12")

			seriesNameEntry := widget.NewEntry()
			seriesNameEntry.SetPlaceHolder("e.g., PWD-TRIPURA")

			formItems := []*widget.FormItem{
				widget.NewFormItem("Series ID", seriesIDEntry),
				widget.NewFormItem("Series Name", seriesNameEntry),
			}

			seriesDialog := dialog.NewForm("Configure New System Series", "Save Series Definition", "Cancel", formItems, func(confirmed bool) {
				resetInactivityTimer()
				if confirmed {
					if seriesIDEntry.Text == "" || seriesNameEntry.Text == "" {
						dialog.ShowError(fmt.Errorf("Validation Exception:\nSeries components cannot be left empty"), window)
						return
					}

					err := db.ExecuteInsertSeries(seriesIDEntry.Text, seriesNameEntry.Text)
					if err == nil {
						dialog.ShowInformation("Series Added", fmt.Sprintf("Successfully registered series profile: %s - %s", seriesIDEntry.Text, seriesNameEntry.Text), window)

						if freshOptions, freshMap, err := db.FetchSeriesDropdownOptions(); err == nil {
							seriesOptions = freshOptions
							seriesMap = freshMap
							sort.Slice(seriesOptions, func(i, j int) bool {
								var id1, id2 int
								fmt.Sscanf(seriesOptions[i], "%d", &id1)
								fmt.Sscanf(seriesOptions[j], "%d", &id2)
								return id1 < id2
							})

							searchSeriesSelect.Options = append([]string{"[All Series Categories]"}, seriesOptions...)
							searchSeriesSelect.Refresh()
						}
					} else {
						dialog.ShowError(fmt.Errorf("Database Execution Error: %v", err), window)
					}
				}
			}, window)
			trackDialog(seriesDialog)
			seriesDialog.Show()
		}))
	}

	if canWrite {
		actionPanel.Add(widget.NewButtonWithIcon("Update Entry", theme.DocumentCreateIcon(), func() {
			resetInactivityTimer()
			if selectedRowIndex == -1 {
				dialog.ShowInformation("Selection Required", "Please click a row in the data grid to update.", window)
				return
			}

			rawSeriesStr := gridData[selectedRowIndex][0]
			var oldSerID string
			fmt.Sscanf(rawSeriesStr, "%s", &oldSerID)
			oldAccNo := gridData[selectedRowIndex][1]

			seriesLabel := widget.NewLabel(rawSeriesStr)
			accLabel := widget.NewLabel(oldAccNo)

			designationEntry := widget.NewEntry()
			designationEntry.SetText(gridData[selectedRowIndex][5])

			mobileEntry := widget.NewEntry()
			mobileEntry.SetText(gridData[selectedRowIndex][6])

			pinEntry := widget.NewEntry()
			pinEntry.SetText(gridData[selectedRowIndex][9])

			items := []*widget.FormItem{
				widget.NewFormItem("Series Target [Locked]", seriesLabel),
				widget.NewFormItem("Account Number [Locked]", accLabel),
				widget.NewFormItem("Designation", designationEntry),
				widget.NewFormItem("Mobile No.", mobileEntry),
				widget.NewFormItem("PIN Code", pinEntry),
			}
			updateDialog := dialog.NewForm("Modify Subscriber Data", "Apply Changes", "Cancel", items, func(confirmed bool) {
				resetInactivityTimer()
				if confirmed {
					err := db.ExecuteUpdateRecord(
						username, oldSerID, oldAccNo, pinEntry.Text,
						mobileEntry.Text, designationEntry.Text, oldSerID, oldAccNo,
					)
					if err == nil {
						syncUiViewGrid(selectedSearchSeriesID, searchAccountBox.Text, searchNameBox.Text, false)
					} else {
						dialog.ShowError(err, window)
					}
				}
			}, window)
			trackDialog(updateDialog)
			updateDialog.Show()
		}))
	}

	if canDelete {
		actionPanel.Add(widget.NewButtonWithIcon("Delete Entry", theme.DeleteIcon(), func() {
			resetInactivityTimer()
			if selectedRowIndex == -1 {
				dialog.ShowInformation("Selection Required", "Please click a data row below to delete.", window)
				return
			}
			rawSeriesStr := gridData[selectedRowIndex][0]
			var targetSerID string
			fmt.Sscanf(rawSeriesStr, "%s", &targetSerID)
			targetAccNo := gridData[selectedRowIndex][1]
			confirmMsg := fmt.Sprintf("Permanently drop subscriber record?\nSeries ID Target: %s\nAccount: %s", targetSerID, targetAccNo)

			deleteDialog := dialog.NewConfirm("Confirm Drop Action", confirmMsg, func(confirmed bool) {
				resetInactivityTimer()
				if confirmed {
					if err := db.ExecuteDeleteRecord(username, targetSerID, targetAccNo); err == nil {
						syncUiViewGrid(selectedSearchSeriesID, searchAccountBox.Text, searchNameBox.Text, false)
					} else {
						dialog.ShowError(err, window)
					}
				}
			}, window)
			trackDialog(deleteDialog)
			deleteDialog.Show()
		}))
	}

	if canManageUsers {
		actionPanel.Add(widget.NewButtonWithIcon("Add System User", theme.AccountIcon(), func() {
			resetInactivityTimer()
			userEntry := widget.NewEntry()
			userEntry.SetPlaceHolder("Enter unique username")
			passEntry := widget.NewPasswordEntry()
			passEntry.SetPlaceHolder("Enter access password")

			roleSelect := widget.NewSelect([]string{"user", "operator", "admin"}, nil)
			roleSelect.SetSelected("user")

			formItems := []*widget.FormItem{
				widget.NewFormItem("New Username", userEntry),
				widget.NewFormItem("Secure Password", passEntry),
				widget.NewFormItem("Clearance Role", roleSelect),
			}

			addUserDialog := dialog.NewForm("Provision New System Terminal User", "Create User", "Cancel", formItems, func(confirmed bool) {
				resetInactivityTimer()
				if confirmed {
					if userEntry.Text == "" || passEntry.Text == "" {
						dialog.ShowError(fmt.Errorf("Validation Error: Fields cannot be left empty"), window)
						return
					}
					err := db.ExecuteInsertUser(username, userEntry.Text, passEntry.Text, roleSelect.Selected)
					if err == nil {
						msg := fmt.Sprintf("Success: Profile assigned for '%s' with role clearance [%s].", userEntry.Text, roleSelect.Selected)
						dialog.ShowInformation("Identity Registered", msg, window)
					} else {
						dialog.ShowError(fmt.Errorf("Database Rejection: %v", err), window)
					}
				}
			}, window)
			trackDialog(addUserDialog)
			addUserDialog.Show()
		}))

		actionPanel.Add(widget.NewButtonWithIcon("User Directory & Roles", theme.SettingsIcon(), func() {
			resetInactivityTimer()
			userListContainer := container.NewVBox()
			userSearchBox := widget.NewEntry()
			userSearchBox.SetPlaceHolder("Search users by username...")

			var renderDirectoryRows func(string)
			renderDirectoryRows = func(filter string) {
				userListContainer.Objects = nil
				users, err := db.FetchSystemUsers(filter)
				if err != nil {
					userListContainer.Add(widget.NewLabel("Error reading system users directory."))
					return
				}

				if len(users) == 0 {
					userListContainer.Add(widget.NewLabel("No system users matched the search parameter."))
					userListContainer.Refresh()
					return
				}

				headerRow := container.NewGridWithColumns(5,
					widget.NewLabelWithStyle("Username", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabelWithStyle("Assigned Role", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabelWithStyle("Account Status", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabelWithStyle("Last Login Session", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabelWithStyle("Action Controls", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				)
				userListContainer.Add(headerRow)
				userListContainer.Add(widget.NewSeparator())

				for _, uData := range users {
					uNameLocal := uData[0]
					currentRole := uData[1]
					lastLoginText := uData[2]
					currentStatus := uData[3]
					terminalMacLock := uData[4]

					if currentStatus == "pending" {
						continue
					}

					displayUsername := uNameLocal
					if currentStatus == "suspended" {
						displayUsername = uNameLocal + " 🚫"
					}

					manageAccountBtn := widget.NewButtonWithIcon("Manage Account", theme.SettingsIcon(), func() {
						resetInactivityTimer()
						var contextOverlayModal dialog.Dialog

						roleDropdown := widget.NewSelect([]string{"user", "operator", "admin"}, nil)
						roleDropdown.SetSelected(currentRole)

						saveRoleBtn := widget.NewButtonWithIcon("Save Role Changes", theme.DocumentSaveIcon(), func() {
							resetInactivityTimer()
							if uNameLocal == username {
								dialog.ShowError(fmt.Errorf("Security Constraint: You cannot alter your own clearance level"), window)
								return
							}
							err := db.ExecuteUpdateUserRole(username, uNameLocal, roleDropdown.Selected)
							if err == nil {
								dialog.ShowInformation("Updated", fmt.Sprintf("User '%s' role is now [%s].", uNameLocal, roleDropdown.Selected), window)
								contextOverlayModal.Hide()
								renderDirectoryRows(userSearchBox.Text)
							} else {
								dialog.ShowError(err, window)
							}
						})

						suspendBtnText := "Suspend User Account"
						if currentStatus == "suspended" {
							suspendBtnText = "Reactivate User Account"
						}

						toggleSuspendBtn := widget.NewButtonWithIcon(suspendBtnText, theme.WarningIcon(), func() {
							resetInactivityTimer()
							if uNameLocal == username {
								dialog.ShowError(fmt.Errorf("Security Exception:\nYou cannot suspend your own administrative session context."), window)
								return
							}
							isSuspending := (currentStatus == "approved")
							err := db.ExecuteToggleUserSuspend(username, uNameLocal, isSuspending)
							if err == nil {
								actionWord := "activated"
								if isSuspending {
									actionWord = "suspended"
								}
								dialog.ShowInformation("Execution Complete", fmt.Sprintf("Identity status for '%s' updated to: %s", uNameLocal, actionWord), window)
								contextOverlayModal.Hide()
								renderDirectoryRows(userSearchBox.Text)
							} else {
								dialog.ShowError(err, window)
							}
						})
						if currentStatus == "approved" {
							toggleSuspendBtn.Importance = widget.HighImportance
						} else {
							toggleSuspendBtn.Importance = widget.MediumImportance
						}

						newPassEntry := widget.NewPasswordEntry()
						newPassEntry.SetPlaceHolder("Enter new complex password string...")

						resetPassBtn := widget.NewButtonWithIcon("Apply Password Override", theme.DocumentCreateIcon(), func() {
							resetInactivityTimer()
							if newPassEntry.Text == "" {
								dialog.ShowError(fmt.Errorf("Validation Exception:\nPassword input string constraint cannot be blank"), window)
								return
							}
							err := db.ExecuteResetUserPassword(username, uNameLocal, newPassEntry.Text)
							if err == nil {
								dialog.ShowInformation("Security Overwrite Done", fmt.Sprintf("Secure credential hash reassigned for user: %s", uNameLocal), window)
								newPassEntry.SetText("")
							} else {
								dialog.ShowError(err, window)
							}
						})

						terminateSessionBtn := widget.NewButtonWithIcon("Terminate Session & Clear Lock", theme.CancelIcon(), func() {
							resetInactivityTimer()
							dialog.ShowConfirm("Kill Core Session Context?", fmt.Sprintf("Force session logouts and unbind physical device locked signatures for '%s'?", uNameLocal), func(confirm bool) {
								resetInactivityTimer()
								if confirm {
									err := db.ExecuteTerminateUserSession(username, uNameLocal)
									if err == nil {
										dialog.ShowInformation("Session Flushed", fmt.Sprintf("All active environment sessions and matching system fingerprints dropped for '%s'.", uNameLocal), window)
										contextOverlayModal.Hide()
										renderDirectoryRows(userSearchBox.Text)
									} else {
										dialog.ShowError(err, window)
									}
								}
							}, window)
						})
						terminateSessionBtn.Importance = widget.LowImportance

						statusLabelText := fmt.Sprintf("Status Profile Context: %s\nHardware Signature Binding: %s", currentStatus, terminalMacLock)

						modalLayout := container.NewVBox(
							widget.NewLabelWithStyle(fmt.Sprintf("User Context ID Target: %s", uNameLocal), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
							widget.NewLabel(statusLabelText),
							widget.NewSeparator(),
							widget.NewLabel("1. Modify Security Authorization Level:"),
							container.NewHBox(roleDropdown, saveRoleBtn),
							widget.NewSeparator(),
							widget.NewLabel("2. Account Access State Administration:"),
							toggleSuspendBtn,
							widget.NewSeparator(),
							widget.NewLabel("3. Administrative Password Overwrite:"),
							newPassEntry,
							resetPassBtn,
							widget.NewSeparator(),
							widget.NewLabel("4. Live Concurrent Session Terminations:"),
							terminateSessionBtn,
						)

						contextOverlayModal = dialog.NewCustom(fmt.Sprintf("Administrative Management Console: %s", uNameLocal), "Close Console", modalLayout, window)
						contextOverlayModal.Resize(fyne.NewSize(520, 500))
						trackDialog(contextOverlayModal)
						contextOverlayModal.Show()
					})

					rowLayout := container.NewGridWithColumns(5,
						widget.NewLabel(displayUsername),
						widget.NewLabel(currentRole),
						widget.NewLabel(currentStatus),
						widget.NewLabel(lastLoginText),
						manageAccountBtn,
					)
					userListContainer.Add(rowLayout)
				}
				userListContainer.Refresh()
			}

			userSearchBox.OnChanged = func(term string) {
				resetInactivityTimer()
				renderDirectoryRows(term)
			}
			renderDirectoryRows("")

			scrollPanel := container.NewScroll(userListContainer)
			scrollPanel.SetMinSize(fyne.NewSize(880, 380))

			portalContent := container.NewBorder(
				container.NewVBox(
					widget.NewLabelWithStyle("👥 System Identity Registry & Role Directory Portal", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: true}),
					userSearchBox,
					widget.NewSeparator(),
				),
				nil, nil, nil,
				scrollPanel,
			)
			userDirDialog := dialog.NewCustom("Enterprise Master User Management Console", "Close View", portalContent, window)
			trackDialog(userDirDialog)
			userDirDialog.Show()
		}))

		actionPanel.Add(widget.NewButtonWithIcon("App Version Control", theme.InfoIcon(), func() {
			resetInactivityTimer()
			currentVer, currentPath, err := db.FetchLatestAppVersion()
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to retrieve current version configuration details: %v", err), window)
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
				resetInactivityTimer()
				if confirmed {
					if verEntry.Text == "" || pathEntry.Text == "" {
						dialog.ShowError(fmt.Errorf("Validation Exception:\nInputs cannot be saved blank."), window)
						return
					}
					err := db.ExecuteUpdateAppConfig(verEntry.Text, pathEntry.Text)
					if err == nil {
						dialog.ShowInformation("Configuration Deployed", fmt.Sprintf("Success: Active tracking rule set to version %s.\nOlder builds will prompt users to download updates on login.", verEntry.Text), window)
					} else {
						dialog.ShowError(fmt.Errorf("Database Rejection Error: %v", err), window)
					}
				}
			}, window)
			trackDialog(versionDialog)
			versionDialog.Show()
		}))

		// REFINED FIXED DIALOG ARGS: Configured explicit Save configurations callback via standalone dialog actions buttons layout definitions
		actionPanel.Add(widget.NewButtonWithIcon("Feature Permissions", theme.GridIcon(), func() {
			resetInactivityTimer()

			freshPerms, _ := db.FetchApplicationPermissions()

			opWriteCheck := widget.NewCheck("Allow 'operators' to Write/Update subscriber database entries", nil)
			opWriteCheck.SetChecked(freshPerms["operator_can_write"])

			opPinCheck := widget.NewCheck("Allow 'operators' to Provision/Assign Subscriber access PIN keys", nil)
			opPinCheck.SetChecked(freshPerms["operator_can_assign_pin"])

			userDdoCheck := widget.NewCheck("Allow basic 'users' role path access to View DDO master registries", nil)
			userDdoCheck.SetChecked(freshPerms["user_can_view_ddo"])

			var permFormDialog dialog.Dialog

			saveBtn := widget.NewButtonWithIcon("Save Configuration Rules", theme.DocumentSaveIcon(), func() {
				resetInactivityTimer()
				_ = db.ExecuteSavePermissionToggle(username, "operator_can_write", opWriteCheck.Checked)
				_ = db.ExecuteSavePermissionToggle(username, "operator_can_assign_pin", opPinCheck.Checked)
				_ = db.ExecuteSavePermissionToggle(username, "user_can_view_ddo", userDdoCheck.Checked)

				if permFormDialog != nil {
					permFormDialog.Hide()
				}
				dialog.ShowInformation("Permissions Saved", "Dynamic access rule adjustments deployed successfully.\n\nChanges will apply instantly across running terminal spaces.", window)
			})
			saveBtn.Importance = widget.HighImportance

			modalContent := container.NewVBox(
				widget.NewLabelWithStyle("🛡️ System Feature Authorizations Matrix Control Console", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: true}),
				widget.NewLabel("Check or uncheck features below to dynamically alter active operational capabilities for roles across the deployment:"),
				widget.NewSeparator(),
				opWriteCheck,
				widget.NewSeparator(),
				opPinCheck,
				widget.NewSeparator(),
				userDdoCheck,
				widget.NewSeparator(),
				saveBtn,
			)

			permFormDialog = dialog.NewCustom("System Access Permissions Administration", "Cancel", modalContent, window)
			permFormDialog.Resize(fyne.NewSize(580, 390))
			trackDialog(permFormDialog)
			permFormDialog.Show()
		}))

		var approvalButton *widget.Button
		var currentApprovalModal dialog.Dialog
		var renderApprovalQueueList func()

		refreshApprovalButtonState := func() {
			pCount := db.FetchPendingUserCount()
			approvalButton.SetText(fmt.Sprintf("Pending Registrations (%d)", pCount))
			if pCount > 0 {
				approvalButton.Importance = widget.HighImportance
			} else {
				approvalButton.Importance = widget.MediumImportance
			}
			approvalButton.Refresh()
		}

		renderApprovalQueueList = func() {
			pendingUsers, err := db.FetchPendingUsers()
			if err != nil {
				dialog.ShowError(err, window)
				return
			}

			if len(pendingUsers) == 0 {
				if currentApprovalModal != nil {
					currentApprovalModal.Hide()
				}
				dialog.ShowInformation("Queue Clean", "All pending registration parameters have been processed successfully.", window)
				refreshApprovalButtonState()
				return
			}

			listContainer := container.NewVBox()
			headerRow := container.NewGridWithColumns(4,
				widget.NewLabelWithStyle("Operator Username", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewLabelWithStyle("Assign Clearance", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewLabelWithStyle("Grant Action", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
				widget.NewLabelWithStyle("Purge Action", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			)
			listContainer.Add(headerRow)
			listContainer.Add(widget.NewSeparator())

			for _, targetPendingUser := range pendingUsers {
				uNameLocal := targetPendingUser

				roleSel := widget.NewSelect([]string{"user", "operator", "admin"}, nil)
				roleSel.SetSelected("user")

				approveBtn := widget.NewButtonWithIcon("Approve", theme.ConfirmIcon(), func() {
					resetInactivityTimer()
					if err := db.ExecuteProcessApproval(username, uNameLocal, roleSel.Selected, true); err == nil {
						refreshApprovalButtonState()
						if currentApprovalModal != nil {
							currentApprovalModal.Hide()
						}
						renderApprovalQueueList()
					} else {
						dialog.ShowError(err, window)
					}
				})
				approveBtn.Importance = widget.HighImportance

				textRejectBtn := widget.NewButtonWithIcon("Reject Request", theme.CancelIcon(), func() {
					resetInactivityTimer()
					if err := db.ExecuteProcessApproval(username, uNameLocal, "", false); err == nil {
						refreshApprovalButtonState()
						if currentApprovalModal != nil {
							currentApprovalModal.Hide()
						}
						renderApprovalQueueList()
					} else {
						dialog.ShowError(err, window)
					}
				})
				textRejectBtn.Importance = widget.MediumImportance

				row := container.NewGridWithColumns(4,
					widget.NewLabelWithStyle(uNameLocal, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					roleSel,
					approveBtn,
					textRejectBtn,
				)
				listContainer.Add(row)
			}

			scrollableLayout := container.NewScroll(listContainer)
			scrollableLayout.SetMinSize(fyne.NewSize(660, 320))

			currentApprovalModal = dialog.NewCustom("Identity Validation Portal Queue", "Close View", scrollableLayout, window)
			currentApprovalModal.Resize(fyne.NewSize(700, 380))
			trackDialog(currentApprovalModal)
			currentApprovalModal.Show()
		}

		approvalButton = widget.NewButtonWithIcon("Pending Registrations (...)", theme.WarningIcon(), func() {
			resetInactivityTimer()
			renderApprovalQueueList()
		})

		refreshApprovalButtonState()

		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()

			for range ticker.C {
				activityLock.Lock()
				activeStatus := isSessionActive
				activityLock.Unlock()

				if !activeStatus {
					return
				}

				pCount := db.FetchPendingUserCount()
				approvalButton.SetText(fmt.Sprintf("Pending Registrations (%d)", pCount))
				if pCount > 0 {
					approvalButton.Importance = widget.HighImportance
				} else {
					approvalButton.Importance = widget.MediumImportance
				}
				approvalButton.Refresh()
			}
		}()

		actionPanel.Add(approvalButton)
	}

	if !canWrite && !canDelete && !canManageUsers {
		actionPanel.Add(widget.NewLabelWithStyle("🛡️ Security Status: Read-Only Data Viewer Mode Active", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
	}

	logoutButton := widget.NewButtonWithIcon("Logout Session", theme.LogoutIcon(), func() {
		resetInactivityTimer()
		dialog.ShowConfirm("End Active Session?", "Are you sure you want to log out of the secure core terminal?", func(confirmed bool) {
			if confirmed {
				activityLock.Lock()
				isSessionActive = false
				activityLock.Unlock()

				clearAllActiveDialogs()

				gridData = [][]string{{
					"Series ID & Name", "Account Number", "Subscriber Name",
					"Employee Code", "Beneficiary Code", "Designation",
					"Mobile No", "Date of Birth", "DDO Code", "PIN",
				}}
				selectedRowIndex = -1
				RenderLoginView(app, window)
			}
		}, window)
	})
	logoutButton.Importance = widget.LowImportance

	dashLogo := canvas.NewImageFromFile("assets/logo.png")
	dashLogo.FillMode = canvas.ImageFillContain
	dashLogo.SetMinSize(fyne.NewSize(75, 75))

	// FIXED INITIALIZATIONS: Initialized text and bound it smoothly inside the card configuration frame
	profileText := fmt.Sprintf("Operator ID: %s\nClearance: User Secure %s\nSession Start: %s", username, role, lastLogin)
	profileCard := widget.NewCard("eGPF Operational Core Enterprise Dashboard", "Secure Session Profile Context", widget.NewLabel(profileText))

	headerLayout := container.NewBorder(nil, nil, nil, container.NewHBox(container.NewPadded(dashLogo), logoutButton), profileCard)

	dashboardFooterLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("System Registry Core v%s\n%s", CurrentClientVersion, CopyrightInfo),
		fyne.TextAlignCenter,
		fyne.TextStyle{Italic: true},
	)
	dashboardFooter := container.NewVBox(
		widget.NewSeparator(),
		dashboardFooterLabel,
	)

	dashboardView := container.NewBorder(
		container.NewVBox(headerLayout, actionPanel, widget.NewSeparator(), searchBarLayout),
		dashboardFooter,
		nil, nil,
		container.NewPadded(dataTable),
	)

	window.SetContent(dashboardView)
}
