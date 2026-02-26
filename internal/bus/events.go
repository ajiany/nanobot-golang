package bus

import "fmt"

// InboundMessage represents a message received from any channel.
type InboundMessage struct {
	Channel            string            // source channel name (e.g. "telegram", "discord", "system")
	SenderID           string            // sender identifier
	ChatID             string            // chat/conversation identifier
	Content            string            // text content
	Media              []Media           // attached media (images, audio, etc.)
	SessionKeyOverride string            // optional override for session routing
	Metadata           map[string]string // arbitrary metadata
}

// Media represents an attached media item.
type Media struct {
	Type     string // "image", "audio", "video", "file"
	URL      string // URL or file path
	MimeType string // MIME type
	Data     []byte // raw data (for inline media)
}

// SessionKey returns the routing key for session management.
// Uses SessionKeyOverride if set, otherwise "channel:chatID".
func (m InboundMessage) SessionKey() string {
	if m.SessionKeyOverride != "" {
		return m.SessionKeyOverride
	}
	return fmt.Sprintf("%s:%s", m.Channel, m.ChatID)
}

// OutboundMessage represents a message to be sent to a channel.
type OutboundMessage struct {
	Channel  string            // target channel
	ChatID   string            // target chat
	Content  string            // text content
	Type     string            // "text", "progress", "tool_hint", "error"
	ReplyTo  string            // optional message ID to reply to
	Metadata map[string]string // arbitrary metadata
}
