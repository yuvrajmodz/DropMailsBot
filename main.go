package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/jhillyerd/enmime"
	"github.com/joho/godotenv"
)


const (
	cbGenRandom = "gen_random"
	cbGenCustom = "gen_custom"
	cbGenCancel = "gen_cancel"
)

type UserState struct {
	State      string
	MsgID      int
	Peer       tg.InputPeerClass
	CustomName string
}

var (
	userStates      = make(map[int64]UserState)
	userStatesMutex sync.RWMutex
)

const StateWaitingCustomName = "waiting_custom_name"
const StateWaitingDomainCustom = "waiting_domain_custom"

var validNameRegex = regexp.MustCompile(`^[a-z0-9.]+$`)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("⚠️  Error loading .env file: %v", err)
	}

	apiID, _ := strconv.ParseInt(os.Getenv("API_ID"), 10, 64)
	apiHash := os.Getenv("API_HASH")
	botToken := os.Getenv("BOT_TOKEN")

	if apiID == 0 || apiHash == "" || botToken == "" {
		log.Fatalf("❌ Missing required credentials in .env file")
	}

	if err := initDB(); err != nil {
		log.Fatalf("❌ Failed to initialize database: %v", err)
	}
	defer CloseDB()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	backupManager := NewBackupManager(ctx)
	if err := backupManager.Start(); err != nil {
		log.Printf("⚠️  Failed to start backup manager: %v", err)
	}
	defer backupManager.Stop()

	dispatcher := tg.NewUpdateDispatcher()
	var api *tg.Client

	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		msg, ok := update.Message.(*tg.Message)
		if !ok || msg.Out {
			return nil
		}
		if api == nil {
			return nil
		}

		text := strings.TrimSpace(msg.Message)

		user := resolveUser(msg, e)

		var err error
		switch {
		case strings.HasPrefix(text, "/start"):
			if user != nil {
				userStatesMutex.Lock()
				delete(userStates, user.ID)
				userStatesMutex.Unlock()
			}
			err = handleStart(ctx, api, user)
		case strings.HasPrefix(text, "/status"):
			if user != nil {
				userStatesMutex.Lock()
				delete(userStates, user.ID)
				userStatesMutex.Unlock()
			}
			err = handleStatus(ctx, api, user)
		case strings.HasPrefix(text, "/about"):
			if user != nil {
				userStatesMutex.Lock()
				delete(userStates, user.ID)
				userStatesMutex.Unlock()
			}
			err = handleAbout(ctx, api, user)
		case strings.HasPrefix(text, "/gen"):
			if user != nil {
				userStatesMutex.Lock()
				delete(userStates, user.ID)
				userStatesMutex.Unlock()
			}
			err = handleGen(ctx, api, user)
		default:
			if user != nil {
				userStatesMutex.RLock()
				state, ok := userStates[user.ID]
				userStatesMutex.RUnlock()
				if ok && state.State == StateWaitingCustomName {
					err = handleCustomNameInput(ctx, api, user, text, state)
					return err
				}
			}
		}

		if err != nil {
		}
		return err
	})

	dispatcher.OnBotCallbackQuery(func(ctx context.Context, e tg.Entities, update *tg.UpdateBotCallbackQuery) error {
		if api == nil {
			return nil
		}

		callbackData := string(update.Data)

		_, ackErr := api.MessagesSetBotCallbackAnswer(ctx, &tg.MessagesSetBotCallbackAnswerRequest{
			QueryID: update.QueryID,
			Alert:   false,
		})
		if ackErr != nil {
		}

		peer := buildInputPeerFromUpdate(update.Peer, e)
		if peer == nil {
			return nil
		}

		var user *tg.User
		if u, ok := e.Users[update.UserID]; ok {
			user = u
		} else {
			user = &tg.User{ID: update.UserID}
		}

		msgID := update.MsgID

		var err error
		switch callbackData {
		case cbGenRandom:
			err = handleCallbackGenRandom(ctx, api, user, peer, msgID)
		case cbGenCustom:
			err = handleCallbackGenCustom(ctx, api, user, peer, msgID)
		case cbGenCancel:
			err = handleCallbackGenCancel(ctx, api, user, peer, msgID)
		default:
			if strings.HasPrefix(callbackData, "cb_dom_rnd_") {
				idxStr := strings.TrimPrefix(callbackData, "cb_dom_rnd_")
				idx, _ := strconv.Atoi(idxStr)
				domains := getDomains()
				if idx >= 0 && idx < len(domains) {
					err = handleDomainSelectionRandom(ctx, api, user, peer, msgID, domains[idx])
				}
			} else if strings.HasPrefix(callbackData, "cb_dom_cst_") {
				idxStr := strings.TrimPrefix(callbackData, "cb_dom_cst_")
				idx, _ := strconv.Atoi(idxStr)
				domains := getDomains()
				if idx >= 0 && idx < len(domains) {
					err = handleDomainSelectionCustom(ctx, api, user, peer, msgID, domains[idx])
				}
			}
		}

		if err != nil {
		}
		return err
	})

	go StartSMTPServer()

	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		UpdateHandler: dispatcher,
	})

	if err := client.Run(ctx, func(ctx context.Context) error {
		if _, err := client.Auth().Bot(ctx, botToken); err != nil {
			return fmt.Errorf("auth failed: %w", err)
		}

		log.Println("✅ Bot authenticated!")
		api = client.API()

		telegramNotifier = func(ctx context.Context, chatID int64, fromEmail, toEmail, fromName, subject, body string, attachments []*enmime.Part) {
			sendEmailNotification(ctx, api, chatID, fromEmail, toEmail, fromName, subject, body, attachments)
		}

		if err := setBotCommands(ctx, api); err != nil {
		} else {
			log.Println("✅ Bot commands registered!")
		}

		log.Println("🤖 TempMiddle Bot is running... Press Ctrl+C to stop.")
		<-ctx.Done()
		return nil
	}); err != nil {
		log.Fatalf("❌ Bot error: %v", err)
	}
}

