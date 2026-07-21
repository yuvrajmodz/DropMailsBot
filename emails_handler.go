package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/jhillyerd/enmime"
)

var ErrEmailExists = errors.New("email already exists")

var Domains = "nacdevs.qzz.io"

func getDomains() []string {
	var list []string
	for _, d := range strings.Split(Domains, ",") {
		trimmed := strings.TrimSpace(d)
		if trimmed != "" {
			list = append(list, trimmed)
		}
	}
	return list
}

const (
	dbDir  = "data"
	dbFile = "data/UsersMaiL.json"
)

type UserData struct {
}

type EmailData struct {
	ChatID    int64 `json:"chat_id"`
	Timestamp int64 `json:"timestamp"`
}

type StoreData struct {
	Emails map[string]EmailData `json:"emails"`
	Users  map[string]UserData  `json:"users"`
}

var (
	storeMutex sync.RWMutex
	store      StoreData
)

func initJSONStore() error {
	storeMutex.Lock()
	defer storeMutex.Unlock()
	return loadStoreLocked()
}

func reloadJSONStore() error {
	storeMutex.Lock()
	defer storeMutex.Unlock()
	return loadStoreLocked()
}

func loadStoreLocked() error {
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("ensureDataDir: %w", err)
	}

	data, err := os.ReadFile(dbFile)
	if err != nil {
		if os.IsNotExist(err) {
			store = StoreData{
				Emails: make(map[string]EmailData),
				Users:  make(map[string]UserData),
			}
			return saveStoreLocked()
		}
		return fmt.Errorf("read db file: %w", err)
	}

	var loaded StoreData
	if err := json.Unmarshal(data, &loaded); err != nil {
		store = StoreData{
			Emails: make(map[string]EmailData),
			Users:  make(map[string]UserData),
		}
		return saveStoreLocked()
	}

	if loaded.Emails == nil {
		loaded.Emails = make(map[string]EmailData)
	}
	if loaded.Users == nil {
		loaded.Users = make(map[string]UserData)
	}
	store = loaded
	return nil
}

func saveStoreLocked() error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dbFile, data, 0644)
}

func generateRandomEmail(domainStr string) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	const digits = "0123456789"

	username := make([]byte, 8)
	for i := 0; i < 5; i++ {
		username[i] = letters[rand.Intn(len(letters))]
	}
	for i := 5; i < 8; i++ {
		username[i] = digits[rand.Intn(len(digits))]
	}

	return fmt.Sprintf("%s@%s", string(username), domainStr)
}

func generateCustomEmail(name, domainStr string) string {
	return fmt.Sprintf("%s@%s", name, domainStr)
}

func generateAndSaveEmail(chatID int64, domainStr string) (string, error) {
	storeMutex.Lock()
	defer storeMutex.Unlock()

	for i := 0; i < 5; i++ {
		email := generateRandomEmail(domainStr)
		if _, exists := store.Emails[email]; exists {
			continue
		}

		store.Emails[email] = EmailData{
			ChatID:    chatID,
			Timestamp: time.Now().Unix(),
		}

		if err := saveStoreLocked(); err != nil {
			return "", fmt.Errorf("save email: %w", err)
		}
		return email, nil
	}
	return "", errors.New("failed to generate unique email")
}

func generateAndSaveCustomEmail(chatID int64, name, domainStr string) (string, error) {
	email := generateCustomEmail(name, domainStr)

	storeMutex.Lock()
	defer storeMutex.Unlock()

	if _, exists := store.Emails[email]; exists {
		return "", ErrEmailExists
	}

	store.Emails[email] = EmailData{
		ChatID:    chatID,
		Timestamp: time.Now().Unix(),
	}

	if err := saveStoreLocked(); err != nil {
		return "", fmt.Errorf("save email: %w", err)
	}
	return email, nil
}

func getUserEmail(chatID int64) string {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	var latestEmail string
	var latestTimestamp int64 = 0

	for email, data := range store.Emails {
		if data.ChatID == chatID && data.Timestamp > latestTimestamp {
			latestTimestamp = data.Timestamp
			latestEmail = email
		}
	}

	return latestEmail
}

func getChatIDForEmail(email string) int64 {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	if data, ok := store.Emails[email]; ok {
		return data.ChatID
	}
	return 0
}

var telegramNotifier func(ctx context.Context, chatID int64, fromEmail, toEmail, fromName, subject, body string, attachments []*enmime.Part)

type Backend struct{}

func (bkd *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{}, nil
}

var _ smtp.Backend = &Backend{}

type Session struct {
	From    string
	To      []string
	rawData []byte
}

func (s *Session) AuthPlain(username, password string) error {
	return smtp.ErrAuthUnsupported
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.From = from
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	valid := false
	for _, d := range getDomains() {
		if strings.HasSuffix(to, "@"+d) {
			valid = true
			break
		}
	}
	if !valid {
		return errors.New("invalid recipient domain")
	}
	s.To = append(s.To, to)
	return nil
}

func (s *Session) Data(r io.Reader) error {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return err
	}
	s.rawData = buf.Bytes()

	env, err := enmime.ReadEnvelope(bytes.NewReader(s.rawData))
	if err != nil {
		return nil
	}

	fromEmail := s.From
	fromName := ""
	if addrs, err := env.AddressList("From"); err == nil && len(addrs) > 0 {
		fromEmail = addrs[0].Address
		fromName = addrs[0].Name
	}

	subject := env.GetHeader("Subject")

	dateStr := env.GetHeader("Date")
	if dateStr == "" {
		dateStr = time.Now().Format(time.RFC3339)
	}

	body := env.Text
	if strings.TrimSpace(body) == "" {
		body = env.HTML
	}
	body = strings.TrimSpace(body)

	allAttachments := make([]*enmime.Part, 0, len(env.Attachments)+len(env.Inlines))
	allAttachments = append(allAttachments, env.Attachments...)
	allAttachments = append(allAttachments, env.Inlines...)

	for _, toAddr := range s.To {
		chatID := getChatIDForEmail(toAddr)
		if chatID == 0 {
			continue
		}

		if telegramNotifier != nil {
			ctx := context.Background()
			telegramNotifier(ctx, chatID, fromEmail, toAddr, fromName, subject, body, allAttachments)
		}
	}

	return nil
}

func (s *Session) Reset() {
	s.To = nil
	s.rawData = nil
}

func (s *Session) Logout() error {
	return nil
}

func StartSMTPServer() {
	be := &Backend{}
	srv := smtp.NewServer(be)

	srv.Addr = ":25"
	domains := getDomains()
	if len(domains) > 0 {
		srv.Domain = domains[0]
	}
	srv.ReadTimeout = 30 * time.Second
	srv.WriteTimeout = 30 * time.Second
	srv.MaxMessageBytes = 25 * 1024 * 1024
	srv.MaxRecipients = 50
	srv.AllowInsecureAuth = true

	log.Printf("🚀 Starting SMTP server on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("❌ SMTP Server error: %v", err)
	}
}
