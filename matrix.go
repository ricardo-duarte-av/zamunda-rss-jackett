package main

import (
	"fmt"
	"log"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	mautrixEvent "maunium.net/go/mautrix/event"
	mautrixID "maunium.net/go/mautrix/id"
)

// MatrixClient handles Matrix operations
type MatrixClient struct {
	client *mautrix.Client
	roomID mautrixID.RoomID
}

// NewMatrixClient creates a new Matrix client
func NewMatrixClient(cfg *Config, configPath string) (*MatrixClient, error) {
	var client *mautrix.Client
	var err error

	// First, try to use existing access token if available
	if cfg.MatrixAccessToken != "" {
		client, err = mautrix.NewClient(cfg.MatrixHomeserver, mautrixID.UserID(cfg.MatrixUserID), cfg.MatrixAccessToken)
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
				roomID: mautrixID.RoomID(cfg.MatrixRoomID),
			}, nil
		}
	}

	// If we reach here, either no token was provided or the existing token is invalid
	// Try to get a new token using username/password
	if cfg.MatrixUser != "" && cfg.MatrixPassword != "" {
		client, err = mautrix.NewClient(cfg.MatrixHomeserver, mautrixID.UserID(cfg.MatrixUserID), "")
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
		roomID: mautrixID.RoomID(cfg.MatrixRoomID),
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

// SendGameNotificationWithImages sends a game notification with cover image and screenshots in a thread
func (mc *MatrixClient) SendGameNotificationWithImages(gameInfo *IGDBGameInfo) error {
	// Create plain text version
	textMessage := formatGameMessageText(gameInfo.Title, formatReleaseDate(gameInfo.Date), "0", "Unknown", "Unknown", gameInfo.Summary, "")

	// Create HTML version
	htmlMessage := formatGameMessageHTML(gameInfo.Title, formatReleaseDate(gameInfo.Date), "0", "Unknown", "Unknown", gameInfo.Summary, "")

	var threadRootID mautrixID.EventID
	var replyID mautrixID.EventID

	// Send cover image as the main message if available
	if gameInfo.CoverURL != "" {
		eventID, err := mc.postIGDBImageToMatrix(gameInfo.CoverURL, textMessage, htmlMessage, "", "")
		if err != nil {
			log.Printf("Failed to send cover image: %v", err)
			// Fallback to text message
			return mc.SendFormattedMessage(textMessage, htmlMessage)
		}
		threadRootID = eventID
		replyID = eventID
	} else {
		// No cover image, send text message
		err := mc.SendFormattedMessage(textMessage, htmlMessage)
		if err != nil {
			return err
		}
		// We'll need to get the event ID from the text message to create a thread
		// For now, we'll skip screenshots if no cover image
		return nil
	}

	// Send screenshots in the thread
	if len(gameInfo.Screenshots) > 0 {
		for i, screenshotURL := range gameInfo.Screenshots {
			// Limit to first 5 screenshots to avoid spam
			if i >= 5 {
				break
			}

			caption := fmt.Sprintf("Screenshot %d of %s", i+1, gameInfo.Title)
			_, err := mc.postIGDBImageToMatrix(screenshotURL, caption, "", threadRootID, replyID)
			if err != nil {
				log.Printf("Failed to send screenshot %d: %v", i+1, err)
			}

			// Small delay between screenshots
			time.Sleep(500 * time.Millisecond)
		}
	}

	return nil
}

// formatGameMessageText creates a plain text version of the game message
func formatGameMessageText(gameName, releaseDate, rating, genres, platforms, summary, downloadLink string) string {
	return `🎮 **` + gameName + `**
📅 Release Date: ` + releaseDate + `
⭐ Rating: ` + rating + `/100
🎯 Genres: ` + genres + `
🖥️ Platforms: ` + platforms + `
📝 Summary: ` + summary
}

// formatGameMessageHTML creates an HTML version of the game message
func formatGameMessageHTML(gameName, releaseDate, rating, genres, platforms, summary, downloadLink string) string {
	return `<h3>🎮 <strong>` + gameName + `</strong></h3>
<p><strong>📅 Release Date:</strong> ` + releaseDate + `</p>
<p><strong>⭐ Rating:</strong> ` + rating + `/100</p>
<p><strong>🎯 Genres:</strong> ` + genres + `</p>
<p><strong>🖥️ Platforms:</strong> ` + platforms + `</p>
<p><strong>📝 Summary:</strong> ` + summary + `</p>`
}

// sendMatrixImage sends an m.image event to the Matrix room
func (mc *MatrixClient) sendMatrixImage(caption, filename string, imgURL, thumbURL string, imgInfo, thumbInfo *MatrixImageInfo, blurhash string, threadRootID mautrixID.EventID, replyID mautrixID.EventID) (mautrixID.EventID, error) {
	imgInfo.ThumbnailURL = thumbURL
	imgInfo.ThumbnailInfo = thumbInfo
	if blurhash != "" {
		if imgInfo.Additional == nil {
			imgInfo.Additional = map[string]interface{}{}
		}
		imgInfo.Additional["xyz.amorgan.blurhash"] = blurhash
	}

	content := map[string]interface{}{
		"msgtype":  "m.image",
		"body":     caption,
		"url":      imgURL,
		"info":     imgInfo,
		"filename": filename,
	}

	// Relationship handling
	if threadRootID != "" {
		// Threaded reply: replyID is required
		if replyID == "" {
			return "", fmt.Errorf("replyID must be set when replying in a thread")
		}
		content["m.relates_to"] = map[string]interface{}{
			"event_id":        threadRootID,
			"rel_type":        "m.thread",
			"is_falling_back": true,
			"m.in_reply_to": map[string]interface{}{
				"event_id": replyID,
			},
		}

	} else if replyID != "" {
		// Normal reply (non-threaded)
		content["m.relates_to"] = map[string]interface{}{
			"m.in_reply_to": map[string]interface{}{
				"event_id": replyID,
			},
		}
	}

	for k, v := range imgInfo.Additional {
		content[k] = v
	}
	evt, err := mc.client.SendMessageEvent(mautrixID.RoomID(mc.roomID), mautrixEvent.EventMessage, content)
	return evt.EventID, err
}

// sendMatrixImageHTML sends an m.image event to the Matrix room with HTML body as well
func (mc *MatrixClient) sendMatrixImageHTML(caption, htmlCaption, filename string, imgURL, thumbURL string, imgInfo, thumbInfo *MatrixImageInfo, blurhash string, threadRootID mautrixID.EventID, replyID mautrixID.EventID) (mautrixID.EventID, error) {
	imgInfo.ThumbnailURL = thumbURL
	imgInfo.ThumbnailInfo = thumbInfo
	if blurhash != "" {
		if imgInfo.Additional == nil {
			imgInfo.Additional = map[string]interface{}{}
		}
		imgInfo.Additional["xyz.amorgan.blurhash"] = blurhash
	}

	content := map[string]interface{}{
		"msgtype":        "m.image",
		"body":           caption,
		"url":            imgURL,
		"info":           imgInfo,
		"filename":       filename,
		"format":         "org.matrix.custom.html",
		"formatted_body": htmlCaption,
	}

	// Relationship handling
	if threadRootID != "" {
		// Threaded reply: replyID is required
		if replyID == "" {
			return "", fmt.Errorf("replyID must be set when replying in a thread")
		}
		content["m.relates_to"] = map[string]interface{}{
			"event_id":        threadRootID,
			"rel_type":        "m.thread",
			"is_falling_back": true,
			"m.in_reply_to": map[string]interface{}{
				"event_id": replyID,
			},
		}

	} else if replyID != "" {
		// Normal reply (non-threaded)
		content["m.relates_to"] = map[string]interface{}{
			"m.in_reply_to": map[string]interface{}{
				"event_id": replyID,
			},
		}
	}

	for k, v := range imgInfo.Additional {
		content[k] = v
	}
	evt, err := mc.client.SendMessageEvent(mautrixID.RoomID(mc.roomID), mautrixEvent.EventMessage, content)
	return evt.EventID, err
}

// postIGDBImageToMatrix downloads, thumbs, blurhashes, uploads, and posts an image to Matrix
func (mc *MatrixClient) postIGDBImageToMatrix(imgURL, caption string, htmlCaption string, threadRootID mautrixID.EventID, replyID mautrixID.EventID) (mautrixID.EventID, error) {
	img, imgBytes, format, err := downloadImage(imgURL)
	if err != nil {
		log.Printf("Failed to download image: %v", err)
		return "", err
	}
	var (
		EventID mautrixID.EventID
	)
	thumb := generateThumbnail(img, 225, 300)
	thumbBytes, _ := encodeImage(thumb, format)
	blur, _ := calcBlurhash(thumb)
	imgMimetype := "image/" + format
	thumbMimetype := imgMimetype
	imgURLMXC, imgInfo, err := uploadToMatrix(mc.client, caption+".webp", imgBytes, imgMimetype, img.Bounds().Dx(), img.Bounds().Dy())
	if err != nil {
		log.Printf("Failed to upload image: %v", err)
		return "", err
	}
	thumbURLMXC, thumbInfo, err := uploadToMatrix(mc.client, caption+"_thumb.webp", thumbBytes, thumbMimetype, thumb.Bounds().Dx(), thumb.Bounds().Dy())
	if err != nil {
		log.Printf("Failed to upload thumbnail: %v", err)
		return "", err
	}
	if htmlCaption == "" {
		EventID, err = mc.sendMatrixImage(caption, caption+".webp", imgURLMXC, thumbURLMXC, imgInfo, thumbInfo, blur, threadRootID, replyID)
		if err != nil {
			log.Printf("Failed to send image event: %v", err)
			return "", err
		}
	} else {
		EventID, err = mc.sendMatrixImageHTML(caption, htmlCaption, caption+".webp", imgURLMXC, thumbURLMXC, imgInfo, thumbInfo, blur, threadRootID, replyID)
		if err != nil {
			log.Printf("Failed to send image event: %v", err)
			return "", err
		}
	}

	return EventID, err
}
