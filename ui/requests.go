package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"egpf-app/db"
)

// ShowRaiseServiceRequestModal presents a modal for submitting a change request
func ShowRaiseServiceRequestModal(window fyne.Window, username, role string, prefillType string, prefillData map[string]string) {
	seriesOptions, seriesMap, _ := db.FetchSeriesDropdownOptions()
	if len(seriesOptions) == 0 {
		seriesOptions = []string{"EDN", "MED", "POL", "FOR"}
	}

	requestTypeSelect := widget.NewSelect([]string{
		"🔑 Request PIN Creation / Reset",
		"👤 Request New Subscriber Profile",
		"✏️ Request Subscriber Data Update",
		"🏛️ Request New DDO Registration",
		"📋 Request DDO Profile Update",
		"🏦 Request New Treasury Registration",
		"📑 Request Treasury Profile Update",
	}, nil)

	formContainer := container.NewVBox()

	// Form inputs for Subscriber operations
	seriesDropdown := widget.NewSelect(seriesOptions, nil)
	seriesDropdown.PlaceHolder = "[ Select Series ]"

	accountNoEntry := widget.NewEntry()
	accountNoEntry.SetPlaceHolder("Subscriber Account Number")

	subscriberNameEntry := widget.NewEntry()
	subscriberNameEntry.SetPlaceHolder("Full Name of Subscriber")

	designationEntry := widget.NewEntry()
	designationEntry.SetPlaceHolder("Official Designation")

	mobileEntry := widget.NewEntry()
	mobileEntry.SetPlaceHolder("10-Digit Mobile Number")

	pinEntry := widget.NewEntry()
	pinEntry.SetPlaceHolder("Security Access PIN (4-Digits)")

	autoPinBtn := widget.NewButtonWithIcon("Auto-PIN", theme.ViewRefreshIcon(), func() {
		pinEntry.SetText(generateRandom4DigitPIN())
	})

	ddoCodeEntry := widget.NewEntry()
	ddoCodeEntry.SetPlaceHolder("DDO Code")

	// Form inputs for DDO operations
	ddoPhoneEntry := widget.NewEntry()
	ddoPhoneEntry.SetPlaceHolder("Official Contact Phone")

	ddoEmailEntry := widget.NewEntry()
	ddoEmailEntry.SetPlaceHolder("Official Email Address")

	ddoTresCodeEntry := widget.NewEntry()
	ddoTresCodeEntry.SetPlaceHolder("Treasury Code")

	ddoVlcCodeEntry := widget.NewEntry()
	ddoVlcCodeEntry.SetPlaceHolder("VLC DDO Code")

	// Form inputs for Treasury operations
	tresNameEntry := widget.NewEntry()
	tresNameEntry.SetPlaceHolder("Treasury Full Name")

	tresVlcCodeEntry := widget.NewEntry()
	tresVlcCodeEntry.SetPlaceHolder("VLC Treasury Code")

	reasonEntry := widget.NewMultiLineEntry()
	reasonEntry.SetPlaceHolder("Reason or justification for this change request...")
	reasonEntry.SetMinRowsVisible(2)

	var currentModal dialog.Dialog

	renderForm := func(chosenType string) {
		formContainer.Objects = nil

		switch chosenType {
		case "🔑 Request PIN Creation / Reset":
			targetChoice := widget.NewRadioGroup([]string{"Subscriber Account", "DDO Login", "Treasury Login"}, nil)
			targetChoice.Horizontal = true
			targetChoice.SetSelected("Subscriber Account")

			subForm := widget.NewForm(
				widget.NewFormItem("Series", seriesDropdown),
				widget.NewFormItem("Account No", accountNoEntry),
			)

			ddoForm := widget.NewForm(
				widget.NewFormItem("DDO Code", ddoCodeEntry),
			)
			ddoForm.Hide()

			tresForm := widget.NewForm(
				widget.NewFormItem("Treasury Code", ddoTresCodeEntry),
			)
			tresForm.Hide()

			targetChoice.OnChanged = func(sel string) {
				subForm.Hide()
				ddoForm.Hide()
				tresForm.Hide()
				switch sel {
				case "Subscriber Account":
					subForm.Show()
				case "DDO Login":
					ddoForm.Show()
				case "Treasury Login":
					tresForm.Show()
				}
			}

			pinRow := container.NewBorder(nil, nil, nil, autoPinBtn, pinEntry)
			commonForm := widget.NewForm(
				widget.NewFormItem("New PIN", pinRow),
				widget.NewFormItem("Justification", reasonEntry),
			)

			formContainer.Add(container.NewVBox(
				widget.NewLabelWithStyle("Target Type:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				targetChoice,
				widget.NewSeparator(),
				subForm,
				ddoForm,
				tresForm,
				widget.NewSeparator(),
				commonForm,
			))

		case "👤 Request New Subscriber Profile":
			pinRow := container.NewBorder(nil, nil, nil, autoPinBtn, pinEntry)
			form := widget.NewForm(
				widget.NewFormItem("Series", seriesDropdown),
				widget.NewFormItem("Account No", accountNoEntry),
				widget.NewFormItem("Full Name", subscriberNameEntry),
				widget.NewFormItem("Designation", designationEntry),
				widget.NewFormItem("Mobile No", mobileEntry),
				widget.NewFormItem("DDO Code", ddoCodeEntry),
				widget.NewFormItem("Initial PIN", pinRow),
				widget.NewFormItem("Justification", reasonEntry),
			)
			formContainer.Add(form)

		case "✏️ Request Subscriber Data Update":
			pinRow := container.NewBorder(nil, nil, nil, autoPinBtn, pinEntry)
			form := widget.NewForm(
				widget.NewFormItem("Series", seriesDropdown),
				widget.NewFormItem("Account No", accountNoEntry),
				widget.NewFormItem("Full Name", subscriberNameEntry),
				widget.NewFormItem("Designation", designationEntry),
				widget.NewFormItem("Mobile No", mobileEntry),
				widget.NewFormItem("DDO Code", ddoCodeEntry),
				widget.NewFormItem("PIN (Optional)", pinRow),
				widget.NewFormItem("Reason / Notes", reasonEntry),
			)
			formContainer.Add(form)

		case "🏛️ Request New DDO Registration":
			pinRow := container.NewBorder(nil, nil, nil, autoPinBtn, pinEntry)
			form := widget.NewForm(
				widget.NewFormItem("DDO Code", ddoCodeEntry),
				widget.NewFormItem("Designation", designationEntry),
				widget.NewFormItem("Phone No", ddoPhoneEntry),
				widget.NewFormItem("Email Address", ddoEmailEntry),
				widget.NewFormItem("Treasury Code", ddoTresCodeEntry),
				widget.NewFormItem("VLC Code", ddoVlcCodeEntry),
				widget.NewFormItem("Gate PIN", pinRow),
				widget.NewFormItem("Justification", reasonEntry),
			)
			formContainer.Add(form)

		case "📋 Request DDO Profile Update":
			pinRow := container.NewBorder(nil, nil, nil, autoPinBtn, pinEntry)
			form := widget.NewForm(
				widget.NewFormItem("DDO Code", ddoCodeEntry),
				widget.NewFormItem("Designation", designationEntry),
				widget.NewFormItem("Phone No", ddoPhoneEntry),
				widget.NewFormItem("Email Address", ddoEmailEntry),
				widget.NewFormItem("Treasury Code", ddoTresCodeEntry),
				widget.NewFormItem("VLC Code", ddoVlcCodeEntry),
				widget.NewFormItem("Gate PIN", pinRow),
				widget.NewFormItem("Reason / Notes", reasonEntry),
			)
			formContainer.Add(form)

		case "🏦 Request New Treasury Registration":
			pinRow := container.NewBorder(nil, nil, nil, autoPinBtn, pinEntry)
			form := widget.NewForm(
				widget.NewFormItem("Treasury Code", ddoTresCodeEntry),
				widget.NewFormItem("Treasury Name", tresNameEntry),
				widget.NewFormItem("VLC Treasury Code", tresVlcCodeEntry),
				widget.NewFormItem("Gate PIN", pinRow),
				widget.NewFormItem("Justification", reasonEntry),
			)
			formContainer.Add(form)

		case "📑 Request Treasury Profile Update":
			pinRow := container.NewBorder(nil, nil, nil, autoPinBtn, pinEntry)
			form := widget.NewForm(
				widget.NewFormItem("Treasury Code", ddoTresCodeEntry),
				widget.NewFormItem("Treasury Name", tresNameEntry),
				widget.NewFormItem("VLC Treasury Code", tresVlcCodeEntry),
				widget.NewFormItem("Gate PIN", pinRow),
				widget.NewFormItem("Reason / Notes", reasonEntry),
			)
			formContainer.Add(form)
		}
		formContainer.Refresh()
	}

	requestTypeSelect.OnChanged = renderForm

	// Apply pre-fills if passed
	if prefillData != nil {
		if s, ok := prefillData["series"]; ok {
			for _, opt := range seriesOptions {
				if strings.HasPrefix(opt, s) || opt == s {
					seriesDropdown.SetSelected(opt)
					break
				}
			}
		}
		if a, ok := prefillData["account_no"]; ok {
			accountNoEntry.SetText(a)
		}
		if n, ok := prefillData["name"]; ok && n != "N/A" {
			subscriberNameEntry.SetText(n)
		}
		if d, ok := prefillData["designation"]; ok && d != "N/A" {
			designationEntry.SetText(d)
		}
		if m, ok := prefillData["mobile"]; ok && m != "N/A" {
			mobileEntry.SetText(m)
		}
		if p, ok := prefillData["pin"]; ok && p != "N/A" && p != "****" && p != "[REDACTED]" {
			pinEntry.SetText(p)
		}
		if ddo, ok := prefillData["ddo_code"]; ok && ddo != "N/A" {
			ddoCodeEntry.SetText(ddo)
		}
		if ph, ok := prefillData["phone"]; ok && ph != "N/A" {
			ddoPhoneEntry.SetText(ph)
		}
		if em, ok := prefillData["email"]; ok && em != "N/A" {
			ddoEmailEntry.SetText(em)
		}
		if tc, ok := prefillData["treasury_code"]; ok && tc != "N/A" {
			ddoTresCodeEntry.SetText(tc)
		}
		if tn, ok := prefillData["treasury_name"]; ok && tn != "N/A" {
			tresNameEntry.SetText(tn)
		}
		if vc, ok := prefillData["vlc_code"]; ok && vc != "N/A" {
			tresVlcCodeEntry.SetText(vc)
		}
	}

	if prefillType != "" {
		requestTypeSelect.SetSelected(prefillType)
	} else {
		requestTypeSelect.SetSelected("🔑 Request PIN Creation / Reset")
	}

	submitBtn := widget.NewButtonWithIcon("Submit Request to Admin", theme.MailSendIcon(), func() {
		chosen := requestTypeSelect.Selected
		var reqType, targetEntity string
		var payload db.RequestPayloadDetails

		resolvedSeriesID := ""
		if seriesDropdown.Selected != "" {
			if id, ok := seriesMap[seriesDropdown.Selected]; ok {
				resolvedSeriesID = id
			} else {
				resolvedSeriesID = seriesDropdown.Selected
			}
		}

		payload.ReasonRemarks = strings.TrimSpace(reasonEntry.Text)

		switch chosen {
		case "🔑 Request PIN Creation / Reset":
			reqType = "CREATE_PIN"
			payload.PIN = strings.TrimSpace(pinEntry.Text)
			if payload.PIN == "" {
				dialog.ShowError(fmt.Errorf("PIN is required"), window)
				return
			}
			if resolvedSeriesID != "" && accountNoEntry.Text != "" {
				payload.SeriesID = resolvedSeriesID
				payload.AccountNo = strings.TrimSpace(accountNoEntry.Text)
				targetEntity = fmt.Sprintf("Subscriber [Series: %s, Acc: %s]", payload.SeriesID, payload.AccountNo)
			} else if ddoCodeEntry.Text != "" {
				payload.DDOCode = strings.TrimSpace(ddoCodeEntry.Text)
				targetEntity = fmt.Sprintf("DDO Login [Code: %s]", payload.DDOCode)
			} else if ddoTresCodeEntry.Text != "" {
				payload.TreasuryCode = strings.TrimSpace(ddoTresCodeEntry.Text)
				targetEntity = fmt.Sprintf("Treasury Login [Code: %s]", payload.TreasuryCode)
			} else {
				dialog.ShowError(fmt.Errorf("Please specify Series & Account No, DDO Code, OR Treasury Code"), window)
				return
			}

		case "👤 Request New Subscriber Profile":
			reqType = "CREATE_SUBSCRIBER"
			if resolvedSeriesID == "" || strings.TrimSpace(accountNoEntry.Text) == "" {
				dialog.ShowError(fmt.Errorf("Series and Account Number are mandatory"), window)
				return
			}
			payload.SeriesID = resolvedSeriesID
			payload.AccountNo = strings.TrimSpace(accountNoEntry.Text)
			payload.SubscriberName = strings.TrimSpace(subscriberNameEntry.Text)
			payload.Designation = strings.TrimSpace(designationEntry.Text)
			payload.MobileNo = strings.TrimSpace(mobileEntry.Text)
			payload.DDOCode = strings.TrimSpace(ddoCodeEntry.Text)
			payload.PIN = strings.TrimSpace(pinEntry.Text)
			targetEntity = fmt.Sprintf("New Subscriber [%s - %s]", payload.SeriesID, payload.AccountNo)

		case "✏️ Request Subscriber Data Update":
			reqType = "UPDATE_SUBSCRIBER"
			if resolvedSeriesID == "" || strings.TrimSpace(accountNoEntry.Text) == "" {
				dialog.ShowError(fmt.Errorf("Series and Account Number are mandatory"), window)
				return
			}
			payload.SeriesID = resolvedSeriesID
			payload.AccountNo = strings.TrimSpace(accountNoEntry.Text)
			payload.SubscriberName = strings.TrimSpace(subscriberNameEntry.Text)
			payload.Designation = strings.TrimSpace(designationEntry.Text)
			payload.MobileNo = strings.TrimSpace(mobileEntry.Text)
			payload.DDOCode = strings.TrimSpace(ddoCodeEntry.Text)
			payload.PIN = strings.TrimSpace(pinEntry.Text)
			targetEntity = fmt.Sprintf("Subscriber Update [%s - %s]", payload.SeriesID, payload.AccountNo)

		case "🏛️ Request New DDO Registration":
			reqType = "CREATE_DDO"
			if strings.TrimSpace(ddoCodeEntry.Text) == "" {
				dialog.ShowError(fmt.Errorf("DDO Code is required"), window)
				return
			}
			payload.DDOCode = strings.TrimSpace(ddoCodeEntry.Text)
			payload.Designation = strings.TrimSpace(designationEntry.Text)
			payload.PhoneNo = strings.TrimSpace(ddoPhoneEntry.Text)
			payload.Email = strings.TrimSpace(ddoEmailEntry.Text)
			payload.TreasuryCode = strings.TrimSpace(ddoTresCodeEntry.Text)
			payload.VLCCode = strings.TrimSpace(ddoVlcCodeEntry.Text)
			payload.PIN = strings.TrimSpace(pinEntry.Text)
			targetEntity = fmt.Sprintf("New DDO Master [%s]", payload.DDOCode)

		case "📋 Request DDO Profile Update":
			reqType = "UPDATE_DDO"
			if strings.TrimSpace(ddoCodeEntry.Text) == "" {
				dialog.ShowError(fmt.Errorf("DDO Code is required"), window)
				return
			}
			payload.DDOCode = strings.TrimSpace(ddoCodeEntry.Text)
			payload.Designation = strings.TrimSpace(designationEntry.Text)
			payload.PhoneNo = strings.TrimSpace(ddoPhoneEntry.Text)
			payload.Email = strings.TrimSpace(ddoEmailEntry.Text)
			payload.TreasuryCode = strings.TrimSpace(ddoTresCodeEntry.Text)
			payload.VLCCode = strings.TrimSpace(ddoVlcCodeEntry.Text)
			payload.PIN = strings.TrimSpace(pinEntry.Text)
			targetEntity = fmt.Sprintf("DDO Update [%s]", payload.DDOCode)

		case "🏦 Request New Treasury Registration":
			reqType = "CREATE_TREASURY"
			if strings.TrimSpace(ddoTresCodeEntry.Text) == "" {
				dialog.ShowError(fmt.Errorf("Treasury Code is required"), window)
				return
			}
			payload.TreasuryCode = strings.TrimSpace(ddoTresCodeEntry.Text)
			payload.TreasuryName = strings.TrimSpace(tresNameEntry.Text)
			payload.VLCCode = strings.TrimSpace(tresVlcCodeEntry.Text)
			payload.PIN = strings.TrimSpace(pinEntry.Text)
			targetEntity = fmt.Sprintf("New Treasury Master [%s]", payload.TreasuryCode)

		case "📑 Request Treasury Profile Update":
			reqType = "UPDATE_TREASURY"
			if strings.TrimSpace(ddoTresCodeEntry.Text) == "" {
				dialog.ShowError(fmt.Errorf("Treasury Code is required"), window)
				return
			}
			payload.TreasuryCode = strings.TrimSpace(ddoTresCodeEntry.Text)
			payload.TreasuryName = strings.TrimSpace(tresNameEntry.Text)
			payload.VLCCode = strings.TrimSpace(tresVlcCodeEntry.Text)
			payload.PIN = strings.TrimSpace(pinEntry.Text)
			targetEntity = fmt.Sprintf("Treasury Update [%s]", payload.TreasuryCode)
		}

		err := db.ExecuteSubmitServiceRequest(username, reqType, targetEntity, payload)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to submit service request: %v", err), window)
			return
		}

		if currentModal != nil {
			currentModal.Hide()
		}
		dialog.ShowInformation("Request Submitted", fmt.Sprintf("Your service request for %s has been submitted successfully.\nAn Administrator will review and approve it.", targetEntity), window)
	})
	submitBtn.Importance = widget.HighImportance

	topHeader := container.NewVBox(
		widget.NewLabelWithStyle("📝 Submit Service Change Request", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Select the type of change you wish to request from System Administrators:"),
		requestTypeSelect,
		widget.NewSeparator(),
	)

	bottomBar := container.NewVBox(
		widget.NewSeparator(),
		submitBtn,
	)

	scroll := container.NewScroll(container.NewPadded(formContainer))
	scroll.SetMinSize(fyne.NewSize(580, 360))

	mainLayout := container.NewBorder(
		topHeader,
		bottomBar,
		nil,
		nil,
		scroll,
	)

	currentModal = dialog.NewCustom("eGPF Service Request Portal", "Cancel", mainLayout, window)
	currentModal.Resize(fyne.NewSize(620, 560))
	currentModal.Show()
}

// ShowMyRequestsPortal opens a modal showing requests raised by the logged-in user
func ShowMyRequestsPortal(window fyne.Window, username string) {
	requestsListContainer := container.NewVBox()
	scroll := container.NewScroll(requestsListContainer)
	scroll.SetMinSize(fyne.NewSize(700, 420))

	filterSelect := widget.NewSelect([]string{"ALL", "PENDING", "APPROVED", "REJECTED"}, nil)
	filterSelect.SetSelected("ALL")

	var refreshList func()
	refreshList = func() {
		requestsListContainer.Objects = nil

		requests, err := db.FetchServiceRequests(filterSelect.Selected, username, 100)
		if err != nil || len(requests) == 0 {
			requestsListContainer.Add(widget.NewLabel("No service requests found in this status category."))
			requestsListContainer.Refresh()
			return
		}

		for _, req := range requests {
			r := req
			statusColor := "🟡"
			switch r.Status {
			case "APPROVED":
				statusColor = "🟢"
			case "REJECTED":
				statusColor = "🔴"
			}

			titleStr := fmt.Sprintf("%s [%s] #%d - %s", statusColor, r.Status, r.ID, r.RequestType)
			targetStr := fmt.Sprintf("Target: %s | Submitted: %s", r.TargetEntity, r.CreatedAt.Format("02-Jan-2006 15:04"))

			var payload db.RequestPayloadDetails
			_ = json.Unmarshal([]byte(r.RequestDetails), &payload)

			detailsVBox := container.NewVBox()
			if payload.ReasonRemarks != "" {
				detailsVBox.Add(widget.NewLabel(fmt.Sprintf("• Remarks: %s", payload.ReasonRemarks)))
			}
			if r.ReviewerUsername != nil && *r.ReviewerUsername != "" {
				remarks := ""
				if r.ReviewerRemarks != nil {
					remarks = *r.ReviewerRemarks
				}
				detailsVBox.Add(widget.NewLabel(fmt.Sprintf("• Reviewed by: %s | Notes: %s", *r.ReviewerUsername, remarks)))
			}

			card := widget.NewCard(titleStr, targetStr, detailsVBox)
			requestsListContainer.Add(card)
			requestsListContainer.Add(widget.NewSeparator())
		}
		requestsListContainer.Refresh()
	}

	filterSelect.OnChanged = func(_ string) {
		refreshList()
	}

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
		refreshList()
	})

	topBar := container.NewBorder(nil, nil, widget.NewLabel("Filter Status:"), refreshBtn, filterSelect)
	mainView := container.NewBorder(topBar, nil, nil, nil, scroll)

	d := dialog.NewCustom("📋 My Service Requests History", "Close", mainView, window)
	refreshList()
	d.Resize(fyne.NewSize(720, 480))
	d.Show()
}

