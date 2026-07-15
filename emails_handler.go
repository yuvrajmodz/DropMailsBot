package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/jhillyerd/enmime"
	_ "github.com/mattn/go-sqlite3"
)

var ErrEmailExists = errors.New("email already exists")

var Domains = "nacdevs.qzz.io,xtg.qzz.io"

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
	dbFile = "data/UsersMaiL.db"
)

var db *sql.DB

func initDB() error {
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("ensureDataDir: %w", err)
	}

	var err error
	db, err = sql.Open("sqlite3", dbFile+"?_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	createTableQuery := `
	CREATE TABLE IF NOT EXISTS emails (
		email TEXT PRIMARY KEY,
		chat_id INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_chat_id ON emails(chat_id);
	`
	if _, err := db.Exec(createTableQuery); err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	return nil
}

func CloseDB() {
	if db != nil {
		db.Close()
	}
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
	for i := 0; i < 5; i++ { // retry up to 5 times for unique constraint
		email := generateRandomEmail(domainStr)
		_, err := db.Exec("INSERT INTO emails (email, chat_id) VALUES (?, ?)", email, chatID)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				continue
			}
			return "", fmt.Errorf("db.Exec: %w", err)
		}
		return email, nil
	}
	return "", errors.New("failed to generate unique email")
}

func generateAndSaveCustomEmail(chatID int64, name, domainStr string) (string, error) {
	email := generateCustomEmail(name, domainStr)

	_, err := db.Exec("INSERT INTO emails (email, chat_id) VALUES (?, ?)", email, chatID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return "", ErrEmailExists
		}
		return "", fmt.Errorf("db.Exec: %w", err)
	}

	return email, nil
}

func getUserEmail(chatID int64) string {
	var email string
	err := db.QueryRow("SELECT email FROM emails WHERE chat_id = ? ORDER BY created_at DESC LIMIT 1", chatID).Scan(&email)
	if err != nil {
		return ""
	}
	return email
}

func getChatIDForEmail(email string) int64 {
	var chatID int64
	err := db.QueryRow("SELECT chat_id FROM emails WHERE email = ?", email).Scan(&chatID)
	if err != nil {
		return 0
	}
	return chatID
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