func buildInputPeerFromUpdate(peer tg.PeerClass, e tg.Entities) tg.InputPeerClass {
	switch p := peer.(type) {
	case *tg.PeerUser:
		if u, ok := e.Users[p.UserID]; ok {
			return &tg.InputPeerUser{
				UserID:     u.ID,
				AccessHash: u.AccessHash,
			}
		}
		return &tg.InputPeerUser{UserID: p.UserID}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}
	case *tg.PeerChannel:
		if ch, ok := e.Channels[p.ChannelID]; ok {
			return &tg.InputPeerChannel{
				ChannelID:  ch.ID,
				AccessHash: ch.AccessHash,
			}
		}
		return &tg.InputPeerChannel{ChannelID: p.ChannelID}
	}
	return nil
}

func resolveUser(msg *tg.Message, e tg.Entities) *tg.User {
	if msg.FromID != nil {
		if p, ok := msg.FromID.(*tg.PeerUser); ok {
			if u, exists := e.Users[p.UserID]; exists {
				return u
			}
		}
	}
	if p, ok := msg.PeerID.(*tg.PeerUser); ok {
		if u, exists := e.Users[p.UserID]; exists {
			return u
		}
	}
	return nil
}

func userToInputPeer(user *tg.User) tg.InputPeerClass {
	if user == nil {
		return nil
	}
	return &tg.InputPeerUser{
		UserID:     user.ID,
		AccessHash: user.AccessHash,
	}
}

func setBotCommands(ctx context.Context, api *tg.Client) error {
	_, err := api.BotsSetBotCommands(ctx, &tg.BotsSetBotCommandsRequest{
		Scope:    &tg.BotCommandScopeDefault{},
		LangCode: "",
		Commands: []tg.BotCommand{
			{Command: "start", Description: "Tᴏ Sᴛᴀʀᴛ Oʀ Rᴇsᴛᴀʀᴛ"},
			{Command: "gen", Description: "Tᴏ Gᴇɴᴇʀᴀᴛᴇ Tᴇᴍᴘ Eᴍᴀɪʟ"},
			{Command: "status", Description: "Tᴏ Cʜᴇᴄᴋ Bᴏᴛ Sᴛᴀᴛᴜs"},
			{Command: "about", Description: "Aʙᴏᴜᴛ Bᴏᴛ & Dᴇᴠᴇʟᴏᴘᴇʀ"},
		},
	})
	return err
}

