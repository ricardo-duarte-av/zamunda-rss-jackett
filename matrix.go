package main

import (
	"log"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// MatrixClient handles Matrix operations
type MatrixClient struct {
	client *mautrix.Client
	roomID id.RoomID
}

// NewMatrixClient creates a new Matrix client
func NewMatrixClient(homeserver, userID, accessToken, roomID string) (*MatrixClient, error) {
	client, err := mautrix.NewClient(homeserver, id.UserID(userID), accessToken)
	if err != nil {
		return nil, err
	}

	return &MatrixClient{
		client: client,
		roomID: id.RoomID(roomID),
	}, nil
}

// SendMessage sends a text message to the configured room
func (mc *MatrixClient) SendMessage(message string) error {
	_, err := mc.client.SendText(mc.roomID, message)
	if err != nil {
		log.Printf("Failed to send Matrix message: %v", err)
		return err
	}
	log.Printf("Successfully sent Matrix message")
	return nil
}

// SendFormattedMessage sends a formatted message with HTML content
func (mc *MatrixClient) SendFormattedMessage(text, html string) error {
	content := &event.MessageEventContent{
		MsgType:       event.MsgText,
		Body:          text,
		Format:        event.FormatHTML,
		FormattedBody: html,
	}

	_, err := mc.client.SendMessageEvent(mc.roomID, event.EventMessage, content)
	if err != nil {
		log.Printf("Failed to send formatted Matrix message: %v", err)
		return err
	}
	log.Printf("Successfully sent formatted Matrix message")
	return nil
}

// SendGameNotification sends a formatted game notification
func (mc *MatrixClient) SendGameNotification(gameName, releaseDate, rating, genres, platforms, summary, downloadLink string) error {
	// Create plain text version
	textMessage := formatGameMessageText(gameName, releaseDate, rating, genres, platforms, summary, downloadLink)

	// Create HTML version
	htmlMessage := formatGameMessageHTML(gameName, releaseDate, rating, genres, platforms, summary, downloadLink)

	return mc.SendFormattedMessage(textMessage, htmlMessage)
}

// formatGameMessageText creates a plain text version of the game message
func formatGameMessageText(gameName, releaseDate, rating, genres, platforms, summary, downloadLink string) string {
	return `🎮 **` + gameName + `**
📅 Release Date: ` + releaseDate + `
⭐ Rating: ` + rating + `/100
🎯 Genres: ` + genres + `
🖥️ Platforms: ` + platforms + `
📝 Summary: ` + summary + `
🔗 Download: ` + downloadLink
}

// formatGameMessageHTML creates an HTML version of the game message
func formatGameMessageHTML(gameName, releaseDate, rating, genres, platforms, summary, downloadLink string) string {
	return `<h3>🎮 <strong>` + gameName + `</strong></h3>
<p><strong>📅 Release Date:</strong> ` + releaseDate + `</p>
<p><strong>⭐ Rating:</strong> ` + rating + `/100</p>
<p><strong>🎯 Genres:</strong> ` + genres + `</p>
<p><strong>🖥️ Platforms:</strong> ` + platforms + `</p>
<p><strong>📝 Summary:</strong> ` + summary + `</p>
<p><strong>🔗 Download:</strong> <a href="` + downloadLink + `">` + downloadLink + `</a></p>`
}
