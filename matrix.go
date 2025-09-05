package main

import (
	"fmt"
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
func NewMatrixClient(cfg *Config, configPath string) (*MatrixClient, error) {
	var client *mautrix.Client
	var err error

	// First, try to use existing access token if available
	if cfg.MatrixAccessToken != "" {
		client, err = mautrix.NewClient(cfg.MatrixHomeserver, id.UserID(cfg.MatrixUserID), cfg.MatrixAccessToken)
		if err != nil {
			return nil, err
		}

		// Test if the token is still valid by making a simple API call
		_, err = client.Whoami()
		if err != nil {
			log.Printf("Access token is invalid, attempting to refresh: %v", err)
			// Token is invalid, we'll need to refresh it
			client = nil
		} else {
			// Token is valid, return the client
			return &MatrixClient{
				client: client,
				roomID: id.RoomID(cfg.MatrixRoomID),
			}, nil
		}
	}

	// If we reach here, either no token was provided or the existing token is invalid
	// Try to get a new token using username/password
	if cfg.MatrixUser != "" && cfg.MatrixPassword != "" {
		client, err = mautrix.NewClient(cfg.MatrixHomeserver, id.UserID(cfg.MatrixUserID), "")
		if err != nil {
			return nil, err
		}
		resp, err := client.Login(&mautrix.ReqLogin{
			Type:       "m.login.password",
			Identifier: mautrix.UserIdentifier{User: cfg.MatrixUser},
			Password:   cfg.MatrixPassword,
		})
		if err != nil {
			return nil, err
		}
		cfg.MatrixAccessToken = resp.AccessToken
		saveErr := saveConfig(configPath, cfg)
		if saveErr != nil {
			log.Printf("Warning: failed to save new access token to config: %v", saveErr)
		} else {
			log.Printf("Successfully refreshed Matrix access token and saved to config")
		}
	} else {
		return nil, fmt.Errorf("no Matrix access token or user/pass provided")
	}

	return &MatrixClient{
		client: client,
		roomID: id.RoomID(cfg.MatrixRoomID),
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