func handleStart(ctx context.Context, api *tg.Client, user *tg.User) error {
	firstName := "User"
	if user != nil && user.FirstName != "" {
		firstName = user.FirstName
	}

	line1 := fmt.Sprintf("Hᴇʏ %s 👋,", firstName)
	line2 := "Wᴇʟᴄᴏᴍᴇ Tᴏ Oᴜʀ TᴇᴍᴘMɪᴅᴅʟᴇ Bᴏᴛ, Wʜɪᴄʜ Cᴀɴ Hᴇʟᴘ Yᴏᴜ Gᴇɴᴇʀᴀᴛᴇ Tᴇᴍᴘ Eᴍᴀɪʟs,"
	line3 := "Sᴀғᴇʟʏ & Sᴇᴄᴜʀᴇʟʏ, Sʜᴀʀᴇ Wɪᴛʜ Aɴʏᴏɴᴇ."
	fullText := line1 + "\n\n" + line2 + "\n" + line3

	entities := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: utf16Len(fullText)},
	}
	return sendMessage(ctx, api, user, fullText, entities)
}

func handleStatus(ctx context.Context, api *tg.Client, user *tg.User) error {
	text := "Sᴛᴀᴛᴜs: Rᴜɴɴɪɴɢ 🟢"
	entities := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: utf16Len(text)},
	}
	return sendMessage(ctx, api, user, text, entities)
}

func handleAbout(ctx context.Context, api *tg.Client, user *tg.User) error {
	line1 := "Nᴏ Aᴅs, Nᴏ Cʜᴀʀɢᴇs, 𝟸𝟺/𝟽 Oɴʟɪɴᴇ."
	line2 := "Mᴀɴᴀɢᴇʀ: @lieqy"
	fullText := line1 + "\n\n" + line2

	prefixUTF16 := utf16Len(line1) + utf16Len("\n\n") + utf16Len("Mᴀɴᴀɢᴇʀ: ")

	entities := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: utf16Len(fullText)},
		&tg.MessageEntityMention{Offset: prefixUTF16, Length: utf16Len("@lieqy")},
	}
	return sendMessage(ctx, api, user, fullText, entities)
}

func handleGen(ctx context.Context, api *tg.Client, user *tg.User) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}

	firstName := "User"
	if user.FirstName != "" {
		firstName = user.FirstName
	}

	line1 := fmt.Sprintf("Oᴋ,, %s", firstName)
	line2 := "Wʜᴀᴛ Tʏᴘᴇ Oғ Eᴍᴀɪʟ Dᴏ Yᴏᴜ Nᴇᴇᴅ?"
	fullText := line1 + "\n\n" + line2

	entities := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: utf16Len(fullText)},
	}

	markup := &tg.ReplyInlineMarkup{
		Rows: []tg.KeyboardButtonRow{
			{
				Buttons: []tg.KeyboardButtonClass{
					&tg.KeyboardButtonCallback{
						Text: "Rᴀɴᴅᴏᴍ Nᴀᴍᴇ Eᴍᴀɪʟ",
						Data: []byte(cbGenRandom),
					},
				},
			},
			{
				Buttons: []tg.KeyboardButtonClass{
					&tg.KeyboardButtonCallback{
						Text: "Cᴜsᴛᴏᴍ Nᴀᴍᴇ Eᴍᴀɪʟ",
						Data: []byte(cbGenCustom),
					},
				},
			},
			{
				Buttons: []tg.KeyboardButtonClass{
					&tg.KeyboardButtonCallback{
						Text: "Cᴀɴᴄᴇʟ",
						Data: []byte(cbGenCancel),
					},
				},
			},
		},
	}

	peer := userToInputPeer(user)
	if peer == nil {
		return fmt.Errorf("could not resolve peer")
	}

	_, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:        peer,
		Message:     fullText,
		Entities:    entities,
		ReplyMarkup: markup,
		RandomID:    rand.Int63(),
		NoWebpage:   true,
	})
	return err
}

