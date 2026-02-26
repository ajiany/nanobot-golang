package channels

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/coopco/nanobot/internal/bus"
)

func init() {
	Register("email", newEmailChannel)
}

type emailConfig struct {
	IMAPServer   string   `json:"imapServer"`
	SMTPServer   string   `json:"smtpServer"`
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	AllowedUsers []string `json:"allowedUsers"`
}

// EmailChannel implements Channel using IMAP polling for receive and SMTP for send.
type EmailChannel struct {
	imapServer   string
	smtpServer   string
	username     string
	password     string
	bus          *bus.MessageBus
	allowedUsers map[string]bool
	cancel       context.CancelFunc
}

func newEmailChannel(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
	var c emailConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(c.AllowedUsers))
	for _, u := range c.AllowedUsers {
		allowed[u] = true
	}
	return &EmailChannel{
		imapServer:   c.IMAPServer,
		smtpServer:   c.SMTPServer,
		username:     c.Username,
		password:     c.Password,
		bus:          msgBus,
		allowedUsers: allowed,
	}, nil
}

func (c *EmailChannel) Name() string { return "email" }

func (c *EmailChannel) Start(ctx context.Context) error {
	pollCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		// Poll immediately on start
		c.pollInbox()
		for {
			select {
			case <-pollCtx.Done():
				return
			case <-ticker.C:
				c.pollInbox()
			}
		}
	}()

	return nil
}

// imapCmd sends an IMAP command and returns the response lines until a tagged response.
func imapCmd(conn *bufio.ReadWriter, tag, cmd string) ([]string, error) {
	line := fmt.Sprintf("%s %s\r\n", tag, cmd)
	if _, err := conn.WriteString(line); err != nil {
		return nil, err
	}
	if err := conn.Flush(); err != nil {
		return nil, err
	}
	var lines []string
	for {
		l, err := conn.ReadString('\n')
		if err != nil {
			return nil, err
		}
		l = strings.TrimRight(l, "\r\n")
		lines = append(lines, l)
		if strings.HasPrefix(l, tag+" ") {
			break
		}
	}
	return lines, nil
}

func (c *EmailChannel) pollInbox() {
	tlsCfg := &tls.Config{ServerName: strings.Split(c.imapServer, ":")[0]}
	rawConn, err := tls.Dial("tcp", c.imapServer, tlsCfg)
	if err != nil {
		// Try plain TCP if TLS fails (port 143)
		host := strings.Split(c.imapServer, ":")[0]
		rawConn2, err2 := net.Dial("tcp", c.imapServer)
		if err2 != nil {
			slog.Error("email: imap connect", "err", err)
			return
		}
		_ = host
		rw := bufio.NewReadWriter(bufio.NewReader(rawConn2), bufio.NewWriter(rawConn2))
		// Read greeting
		rw.ReadString('\n')
		c.processIMAP(rw)
		rawConn2.Close()
		return
	}
	defer rawConn.Close()
	rw := bufio.NewReadWriter(bufio.NewReader(rawConn), bufio.NewWriter(rawConn))
	// Read greeting
	rw.ReadString('\n')
	c.processIMAP(rw)
}

func (c *EmailChannel) processIMAP(rw *bufio.ReadWriter) {
	// LOGIN
	loginCmd := fmt.Sprintf("LOGIN %q %q", c.username, c.password)
	if _, err := imapCmd(rw, "a1", loginCmd); err != nil {
		slog.Error("email: imap login", "err", err)
		return
	}

	// SELECT INBOX
	if _, err := imapCmd(rw, "a2", "SELECT INBOX"); err != nil {
		slog.Error("email: imap select", "err", err)
		return
	}

	// SEARCH UNSEEN
	lines, err := imapCmd(rw, "a3", "SEARCH UNSEEN")
	if err != nil {
		slog.Error("email: imap search", "err", err)
		return
	}

	var uids []string
	for _, l := range lines {
		if strings.HasPrefix(l, "* SEARCH") {
			parts := strings.Fields(l)
			if len(parts) > 2 {
				uids = parts[2:]
			}
		}
	}

	for _, uid := range uids {
		fetchLines, err := imapCmd(rw, "a4", fmt.Sprintf("FETCH %s (BODY[HEADER.FIELDS (FROM SUBJECT)] BODY[TEXT])", uid))
		if err != nil {
			slog.Error("email: imap fetch", "err", err, "uid", uid)
			continue
		}

		from, subject, body := parseIMAPFetch(fetchLines)
		if !c.IsAllowed(from) {
			slog.Warn("email: message from disallowed user", "from", from)
		} else {
			c.bus.PublishInbound(bus.InboundMessage{
				Channel:  "email",
				SenderID: from,
				ChatID:   from,
				Content:  fmt.Sprintf("Subject: %s\n%s", subject, body),
			})
		}

		// Mark as seen
		imapCmd(rw, "a5", fmt.Sprintf("STORE %s +FLAGS (\\Seen)", uid))
	}

	imapCmd(rw, "a6", "LOGOUT")
}

func parseIMAPFetch(lines []string) (from, subject, body string) {
	inHeader := true
	var bodyLines []string
	for _, l := range lines {
		if inHeader {
			if strings.HasPrefix(strings.ToLower(l), "from:") {
				from = strings.TrimSpace(l[5:])
			} else if strings.HasPrefix(strings.ToLower(l), "subject:") {
				subject = strings.TrimSpace(l[8:])
			} else if l == "" {
				inHeader = false
			}
		} else {
			if !strings.HasPrefix(l, "* ") && !strings.HasPrefix(l, "a4 ") {
				bodyLines = append(bodyLines, l)
			}
		}
	}
	body = strings.Join(bodyLines, "\n")
	return
}

func (c *EmailChannel) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

func (c *EmailChannel) Send(msg bus.OutboundMessage) error {
	host := strings.Split(c.smtpServer, ":")[0]
	auth := smtp.PlainAuth("", c.username, c.password, host)

	body := fmt.Sprintf("To: %s\r\nSubject: Re: nanobot\r\n\r\n%s", msg.ChatID, msg.Content)
	err := smtp.SendMail(c.smtpServer, auth, c.username, []string{msg.ChatID}, []byte(body))
	if err != nil {
		return fmt.Errorf("email: send: %w", err)
	}
	return nil
}

func (c *EmailChannel) IsAllowed(senderID string) bool {
	if len(c.allowedUsers) == 0 {
		return true
	}
	return c.allowedUsers[senderID]
}
