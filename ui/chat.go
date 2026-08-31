package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"egpf-app/db"
)

// ShowSupportChatPortal opens a dedicated popup dialog for real-time Support Communication
func ShowSupportChatPortal(window fyne.Window, currentUsername string, currentRole string) {
	var targetUser string
	var chatLock sync.Mutex
	isAdmin := (currentRole == "admin")

	var lastLoadedCount int = -1
	var lastLoadedTarget string = ""

	// 1. UI Widgets Setup
	messageListContainer := container.NewVBox()
	chatScroll := container.NewScroll(messageListContainer)
	chatScroll.SetMinSize(fyne.NewSize(620, 360))

	messageInput := widget.NewEntry()
	messageInput.SetPlaceHolder("Type your support message here...")

	headerStatusLabel := widget.NewLabelWithStyle(
		"Select a contact thread to begin communication.",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)

	// 2. Chat History Refresh Logic
	refreshChatHistory := func(force bool) {
		chatLock.Lock()
		defer chatLock.Unlock()

		if targetUser == "" {
			return
		}

		messages, err := db.FetchChatHistory(currentUsername, targetUser, 150)
		if err != nil {
			return
		}

		// Avoid re-rendering and layout thrashing if no new messages arrived
		if !force && len(messages) == lastLoadedCount && targetUser == lastLoadedTarget {
			return
		}

		lastLoadedCount = len(messages)
		lastLoadedTarget = targetUser

		_ = db.MarkMessagesAsRead(currentUsername, targetUser)

		messageListContainer.Objects = nil

		if len(messages) == 0 {
			emptyLbl := widget.NewLabelWithStyle(
				fmt.Sprintf("No prior messages with %s. Send a message below to start the conversation.", targetUser),
				fyne.TextAlignCenter,
				fyne.TextStyle{Italic: true},
			)
			messageListContainer.Add(container.NewPadded(container.NewCenter(emptyLbl)))
		} else {
			for _, msg := range messages {
				timeStr := msg.CreatedAt.Format("15:04:05 | Jan 02")
				isSelf := strings.EqualFold(msg.SenderUsername, currentUsername)

				var senderTitle string
				if isSelf {
					senderTitle = fmt.Sprintf("You (%s) • %s", msg.SenderUsername, timeStr)
				} else {
					senderTitle = fmt.Sprintf("%s • %s", msg.SenderUsername, timeStr)
				}

				headerLbl := widget.NewLabelWithStyle(senderTitle, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
				bodyLbl := widget.NewLabel(msg.MessageText)
				bodyLbl.Wrapping = fyne.TextWrapWord

				bubbleContent := container.NewVBox(headerLbl, bodyLbl)
				card := widget.NewCard("", "", bubbleContent)

				// Constrain bubble width and align (outgoing to right, incoming to left)
				bubbleWrapper := container.NewGridWrap(fyne.NewSize(450, card.MinSize().Height), card)

				if isSelf {
					row := container.NewBorder(nil, nil, layout.NewSpacer(), nil, bubbleWrapper)
					messageListContainer.Add(row)
				} else {
					row := container.NewBorder(nil, nil, nil, layout.NewSpacer(), bubbleWrapper)
					messageListContainer.Add(row)
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
			dialog.ShowInformation("Select Contact", "Please select an active contact thread first.", window)
			return
		}

		err := db.ExecuteSendMessage(currentUsername, targetUser, text)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to send message: %v", err), window)
			return
		}

		messageInput.SetText("")
		refreshChatHistory(true)
	}

	sendBtn := widget.NewButtonWithIcon("Send Message", theme.MailSendIcon(), sendMessage)
	sendBtn.Importance = widget.HighImportance

	messageInput.OnSubmitted = func(_ string) {
		sendMessage()
	}

	inputRow := container.NewBorder(nil, nil, nil, sendBtn, messageInput)

	// 4. Contact Selector Panel Setup
	var contactSelector fyne.CanvasObject

	if isAdmin {
		labelToUsername := make(map[string]string)
		threadSelectDropdown := widget.NewSelect([]string{}, nil)
		threadSelectDropdown.PlaceHolder = "[ Select User Conversation ]"

		loadUserThreads := func() {
			threads, err := db.FetchActiveUserThreads(currentUsername)
			if err != nil || len(threads) == 0 {
				threadSelectDropdown.Options = []string{"No user threads registered"}
				threadSelectDropdown.Refresh()
				return
			}

			var options []string
			labelToUsername = make(map[string]string)

			for _, t := range threads {
				var opt string
				if t.UnreadCount > 0 {
					opt = fmt.Sprintf("🔴 %s (%d unread) - %s", t.Username, t.UnreadCount, t.LastMessage)
				} else if t.LastMessage != "No messages yet" {
					opt = fmt.Sprintf("💬 %s - %s", t.Username, t.LastMessage)
				} else {
					opt = fmt.Sprintf("👤 %s (New User)", t.Username)
				}
				options = append(options, opt)
				labelToUsername[opt] = t.Username
			}

			threadSelectDropdown.Options = options
			threadSelectDropdown.Refresh()

			if len(options) > 0 {
				threadSelectDropdown.SetSelected(options[0])
			}
		}

		threadSelectDropdown.OnChanged = func(selected string) {
			if selected == "" || selected == "No user threads registered" {
				return
			}

			u, exists := labelToUsername[selected]
			if exists && u != "" {
				targetUser = u
			} else {
				// Fallback cleanup
				clean := strings.TrimPrefix(selected, "🔴 ")
				clean = strings.TrimPrefix(clean, "💬 ")
				clean = strings.TrimPrefix(clean, "👤 ")
				parts := strings.Split(clean, " ")
				if len(parts) > 0 {
					targetUser = parts[0]
				}
			}

			headerStatusLabel.SetText(fmt.Sprintf("💬 Conversation with User: %s", targetUser))
			refreshChatHistory(true)
		}

		contactSelector = container.NewVBox(
			widget.NewLabelWithStyle("User Support Queue:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			threadSelectDropdown,
		)

		loadUserThreads()
	} else {
		adminSelectDropdown := widget.NewSelect([]string{}, nil)
		adminSelectDropdown.PlaceHolder = "[ Select System Administrator ]"

		adminSelectDropdown.OnChanged = func(selected string) {
			if selected != "" {
				targetUser = selected
				headerStatusLabel.SetText(fmt.Sprintf("💬 Support Channel with Admin: %s", targetUser))
				refreshChatHistory(true)
			}
		}

		admins, err := db.FetchAdminList()
		if err == nil && len(admins) > 0 {
			adminSelectDropdown.Options = admins
			adminSelectDropdown.SetSelected(admins[0])
			targetUser = admins[0]
			headerStatusLabel.SetText(fmt.Sprintf("💬 Support Channel with Admin: %s", targetUser))
			refreshChatHistory(true)
		} else {
			adminSelectDropdown.Options = []string{"admin"}
			adminSelectDropdown.SetSelected("admin")
			targetUser = "admin"
			headerStatusLabel.SetText("💬 Support Channel with Central Admin")
			refreshChatHistory(true)
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

	chatMainView := container.NewBorder(topPanel, container.NewPadded(inputRow), nil, nil, chatScroll)

	chatDialog := dialog.NewCustom("💬 eGPF Enterprise Support Chat Portal", "Close Portal", chatMainView, window)
	chatDialog.Resize(fyne.NewSize(700, 560))

	// 5. Real-time Background Polling Ticker
	stopPolling := make(chan struct{})
	var isClosed bool
	var pollLock sync.Mutex

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pollLock.Lock()
				closed := isClosed
				pollLock.Unlock()
				if closed {
					return
				}
				if targetUser != "" {
					refreshChatHistory(false)
				}
			case <-stopPolling:
				return
			}
		}
	}()

	chatDialog.SetOnClosed(func() {
		pollLock.Lock()
		isClosed = true
		pollLock.Unlock()
		select {
		case <-stopPolling:
		default:
			close(stopPolling)
		}
	})

	chatDialog.Show()
}