func handleCallbackGenRandom(ctx context.Context, api *tg.Client, user *tg.User, peer tg.InputPeerClass, msgID int) error {
	return promptDomainSelection(ctx, api, peer, msgID, true)
}

func handleCallbackGenCustom(ctx context.Context, api *tg.Client, user *tg.User, peer tg.InputPeerClass, msgID int) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}

	text := "Eɴᴛᴇʀ Nᴀᴍᴇ:"
	entities := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: utf16Len(text)},
	}
	markup := &tg.ReplyInlineMarkup{
		Rows: []tg.KeyboardButtonRow{
			{
				Buttons: []tg.KeyboardButtonClass{
					&tg.KeyboardButtonCallback{
						Text: "Cᴀɴᴄᴇʟ",
						Data: []byte(cbGenCancel),
					},
				},
			},
		},
	}

	_ = deleteMessage(ctx, api, peer, msgID)

	newMsgID, err := sendMessageWithMarkupAndGetID(ctx, api, peer, text, entities, markup)
	if err != nil {
		return err
	}
	if newMsgID == 0 {
		newMsgID = msgID
	}

	userStatesMutex.Lock()
	userStates[user.ID] = UserState{
		State: StateWaitingCustomName,
		MsgID: newMsgID,
		Peer:  peer,
	}
	userStatesMutex.Unlock()

	return nil
}

func handleCallbackGenCancel(ctx context.Context, api *tg.Client, user *tg.User, peer tg.InputPeerClass, msgID int) error {
	if user != nil {
		userStatesMutex.Lock()
		delete(userStates, user.ID)
		userStatesMutex.Unlock()
	}

	text := "Cᴀɴᴄᴇʟʟᴇᴅ 🔴"
	entities := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: utf16Len(text)},
	}
	return editMessage(ctx, api, peer, msgID, text, entities)
}

func handleCustomNameInput(ctx context.Context, api *tg.Client, user *tg.User, text string, state UserState) error {
	name := strings.TrimSpace(text)

	if !validNameRegex.MatchString(name) {
		_ = deleteMessage(ctx, api, state.Peer, state.MsgID)
		
		line1 := "Wʀᴏɴɢ Eᴍᴀɪʟ's ɴᴀᴍᴇ 🔴"
		line2 := "Dᴏɴ'ᴛ Usᴇ Aɴʏ Sᴘᴀᴄᴇ Oʀ Sᴘᴇᴄɪᴀʟ Cʜᴀʀᴀᴄᴛᴇʀ,"
		line3 := "Usᴇ Oɴʟʏ Sᴍᴀʟʟ Aʟᴘʜᴀʙᴇᴛs, Nᴜᴍʙᴇʀs Aɴᴅ Dᴏᴛ."
		line4 := "Tʀʏ Aɢᴀɪɴ –"
		fullText := line1 + "\n\n" + line2 + "\n" + line3 + "\n\n" + line4

		entities := []tg.MessageEntityClass{
			&tg.MessageEntityBold{Offset: 0, Length: utf16Len(line1)},
			&tg.MessageEntityBold{Offset: utf16Len(line1 + "\n\n"), Length: utf16Len(line2)},
			&tg.MessageEntityBold{Offset: utf16Len(line1 + "\n\n" + line2 + "\n"), Length: utf16Len(line3)},
			&tg.MessageEntityBold{Offset: utf16Len(line1 + "\n\n" + line2 + "\n" + line3 + "\n\n"), Length: utf16Len(line4)},
		}

		markup := &tg.ReplyInlineMarkup{
			Rows: []tg.KeyboardButtonRow{
				{
					Buttons: []tg.KeyboardButtonClass{
						&tg.KeyboardButtonCallback{
							Text: "Cᴀɴᴄᴇʟ",
							Data: []byte(cbGenCancel),
						},
					},
				},
			},
		}

		newMsgID, err := sendMessageWithMarkupAndGetID(ctx, api, state.Peer, fullText, entities, markup)
		if err == nil && newMsgID != 0 {
			userStatesMutex.Lock()
			userStates[user.ID] = UserState{
				State: StateWaitingCustomName,
				MsgID: newMsgID,
				Peer:  state.Peer,
			}
			userStatesMutex.Unlock()
		}
		return err
	}

	userStatesMutex.Lock()
	userStates[user.ID] = UserState{
		State:      StateWaitingDomainCustom,
		MsgID:      0,
		Peer:       state.Peer,
		CustomName: name,
	}
	userStatesMutex.Unlock()

	_ = deleteMessage(ctx, api, state.Peer, state.MsgID)

	return promptDomainSelection(ctx, api, state.Peer, 0, false)
}

