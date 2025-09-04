package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/mmcdole/gofeed"
)

// Config holds configuration for the application
type Config struct {
	RSSURL            string
	MatrixHomeserver  string
	MatrixUserID      string
	MatrixAccessToken string
	MatrixRoomID      string
	IGDBClientID      string
	IGDBClientSecret  string
}

// RSSProcessor handles RSS feed processing
type RSSProcessor struct {
	config       *Config
	client       *http.Client
	matrixClient *MatrixClient
	igdbClient   *IGDBClient
}

// NewRSSProcessor creates a new RSS processor
func NewRSSProcessor(config *Config) (*RSSProcessor, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Initialize Matrix client
	matrixClient, err := NewMatrixClient(
		config.MatrixHomeserver,
		config.MatrixUserID,
		config.MatrixAccessToken,
		config.MatrixRoomID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Matrix client: %v", err)
	}

	// Initialize IGDB client
	igdbClient, err := NewIGDBClient(config.IGDBClientID, config.IGDBClientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create IGDB client: %v", err)
	}

	return &RSSProcessor{
		config:       config,
		client:       client,
		matrixClient: matrixClient,
		igdbClient:   igdbClient,
	}, nil
}

// extractGameName extracts game name from RSS item title
func (rp *RSSProcessor) extractGameName(title string) string {
	// Common patterns for game names in torrent titles
	patterns := []string{
		`^(.+?)\s*\[.*?\]`,    // Game Name [Release Info]
		`^(.+?)\s*\(.*?\)`,    // Game Name (Release Info)
		`^(.+?)\s*-\s*.*`,     // Game Name - Release Info
		`^(.+?)\s*v?\d+\.\d+`, // Game Name v1.0
		`^(.+?)\s*PC.*`,       // Game Name PC
		`^(.+?)\s*REPACK.*`,   // Game Name REPACK
		`^(.+?)\s*CRACK.*`,    // Game Name CRACK
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(title)
		if len(matches) > 1 {
			gameName := strings.TrimSpace(matches[1])
			// Clean up common prefixes/suffixes
			gameName = strings.TrimPrefix(gameName, "[")
			gameName = strings.TrimSuffix(gameName, "]")
			gameName = strings.TrimSpace(gameName)
			return gameName
		}
	}

	// If no pattern matches, return the original title
	return strings.TrimSpace(title)
}

// processRSSFeed processes the RSS feed and sends notifications
func (rp *RSSProcessor) processRSSFeed() error {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(rp.config.RSSURL)
	if err != nil {
		return fmt.Errorf("failed to parse RSS feed: %v", err)
	}

	log.Printf("Processing %d items from RSS feed", len(feed.Items))

	for _, item := range feed.Items {
		gameName := rp.extractGameName(item.Title)
		log.Printf("Extracted game name: %s", gameName)

		// Search IGDB for game information
		gameInfo, err := rp.igdbClient.SearchGame(gameName)
		if err != nil {
			log.Printf("Failed to get IGDB info for %s: %v", gameName, err)
			// Send basic notification even without IGDB info
			message := fmt.Sprintf("🎮 New Game: %s\n🔗 %s", gameName, item.Link)
			if err := rp.matrixClient.SendMessage(message); err != nil {
				log.Printf("Failed to send Matrix message: %v", err)
			}
			continue
		}

		// Send detailed notification with game info
		err = rp.matrixClient.SendGameNotification(
			gameInfo.Name,
			gameInfo.ReleaseDate,
			fmt.Sprintf("%.1f", gameInfo.Rating),
			formatGenres(gameInfo.Genres),
			formatPlatforms(gameInfo.Platforms),
			formatSummary(gameInfo.Summary, 200),
			item.Link,
		)
		if err != nil {
			log.Printf("Failed to send Matrix message: %v", err)
		} else {
			log.Printf("Sent Matrix message for: %s", gameInfo.Name)
		}

		// Add delay to avoid rate limiting
		time.Sleep(2 * time.Second)
	}

	return nil
}

// loadConfig loads configuration from environment variables
func loadConfig() (*Config, error) {
	// Load .env file if it exists
	godotenv.Load()

	config := &Config{
		RSSURL:            getEnv("RSS_URL", ""),
		MatrixHomeserver:  getEnv("MATRIX_HOMESERVER", ""),
		MatrixUserID:      getEnv("MATRIX_USER_ID", ""),
		MatrixAccessToken: getEnv("MATRIX_ACCESS_TOKEN", ""),
		MatrixRoomID:      getEnv("MATRIX_ROOM_ID", ""),
		IGDBClientID:      getEnv("IGDB_CLIENT_ID", ""),
		IGDBClientSecret:  getEnv("IGDB_CLIENT_SECRET", ""),
	}

	// Validate required configuration
	if config.RSSURL == "" {
		return nil, fmt.Errorf("RSS_URL is required")
	}
	if config.MatrixHomeserver == "" {
		return nil, fmt.Errorf("MATRIX_HOMESERVER is required")
	}
	if config.MatrixUserID == "" {
		return nil, fmt.Errorf("MATRIX_USER_ID is required")
	}
	if config.MatrixAccessToken == "" {
		return nil, fmt.Errorf("MATRIX_ACCESS_TOKEN is required")
	}
	if config.MatrixRoomID == "" {
		return nil, fmt.Errorf("MATRIX_ROOM_ID is required")
	}
	if config.IGDBClientID == "" {
		return nil, fmt.Errorf("IGDB_CLIENT_ID is required")
	}
	if config.IGDBClientSecret == "" {
		return nil, fmt.Errorf("IGDB_CLIENT_SECRET is required")
	}

	return config, nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	log.Println("Starting Zamunda RSS Jackett processor...")

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create RSS processor
	processor, err := NewRSSProcessor(config)
	if err != nil {
		log.Fatalf("Failed to create RSS processor: %v", err)
	}

	// Process RSS feed
	if err := processor.processRSSFeed(); err != nil {
		log.Fatalf("Failed to process RSS feed: %v", err)
	}

	log.Println("RSS processing completed successfully!")
}
