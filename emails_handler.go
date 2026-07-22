package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"math/rand"
	"mime"
	"mime/multipart"
	netsmtp "net/smtp"
	"net/textproto"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/jhillyerd/enmime"
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
	dbFile = "data/UsersMaiL.json"
)

type EmailData struct {
	ChatID    int64 `json:"chat_id"`
	Timestamp int64 `json:"timestamp"`
}

type StoreData struct {
	Emails map[string]EmailData `json:"emails"`
	Users  map[string]string    `json:"users"`
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
				Users:  make(map[string]string),
			}
			return saveStoreLocked()
		}
		return fmt.Errorf("read db file: %w", err)
	}

	var loaded StoreData
	if err := json.Unmarshal(data, &loaded); err != nil {
		store = StoreData{
			Emails: make(map[string]EmailData),
			Users:  make(map[string]string),
		}
		return saveStoreLocked()
	}

	if loaded.Emails == nil {
		loaded.Emails = make(map[string]EmailData)
	}
	if loaded.Users == nil {
		loaded.Users = make(map[string]string)
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

func ensureUserInStore(userID int64) {
	storeMutex.Lock()
	defer storeMutex.Unlock()
	
	idStr := fmt.Sprintf("%d", userID)
	if store.Users == nil {
		store.Users = make(map[string]string)
	}
	if _, exists := store.Users[idStr]; !exists {
		store.Users[idStr] = "nomail"
		_ = saveStoreLocked()
	}
}

func getForwardEmail(userID int64) string {
	storeMutex.RLock()
	defer storeMutex.RUnlock()
	
	idStr := fmt.Sprintf("%d", userID)
	if store.Users == nil {
		return "nomail"
	}
	if email, exists := store.Users[idStr]; exists {
		return email
	}
	return "nomail"
}

func setForwardEmail(userID int64, email string) {
	storeMutex.Lock()
	defer storeMutex.Unlock()
	
	idStr := fmt.Sprintf("%d", userID)
	if store.Users == nil {
		store.Users = make(map[string]string)
	}
	store.Users[idStr] = email
	_ = saveStoreLocked()
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

		forwardEmail := getForwardEmail(chatID)
		if forwardEmail != "" && forwardEmail != "nomail" && strings.HasSuffix(strings.ToLower(forwardEmail), "@gmail.com") {
			go func(toGmail, fEmail, tAddr, sub string, envelope *enmime.Envelope) {
				_ = forwardEmailToGmail(toGmail, fEmail, tAddr, sub, envelope)
			}(forwardEmail, fromEmail, toAddr, subject, env)
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

type mimeLineWrapper struct {
	w   io.Writer
	len int
}

func (l *mimeLineWrapper) Write(p []byte) (n int, err error) {
	for _, b := range p {
		if l.len >= 76 {
			if _, err := l.w.Write([]byte("\r\n")); err != nil {
				return n, err
			}
			l.len = 0
		}
		if _, err := l.w.Write([]byte{b}); err != nil {
			return n, err
		}
		l.len++
		n++
	}
	return n, nil
}

func forwardEmailToGmail(toGmail, fromEmail, toAddr, subject string, env *enmime.Envelope) error {
	banner := `<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="margin:0 0 18px 0;font-family:Arial,Helvetica,sans-serif;border:1px solid #f4d03f;border-radius:12px;background:#fffdf5;">
  <tr>
    <td style="padding:18px 22px;background:#f4b400;border-radius:12px 12px 0 0;">
      <div style="font-size:20px;font-weight:700;color:#111827;line-height:1;">DropMails</div>
      <div style="margin-top:6px;font-size:13px;color:#5f4300;">Forwarded Email</div>
    </td>
  </tr>
  <tr>
    <td style="padding:22px;">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="border-collapse:collapse;font-size:14px;">
        <tr>
          <td width="70" style="padding:11px 0;color:#8a6d00;font-weight:600;">From</td>
          <td style="padding:11px 0;color:#1f2937;">{{FROM}}</td>
        </tr>
        <tr>
          <td colspan="2" style="border-top:1px solid #f7df87;"></td>
        </tr>
        <tr>
          <td width="70" style="padding:11px 0;color:#8a6d00;font-weight:600;">To</td>
          <td style="padding:11px 0;color:#1f2937;">{{TO}}</td>
        </tr>
      </table>
    </td>
  </tr>
</table>
<hr style="border:none;border-top:2px solid #f4d03f;margin:22px 0;">`

	banner = strings.ReplaceAll(banner, "{{FROM}}", html.EscapeString(fromEmail))
	banner = strings.ReplaceAll(banner, "{{TO}}", html.EscapeString(toAddr))

	var htmlBody string
	if env.HTML != "" {
		htmlBody = banner + "\n<br>\n" + env.HTML
	} else {
		htmlBody = banner + "\n<br>\n<pre style=\"white-space: pre-wrap;\">" + html.EscapeString(env.Text) + "</pre>"
	}

	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)

	buf.WriteString("From: DropMails Bot <dropmails.services@gmail.com>\r\n")
	buf.WriteString(fmt.Sprintf("To: %s\r\n", toGmail))
	
	encodedSubject := mime.QEncoding.Encode("utf-8", subject)
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", encodedSubject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n\r\n", writer.Boundary()))

	htmlHeader := make(textproto.MIMEHeader)
	htmlHeader.Set("Content-Type", "text/html; charset=utf-8")
	htmlHeader.Set("Content-Transfer-Encoding", "base64")
	
	htmlPart, _ := writer.CreatePart(htmlHeader)
	
	b64Writer := base64.NewEncoder(base64.StdEncoding, &mimeLineWrapper{w: htmlPart})
	b64Writer.Write([]byte(htmlBody))
	b64Writer.Close()

	for _, att := range env.Attachments {
		writeEmailPart(writer, att, false)
	}

	for _, inl := range env.Inlines {
		writeEmailPart(writer, inl, true)
	}

	writer.Close()

	auth := netsmtp.PlainAuth("", "dropmails.services@gmail.com", "hwzo pbmo ebdc oxkm", "smtp.gmail.com")
	return netsmtp.SendMail("smtp.gmail.com:587", auth, "dropmails.services@gmail.com", []string{toGmail}, buf.Bytes())
}

func writeEmailPart(writer *multipart.Writer, p *enmime.Part, isInline bool) {
	h := make(textproto.MIMEHeader)
	contentType := p.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	
	h.Set("Content-Type", contentType)
	h.Set("Content-Transfer-Encoding", "base64")
	
	if isInline && p.ContentID != "" {
		h.Set("Content-ID", fmt.Sprintf("<%s>", p.ContentID))
		h.Set("Content-Disposition", "inline")
	} else {
		h.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, p.FileName))
	}
	
	part, err := writer.CreatePart(h)
	if err == nil {
		b64Writer := base64.NewEncoder(base64.StdEncoding, &mimeLineWrapper{w: part})
		b64Writer.Write(p.Content)
		b64Writer.Close()
	}
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