func promptDomainSelection(ctx context.Context, api *tg.Client, peer tg.InputPeerClass, msgID int, isRandom bool) error {
	text := "Cʜᴏᴏsᴇ Exᴛᴇɴsɪᴏɴ Fᴏʀ Yᴏᴜʀ Eᴍᴀɪʟ..."
	entities := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: utf16Len(text)},
	}

	domains := getDomains()
	var rows []tg.KeyboardButtonRow
	for i, d := range domains {
		var cbData string
		if isRandom {
			cbData = fmt.Sprintf("cb_dom_rnd_%d", i)
		} else {
			cbData = fmt.Sprintf("cb_dom_cst_%d", i)
		}
		rows = append(rows, tg.KeyboardButtonRow{
			Buttons: []tg.KeyboardButtonClass{
				&tg.KeyboardButtonCallback{
					Text: d,
					Data: []byte(cbData),
				},
			},
		})
	}
	rows = append(rows, tg.KeyboardButtonRow{
		Buttons: []tg.KeyboardButtonClass{
			&tg.KeyboardButtonCallback{
				Text: "Cᴀɴᴄᴇʟ",
				Data: []byte(cbGenCancel),
			},
		},
	})

	markup := &tg.ReplyInlineMarkup{Rows: rows}
	
	if msgID != 0 {
		return editMessageWithMarkup(ctx, api, peer, msgID, text, entities, markup)
	}
	_, err := sendMessageWithMarkupAndGetID(ctx, api, peer, text, entities, markup)
	return err
}

func handleDomainSelectionRandom(ctx context.Context, api *tg.Client, user *tg.User, peer tg.InputPeerClass, msgID int, domainStr string) error {
	email, err := generateAndSaveEmail(user.ID, domainStr)
	if err != nil {
		return err
	}

	line1 := "Tᴇᴍᴘ Mᴀɪʟ Gᴇɴᴇʀᴀᴛᴇᴅ 🟢"
	line2 := fmt.Sprintf("Eᴍᴀɪʟ: %s", email)
	fullText := line1 + "\n\n" + line2

	entities := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: utf16Len(fullText)},
	}

	return editMessage(ctx, api, peer, msgID, fullText, entities)
}

func handleDomainSelectionCustom(ctx context.Context, api *tg.Client, user *tg.User, peer tg.InputPeerClass, msgID int, domainStr string) error {
	userStatesMutex.RLock()
	state, ok := userStates[user.ID]
	userStatesMutex.RUnlock()

	if !ok || state.State != StateWaitingDomainCustom {
		return fmt.Errorf("invalid state for custom domain selection")
	}

	name := state.CustomName

	userStatesMutex.Lock()
	delete(userStates, user.ID)
	userStatesMutex.Unlock()

	email, err := generateAndSaveCustomEmail(user.ID, name, domainStr)
	if err == ErrEmailExists {
		line1 := "Aʟʀᴇᴀᴅʏ Exɪsᴛs 🔴"
		line2 := "Wɪᴛʜ Tʜɪs Nᴀᴍᴇ, Eᴍᴀɪʟ Aʟʀᴇᴀᴅʏ Exɪsᴛɪɴɢ."
		fullText := line1 + "\n\n" + line2
		entities := []tg.MessageEntityClass{
			&tg.MessageEntityBold{Offset: 0, Length: utf16Len(line1)},
			&tg.MessageEntityBold{Offset: utf16Len(line1 + "\n\n"), Length: utf16Len(line2)},
		}
		return editMessage(ctx, api, peer, msgID, fullText, entities)
	} else if err != nil {
		return err
	}

	line1 := "Tᴇᴍᴘ Mᴀɪʟ Gᴇɴᴇʀᴀᴛᴇᴅ 🟢"
	line2 := fmt.Sprintf("Eᴍᴀɪʟ: %s", email)
	fullText := line1 + "\n\n" + line2

	entities := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: utf16Len(fullText)},
	}

	return editMessage(ctx, api, peer, msgID, fullText, entities)
}

