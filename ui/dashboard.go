package ui

import (
	"fmt"
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
	CurrentClientVersion   = "2.2.0.1"
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

	// 🔒 Thread-safe activity metrics parameters tracking
	var activityLock sync.Mutex
	lastActivityTime := time.Now()
	isSessionActive := true

	resetInactivityTimer := func() {
		activityLock.Lock()
		lastActivityTime = time.Now()
		activityLock.Unlock()
	}

	// Automatically update activity timestamp when user interacts via keyboard context entries
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

				// Directly execute UI state modifications and view shifts (Safe in Fyne v2.4.0)
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

	canWrite := (role == "admin" || role == "operator")
	canDelete := (role == "admin")
	canManageUsers := (role == "admin")

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

	// NEW FEATURE: Added View DDO Details lookup context action item button inside panels array
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

		infoContent := container.NewVBox(
			widget.NewLabelWithStyle(fmt.Sprintf("DDO Master Code: %s", ddoInfo["ddo_code"]), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			widget.NewLabel(fmt.Sprintf("Official Designation: %s", ddoInfo["ddo_desg"])),
			widget.NewLabel(fmt.Sprintf("Contact Phone No: %s", ddoInfo["ddo_phone"])),
			widget.NewLabel(fmt.Sprintf("Secure Network Email: %s", ddoInfo["ddo_email"])),
			widget.NewLabel(fmt.Sprintf("Treasury Node Office Code: %s", ddoInfo["ddo_tres_code"])),
			widget.NewLabel(fmt.Sprintf("VLC Assigned Code Matrix: %s", ddoInfo["vlc_ddo_code"])),
			widget.NewLabelWithStyle(fmt.Sprintf("Gate Authorization PIN String: %s", ddoInfo["pin"]), fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
		)

		ddoModal := dialog.NewCustom("Drawing and Disbursing Officer (DDO) Master Reference", "Close View", infoContent, window)
		ddoModal.Resize(fyne.NewSize(480, 320))
		ddoModal.Show()
	}))

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

			dialog.ShowForm("Configure New System Series", "Save Series Definition", "Cancel", formItems, func(confirmed bool) {
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
			dialog.ShowForm("Modify Subscriber Data", "Apply Changes", "Cancel", items, func(confirmed bool) {
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

			dialog.ShowConfirm("Confirm Drop Action", confirmMsg, func(confirmed bool) {
				resetInactivityTimer()
				if confirmed {
					if err := db.ExecuteDeleteRecord(username, targetSerID, targetAccNo); err == nil {
						syncUiViewGrid(selectedSearchSeriesID, searchAccountBox.Text, searchNameBox.Text, false)
					} else {
						dialog.ShowError(err, window)
					}
				}
			}, window)
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

			dialog.ShowForm("Provision New System Terminal User", "Create User", "Cancel", formItems, func(confirmed bool) {
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
			dialog.ShowCustom("Enterprise Master User Management Console", "Close View", portalContent, window)
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

			dialog.ShowForm("Modify Distribution Setup Metadata", "Publish Update Rules", "Cancel", formItems, func(confirmed bool) {
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
		}))

		// =======================================================
		// LIVE SELF-REGISTRATION APPROVAL PANEL WITH AUDIT LOGGING
		// =======================================================
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