// ShowAdminServiceRequestsPortal opens the Admin Portal to review, approve, and reject user requests
func ShowAdminServiceRequestsPortal(window fyne.Window, adminUsername string) {
	requestsContainer := container.NewVBox()
	scroll := container.NewScroll(requestsContainer)
	scroll.SetMinSize(fyne.NewSize(820, 480))

	filterSelect := widget.NewSelect([]string{"PENDING", "ALL", "APPROVED", "REJECTED"}, nil)
	filterSelect.SetSelected("PENDING")

	headerCountLabel := widget.NewLabelWithStyle("📥 Pending Service Requests Queue", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	var refreshAdminQueue func()
	refreshAdminQueue = func() {
		requestsContainer.Objects = nil

		pendingCount := db.FetchPendingServiceRequestCount()
		headerCountLabel.SetText(fmt.Sprintf("📥 Service Requests Queue (%d Pending Awaiting Review)", pendingCount))

		requests, err := db.FetchServiceRequests(filterSelect.Selected, "", 150)
		if err != nil || len(requests) == 0 {
			requestsContainer.Add(widget.NewLabel("No requests found matching the selected filter criteria."))
			requestsContainer.Refresh()
			return
		}

		for _, req := range requests {
			r := req
			statusColor := "🟡"
			switch r.Status {
			case "APPROVED":
				statusColor = "🟢"
			case "REJECTED":
				statusColor = "🔴"
			}

			titleStr := fmt.Sprintf("%s [%s] Req #%d: %s (by @%s)", statusColor, r.Status, r.ID, r.RequestType, r.RequesterUsername)
			subTitleStr := fmt.Sprintf("Target: %s | Date: %s", r.TargetEntity, r.CreatedAt.Format("02-Jan-2006 15:04:05"))

			var payload db.RequestPayloadDetails
			_ = json.Unmarshal([]byte(r.RequestDetails), &payload)

			payloadDetailsText := []string{}
			if payload.SeriesID != "" {
				payloadDetailsText = append(payloadDetailsText, fmt.Sprintf("Series: %s", payload.SeriesID))
			}
			if payload.AccountNo != "" {
				payloadDetailsText = append(payloadDetailsText, fmt.Sprintf("Account No: %s", payload.AccountNo))
			}
			if payload.SubscriberName != "" {
				payloadDetailsText = append(payloadDetailsText, fmt.Sprintf("Subscriber Name: %s", payload.SubscriberName))
			}
			if payload.Designation != "" {
				payloadDetailsText = append(payloadDetailsText, fmt.Sprintf("Designation: %s", payload.Designation))
			}
			if payload.MobileNo != "" {
				payloadDetailsText = append(payloadDetailsText, fmt.Sprintf("Mobile: %s", payload.MobileNo))
			}
			if payload.DDOCode != "" {
				payloadDetailsText = append(payloadDetailsText, fmt.Sprintf("DDO Code: %s", payload.DDOCode))
			}
			if payload.PhoneNo != "" {
				payloadDetailsText = append(payloadDetailsText, fmt.Sprintf("Phone: %s", payload.PhoneNo))
			}
			if payload.Email != "" {
				payloadDetailsText = append(payloadDetailsText, fmt.Sprintf("Email: %s", payload.Email))
			}
			if payload.TreasuryCode != "" {
				payloadDetailsText = append(payloadDetailsText, fmt.Sprintf("Treasury Code: %s", payload.TreasuryCode))
			}
			if payload.TreasuryName != "" {
				payloadDetailsText = append(payloadDetailsText, fmt.Sprintf("Treasury Name: %s", payload.TreasuryName))
			}
			if payload.VLCCode != "" {
				payloadDetailsText = append(payloadDetailsText, fmt.Sprintf("VLC Code: %s", payload.VLCCode))
			}
			if payload.PIN != "" {
				payloadDetailsText = append(payloadDetailsText, fmt.Sprintf("PIN: %s", payload.PIN))
			}
			if payload.ReasonRemarks != "" {
				payloadDetailsText = append(payloadDetailsText, fmt.Sprintf("Requester Justification: %s", payload.ReasonRemarks))
			}

			detailsVBox := container.NewVBox(
				widget.NewLabelWithStyle("Requested Data Payload:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: true}),
				widget.NewLabel(strings.Join(payloadDetailsText, "  |  ")),
			)

			if r.ReviewerUsername != nil && *r.ReviewerUsername != "" {
				remarks := ""
				if r.ReviewerRemarks != nil {
					remarks = *r.ReviewerRemarks
				}
				detailsVBox.Add(widget.NewLabel(fmt.Sprintf("Reviewed by: %s | Remarks: %s", *r.ReviewerUsername, remarks)))
			}

			var cardContent fyne.CanvasObject = detailsVBox

			if r.Status == "PENDING" {
				adminRemarksEntry := widget.NewEntry()
				adminRemarksEntry.SetPlaceHolder("Optional Reviewer Remarks / Feedback...")

				approveBtn := widget.NewButtonWithIcon("Approve & Apply to DB", theme.ConfirmIcon(), func() {
					dialog.ShowConfirm("Confirm Approval", fmt.Sprintf("Approve and automatically execute changes for Request #%d (%s)?", r.ID, r.TargetEntity), func(confirmed bool) {
						if confirmed {
							err := db.ExecuteApproveServiceRequest(adminUsername, r.ID, adminRemarksEntry.Text)
							if err != nil {
								dialog.ShowError(fmt.Errorf("Failed to approve request: %v", err), window)
								return
							}
							dialog.ShowInformation("Approved", fmt.Sprintf("Request #%d approved and applied successfully.", r.ID), window)
							refreshAdminQueue()
						}
					}, window)
				})
				approveBtn.Importance = widget.HighImportance

				rejectBtn := widget.NewButtonWithIcon("Reject Request", theme.CancelIcon(), func() {
					reasonInput := widget.NewEntry()
					reasonInput.SetPlaceHolder("Enter reason for rejection...")

					d := dialog.NewForm("Reject Service Request", "Confirm Rejection", "Cancel", []*widget.FormItem{
						widget.NewFormItem("Rejection Reason", reasonInput),
					}, func(confirmed bool) {
						if confirmed {
							err := db.ExecuteRejectServiceRequest(adminUsername, r.ID, reasonInput.Text)
							if err != nil {
								dialog.ShowError(fmt.Errorf("Failed to reject request: %v", err), window)
								return
							}
							refreshAdminQueue()
						}
					}, window)
					d.Show()
				})

				actionsBar := container.NewHBox(
					approveBtn,
					rejectBtn,
				)

				cardContent = container.NewVBox(
					detailsVBox,
					widget.NewSeparator(),
					container.NewBorder(nil, nil, widget.NewLabel("Admin Remarks:"), actionsBar, adminRemarksEntry),
				)
			}

			card := widget.NewCard(titleStr, subTitleStr, cardContent)
			requestsContainer.Add(card)
			requestsContainer.Add(widget.NewSeparator())
		}
		requestsContainer.Refresh()
	}

	filterSelect.OnChanged = func(_ string) {
		refreshAdminQueue()
	}

	refreshBtn := widget.NewButtonWithIcon("Refresh Queue", theme.ViewRefreshIcon(), func() {
		refreshAdminQueue()
	})

	topControlBar := container.NewBorder(
		nil, nil,
		headerCountLabel,
		refreshBtn,
		container.NewHBox(widget.NewLabel("Status:"), filterSelect),
	)

	mainLayout := container.NewBorder(
		container.NewVBox(topControlBar, widget.NewSeparator()),
		nil, nil, nil,
		scroll,
	)

	adminDialog := dialog.NewCustom("📥 eGPF Enterprise Service Requests Console", "Close Console", mainLayout, window)
	adminDialog.Resize(fyne.NewSize(860, 560))
	refreshAdminQueue()
	adminDialog.Show()
}