func sendMessage(ctx context.Context, api *tg.Client, user *tg.User, text string, entities []tg.MessageEntityClass) error {
	peer := userToInputPeer(user)
	if peer == nil {
		return fmt.Errorf("could not resolve peer")
	}
	_, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:      peer,
		Message:   text,
		Entities:  entities,
		RandomID:  rand.Int63(),
		NoWebpage: true,
	})
	return err
}

func editMessage(ctx context.Context, api *tg.Client, peer tg.InputPeerClass, msgID int, text string, entities []tg.MessageEntityClass) error {
	_, err := api.MessagesEditMessage(ctx, &tg.MessagesEditMessageRequest{
		Peer:      peer,
		ID:        msgID,
		Message:   text,
		Entities:  entities,
		NoWebpage: true,
	})
	return err
}

func editMessageWithMarkup(ctx context.Context, api *tg.Client, peer tg.InputPeerClass, msgID int, text string, entities []tg.MessageEntityClass, markup tg.ReplyMarkupClass) error {
	req := &tg.MessagesEditMessageRequest{
		Peer:      peer,
		ID:        msgID,
		Message:   text,
		Entities:  entities,
		NoWebpage: true,
	}
	if markup != nil {
		req.ReplyMarkup = markup
	}
	_, err := api.MessagesEditMessage(ctx, req)
	return err
}

func deleteMessage(ctx context.Context, api *tg.Client, peer tg.InputPeerClass, msgID int) error {
	if ch, ok := peer.(*tg.InputPeerChannel); ok {
		_, err := api.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{
				ChannelID:  ch.ChannelID,
				AccessHash: ch.AccessHash,
			},
			ID:      []int{msgID},
		})
		return err
	}
	
	_, err := api.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
		Revoke: true,
		ID:     []int{msgID},
	})
	return err
}

func sendMessageWithMarkupAndGetID(ctx context.Context, api *tg.Client, peer tg.InputPeerClass, text string, entities []tg.MessageEntityClass, markup tg.ReplyMarkupClass) (int, error) {
	res, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:        peer,
		Message:     text,
		Entities:    entities,
		ReplyMarkup: markup,
		RandomID:    rand.Int63(),
		NoWebpage:   true,
	})
	if err != nil {
		return 0, err
	}
	
	switch u := res.(type) {
	case *tg.UpdateShortSentMessage:
		return u.ID, nil
	case *tg.Updates:
		for _, up := range u.Updates {
			if msg, ok := up.(*tg.UpdateNewMessage); ok {
				if m, ok := msg.Message.(*tg.Message); ok {
					return m.ID, nil
				}
			}
		}
	}
	return 0, nil
}

func utf16Len(s string) int {
	length := 0
	for _, r := range s {
		if r > 0xFFFF {
			length += 2
		} else {
			length++
		}
	}
	return length
}

type botAPIResponse struct {
	Ok     bool `json:"ok"`
	Result []struct {
		MessageID int `json:"message_id"`
	} `json:"result"`
}

func sendMediaGroupBotAPI(chatID int64, attachments []*enmime.Part) (int, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	writer.WriteField("chat_id", strconv.FormatInt(chatID, 10))

	var mediaJSON []map[string]interface{}

	for i, att := range attachments {
		if att == nil || len(att.Content) == 0 {
			continue
		}
		fileName := att.FileName
		if fileName == "" {
			fileName = fmt.Sprintf("attachment_%d", i)
		}

		partName := fmt.Sprintf("file%d", i)
		
		part, err := writer.CreateFormFile(partName, fileName)
		if err != nil {
			return 0, err
		}
		_, err = io.Copy(part, bytes.NewReader(att.Content))
		if err != nil {
			return 0, err
		}

		mediaItem := map[string]interface{}{
			"type":  "document",
			"media": "attach://" + partName,
		}
		mediaJSON = append(mediaJSON, mediaItem)
	}

	mediaBytes, _ := json.Marshal(mediaJSON)
	writer.WriteField("media", string(mediaBytes))
	
	writer.Close()

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMediaGroup", botToken)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var apiResp botAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return 0, err
	}

	if !apiResp.Ok || len(apiResp.Result) == 0 {
		return 0, fmt.Errorf("bot api error or empty result")
	}

	return apiResp.Result[0].MessageID, nil
}

