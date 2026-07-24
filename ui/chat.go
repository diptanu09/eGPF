package ui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"egpf-app/db"
)

// ShowSupportChatPortal opens a dedicated popup dialog for real-time Support Communication
func ShowSupportChatPortal(window fyne.Window, currentUsername string, currentRole string) {
	var targetUser string
	isAdmin := (currentRole == "admin")

	// 1. UI Widgets Setup
	messageListContainer := container.NewVBox()
	chatScroll := container.NewScroll(messageListContainer)
	chatScroll.SetMinSize(fyne.NewSize(580, 360))

	messageInput := widget.NewEntry()
	messageInput.SetPlaceHolder("Type your message here...")

	headerStatusLabel := widget.NewLabelWithStyle(
		"Select a contact thread to begin communication.",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true, Italic: true},
	)

	// 2. Chat History Refresh Logic
	refreshChatHistory := func() {
		if targetUser == "" {
			return
		}

		messages, err := db.FetchChatHistory(currentUsername, targetUser, 100)
		if err != nil {
			return
		}

		_ = db.MarkMessagesAsRead(currentUsername, targetUser)

		messageListContainer.Objects = nil

		if len(messages) == 0 {
			emptyLbl := widget.NewLabelWithStyle("No prior messages found. Start the conversation!", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
			messageListContainer.Add(container.NewCenter(emptyLbl))
		} else {
			for _, msg := range messages {
				timeStr := msg.CreatedAt.Format("15:04:05 | Jan 02")
				isSelf := (strings.EqualFold(msg.SenderUsername, currentUsername))

				senderPrefix := fmt.Sprintf("[%s] %s:", timeStr, msg.SenderUsername)
				if isSelf {
					senderPrefix = fmt.Sprintf("[%s] You (%s):", timeStr, msg.SenderUsername)
				}

				headerLbl := widget.NewLabelWithStyle(senderPrefix, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
				bodyLbl := widget.NewLabel(msg.MessageText)
				bodyLbl.Wrapping = fyne.TextWrapWord

				bubbleContent := container.NewVBox(headerLbl, bodyLbl)
				card := widget.NewCard("", "", bubbleContent)

				if isSelf {
					messageListContainer.Add(container.NewHBox(widget.NewLabel("       "), container.NewGridWrap(fyne.NewSize(420, card.MinSize().Height+20), card)))
				} else {
					messageListContainer.Add(container.NewHBox(container.NewGridWrap(fyne.NewSize(420, card.MinSize().Height+20), card), widget.NewLabel("       ")))
				}
			}
		}

		messageListContainer.Refresh()
		chatScroll.ScrollToBottom()
	}

	// 3. Message Dispatch Logic
	sendMessage := func() {
		text := strings.TrimSpace(messageInput.Text)
		if text == "" {
			return
		}
		if targetUser == "" {
			dialog.ShowInformation("Select Contact", "Please select an admin or user thread first.", window)
			return
		}

		err := db.ExecuteSendMessage(currentUsername, targetUser, text)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to send message: %v", err), window)
			return
		}

		messageInput.SetText("")
		refreshChatHistory()
	}

	sendBtn := widget.NewButtonWithIcon("Send", theme.MailSendIcon(), sendMessage)
	sendBtn.Importance = widget.HighImportance

	messageInput.OnSubmitted = func(_ string) {
		sendMessage()
	}

	inputRow := container.NewBorder(nil, nil, nil, sendBtn, messageInput)

	// 4. Contact Selector Panel
	var contactSelector fyne.CanvasObject

	if isAdmin {
		threadSelectDropdown := widget.NewSelect([]string{}, nil)
		threadSelectDropdown.PlaceHolder = "[ Select User Thread ]"

		loadUserThreads := func() {
			threads, err := db.FetchActiveUserThreads()
			if err != nil || len(threads) == 0 {
				threadSelectDropdown.Options = []string{"No user threads found"}
				threadSelectDropdown.Refresh()
				return
			}

			var options []string
			for _, t := range threads {
				opt := t.Username
				if t.UnreadCount > 0 {
					opt = fmt.Sprintf("🔴 %s (%d unread)", t.Username, t.UnreadCount)
				}
				options = append(options, opt)
			}
			threadSelectDropdown.Options = options
			threadSelectDropdown.Refresh()
		}

		threadSelectDropdown.OnChanged = func(selected string) {
			if selected == "" || selected == "No user threads found" {
				return
			}

			parsedUsername := strings.TrimSpace(selected)
			if strings.Contains(parsedUsername, "🔴") {
				parts := strings.Split(parsedUsername, " ")
				if len(parts) >= 2 {
					parsedUsername = parts[1]
				}
			}

			targetUser = parsedUsername
			headerStatusLabel.SetText(fmt.Sprintf("💬 Active Conversation with User: %s", targetUser))
			refreshChatHistory()
		}

		loadUserThreads()
		contactSelector = container.NewVBox(
			widget.NewLabelWithStyle("User Support Queue:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			threadSelectDropdown,
		)
	} else {
		adminSelectDropdown := widget.NewSelect([]string{}, nil)
		adminSelectDropdown.PlaceHolder = "[ Select System Administrator ]"

		admins, err := db.FetchAdminList()
		if err == nil && len(admins) > 0 {
			adminSelectDropdown.Options = admins
			adminSelectDropdown.SetSelected(admins[0])
			targetUser = admins[0]
			headerStatusLabel.SetText(fmt.Sprintf("💬 Support Channel with Admin: %s", targetUser))
			refreshChatHistory()
		} else {
			adminSelectDropdown.Options = []string{"admin"}
			adminSelectDropdown.SetSelected("admin")
			targetUser = "admin"
			headerStatusLabel.SetText("💬 Support Channel with Central Admin")
			refreshChatHistory()
		}

		adminSelectDropdown.OnChanged = func(selected string) {
			if selected != "" {
				targetUser = selected
				headerStatusLabel.SetText(fmt.Sprintf("💬 Support Channel with Admin: %s", targetUser))
				refreshChatHistory()
			}
		}

		contactSelector = container.NewVBox(
			widget.NewLabelWithStyle("Contact Administrator:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			adminSelectDropdown,
		)
	}

	topPanel := container.NewVBox(
		contactSelector,
		widget.NewSeparator(),
		headerStatusLabel,
		widget.NewSeparator(),
	)

	chatMainView := container.NewBorder(topPanel, inputRow, nil, nil, chatScroll)

	chatDialog := dialog.NewCustom("💬 eGPF Enterprise Support Chat Portal", "Close Portal", chatMainView, window)
	chatDialog.Resize(fyne.NewSize(680, 560))

	// 5. Real-time Background Polling Ticker
	stopPolling := make(chan struct{})

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if targetUser != "" {
					refreshChatHistory()
				}
			case <-stopPolling:
				return
			}
		}
	}()

	chatDialog.SetOnClosed(func() {
		close(stopPolling)
	})

	chatDialog.Show()
}