func sendEmailNotification(ctx context.Context, api *tg.Client, chatID int64, fromEmail, toEmail, fromName, subject, body string, attachments []*enmime.Part) {
	peer := &tg.InputPeerUser{UserID: chatID}

	line1Text := "Eᴍᴀɪʟ Rᴇᴄᴇɪᴠᴇᴅ 🟢"
	
	var baseData string
	baseData += fmt.Sprintf("From: %s\n", fromEmail)
	baseData += fmt.Sprintf("To: %s", toEmail)
	if fromName != "" {
		baseData += fmt.Sprintf("\nFrom_Name: %s", fromName)
	}

	subjectData := strings.TrimSpace(subject)
	if subjectData == "" {
		subjectData = "(No Subject)"
	}

	bodyData := strings.TrimSpace(body)
	if bodyData == "" {
		bodyData = "(Empty Body)"
	}
	
	runes := []rune(bodyData)
	limit := 3000
	if len(attachments) > 0 {
		limit = 800
	}
	if len(runes) > limit {
		bodyData = string(runes[:limit]) + "..."
	}

	caption := line1Text + "\n\n"
	
	baseDataOffset := utf16Len(caption)
	caption += baseData
	
	caption += "\n\n"
	subjectDataOffset := utf16Len(caption)
	caption += subjectData
	
	caption += "\n\n"
	bodyDataOffset := utf16Len(caption)
	caption += bodyData

	entities := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: utf16Len(line1Text)},
		&tg.MessageEntityPre{Offset: baseDataOffset, Length: utf16Len(baseData), Language: "Bᴀsᴇ Dᴀᴛᴀ"},
		&tg.MessageEntityPre{Offset: subjectDataOffset, Length: utf16Len(subjectData), Language: "Sᴜʙᴊᴇᴄᴛ"},
		&tg.MessageEntityPre{Offset: bodyDataOffset, Length: utf16Len(bodyData), Language: "Bᴏᴅʏ Mᴇssᴀɢᴇ"},
	}

	if len(attachments) == 0 {
		_, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
			Peer:      peer,
			Message:   caption,
			Entities:  entities,
			RandomID:  rand.Int63(),
			NoWebpage: true,
		})
		if err != nil {
		}
		return
	}

	if len(attachments) > 1 {
		msgID, err := sendMediaGroupBotAPI(chatID, attachments)
		if err == nil && msgID != 0 {
			replyTo := &tg.InputReplyToMessage{ReplyToMsgID: msgID}
			_, err = api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
				Peer:      peer,
				Message:   caption,
				Entities:  entities,
				ReplyTo:   replyTo,
				RandomID:  rand.Int63(),
				NoWebpage: true,
			})
			if err != nil {
			}
			return
		}
	}

	u := uploader.NewUploader(api)

	for _, att := range attachments {
		if att == nil || len(att.Content) == 0 {
			continue
		}

		fileName := att.FileName
		if fileName == "" {
			fileName = "attachment"
		}
		contentType := att.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		fileReader := bytes.NewReader(att.Content)
		inputFile, err := u.FromReader(ctx, fileName, fileReader)
		if err != nil {
			continue
		}

		media := &tg.InputMediaUploadedDocument{
			File: inputFile,
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeFilename{FileName: fileName},
			},
			MimeType: contentType,
		}

		_, err = api.MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
			Peer:     peer,
			Media:    media,
			Message:  caption,
			Entities: entities,
			RandomID: rand.Int63(),
		})
		if err != nil {
		}
	}
}
