package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Henry-Sarabia/igdb/v2"
)

// IGDBGameInfo holds the info we want from IGDB
type IGDBGameInfo struct {
	Title       string
	Date        int64
	Summary     string
	Storyline   string
	IGDBURL     string
	CoverURL    string
	Screenshots []string
}

// GameInfo represents game information from IGDB (for compatibility)
type GameInfo struct {
	Name        string
	Summary     string
	ReleaseDate string
	Rating      float64
	Genres      []string
	Platforms   []string
}

// IGDBAuthTransport handles OAuth2 authentication for IGDB
type IGDBAuthTransport struct {
	Token     string
	ClientID  string
	Transport http.RoundTripper
}

func (t *IGDBAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.Token)
	req.Header.Set("Client-ID", t.ClientID)
	return t.Transport.RoundTrip(req)
}

// IGDBClient handles IGDB API operations
type IGDBClient struct {
	client *igdb.Client
}

// NewIGDBClient creates a new IGDB client
func NewIGDBClient(clientID, clientSecret string) (*IGDBClient, error) {
	token, err := getIGDBAccessToken(clientID, clientSecret)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Transport: &IGDBAuthTransport{
			Token:     token,
			ClientID:  clientID,
			Transport: http.DefaultTransport,
		},
	}

	client := igdb.NewClient(clientID, "", httpClient)

	return &IGDBClient{
		client: client,
	}, nil
}

// getIGDBAccessToken retrieves an access token from Twitch OAuth2
func getIGDBAccessToken(clientID, clientSecret string) (string, error) {
	url := "https://id.twitch.tv/oauth2/token"
	data := fmt.Sprintf("client_id=%s&client_secret=%s&grant_type=client_credentials", clientID, clientSecret)
	resp, err := http.Post(url, "application/x-www-form-urlencoded", strings.NewReader(data))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.AccessToken, nil
}

// SearchGame searches for a game by name and returns game information
func (ic *IGDBClient) SearchGame(gameName string) (*GameInfo, error) {
	igdbInfo, err := ic.SearchGameWithImages(gameName)
	if err != nil {
		return nil, err
	}

	// Convert to GameInfo for compatibility
	gameInfo := &GameInfo{
		Name:        igdbInfo.Title,
		Summary:     igdbInfo.Summary,
		Rating:      0, // We'll need to get this from the original game data
		ReleaseDate: formatReleaseDate(igdbInfo.Date),
		Genres:      []string{}, // We'll need to get this from the original game data
		Platforms:   []string{}, // We'll need to get this from the original game data
	}

	return gameInfo, nil
}

// SearchGameWithImages searches for a game by name and returns full IGDB information including images
func (ic *IGDBClient) SearchGameWithImages(gameName string) (*IGDBGameInfo, error) {
	// Add context with timeout for API calls
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Search for the game with a higher limit to get multiple results
	// IGDB search returns results in relevance order by default
	games, err := ic.client.Games.Search(gameName,
		igdb.SetFields("name,first_release_date,summary,storyline,slug,cover,screenshots,rating,genres,platforms,category,status"),
		igdb.SetLimit(20), // Get more results to have better selection
		igdb.SetFilter("first_release_date", igdb.OpGreaterThan, fmt.Sprintf("%d", time.Now().AddDate(-20, 0, 0).Unix())), // Only games from last 20 years
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search IGDB for game '%s': %w", gameName, err)
	}
	if len(games) == 0 {
		return nil, fmt.Errorf("no games found for '%s'", gameName)
	}

	// Find the best matching game using our scoring system
	bestGame := findBestMatch(gameName, games)
	if bestGame == nil {
		return nil, fmt.Errorf("no suitable match found for '%s' among %d results", gameName, len(games))
	}

	info := &IGDBGameInfo{
		Title:     bestGame.Name,
		Date:      int64(bestGame.FirstReleaseDate),
		Summary:   bestGame.Summary,
		Storyline: bestGame.Storyline,
		IGDBURL:   fmt.Sprintf("https://www.igdb.com/games/%s", bestGame.Slug),
	}

	// Fetch cover if present
	if bestGame.Cover != 0 {
		if err := ic.fetchCover(ctx, bestGame.Cover, info); err != nil {
			log.Printf("Failed to fetch cover for '%s': %v", bestGame.Name, err)
		}
	}

	// Fetch screenshots in parallel if present
	if len(bestGame.Screenshots) > 0 {
		if err := ic.fetchScreenshots(ctx, bestGame.Screenshots, info, bestGame.Name); err != nil {
			log.Printf("Failed to fetch some screenshots for '%s': %v", bestGame.Name, err)
		}
	}

	return info, nil
}

// findBestMatch implements a scoring system to find the best matching game
func findBestMatch(searchQuery string, games []*igdb.Game) *igdb.Game {
	if len(games) == 0 {
		return nil
	}

	var bestGame *igdb.Game
	var bestScore float64

	searchLower := strings.ToLower(strings.TrimSpace(searchQuery))

	log.Printf("=== FINDING BEST MATCH FOR '%s' ===", searchQuery)
	log.Printf("Found %d games to evaluate:", len(games))

	for i, game := range games {
		score := calculateMatchScore(searchLower, game)
		recencyBonus := calculateRecencyBonus(game.FirstReleaseDate)
		releaseDate := "Unknown"
		if game.FirstReleaseDate != 0 {
			releaseDate = time.Unix(int64(game.FirstReleaseDate), 0).Format("2006-01-02")
		}

		// Get game category info
		category := "Unknown"
		switch game.Category {
		case 0:
			category = "Main Game"
		case 1:
			category = "DLC/Add-on"
		case 2:
			category = "Expansion"
		case 3:
			category = "Bundle"
		case 4:
			category = "Standalone Expansion"
		case 5:
			category = "Mod"
		case 6:
			category = "Episode"
		case 7:
			category = "Season"
		case 8:
			category = "Remake"
		case 9:
			category = "Remaster"
		case 10:
			category = "Expanded Game"
		case 11:
			category = "Port"
		case 12:
			category = "Fork"
		case 13:
			category = "Pack"
		case 14:
			category = "Update"
		}

		// Get game status
		status := "Unknown"
		switch game.Status {
		case 0:
			status = "Released"
		case 2:
			status = "Alpha"
		case 3:
			status = "Beta"
		case 4:
			status = "Early Access"
		case 5:
			status = "Offline"
		case 6:
			status = "Cancelled"
		case 7:
			status = "Rumored"
		case 8:
			status = "Delisted"
		}

		log.Printf("  %d. '%s'", i+1, game.Name)
		log.Printf("      Score: %.3f (base: %.3f + recency: %.3f)", score, score-recencyBonus, recencyBonus)
		log.Printf("      Released: %s | Category: %s | Status: %s", releaseDate, category, status)
		log.Printf("      ID: %d | Rating: %.1f | Summary: %.100s...", game.ID, game.Rating, game.Summary)

		if bestGame == nil || score > bestScore {
			bestGame = game
			bestScore = score
		}
	}

	// Calculate recency bonus for logging
	recencyBonus := calculateRecencyBonus(bestGame.FirstReleaseDate)
	log.Printf("=== SELECTED: '%s' (final score: %.3f, recency bonus: %.3f) ===", bestGame.Name, bestScore, recencyBonus)
	return bestGame
}

// calculateMatchScore returns a score between 0 and 1, where 1 is a perfect match
func calculateMatchScore(searchQuery string, game *igdb.Game) float64 {
	gameName := strings.ToLower(strings.TrimSpace(game.Name))
	baseScore := 0.0

	// Perfect exact match
	if gameName == searchQuery {
		baseScore = 1.0
	} else if gameName == searchQuery {
		// Exact word match (e.g., "subnautica" matches "Subnautica")
		baseScore = 0.95
	} else if strings.Contains(gameName, searchQuery) {
		// Check if search query is contained in game name
		if strings.HasPrefix(gameName, searchQuery) {
			baseScore = 0.9
		} else {
			baseScore = 0.8
		}
	} else if strings.Contains(searchQuery, gameName) {
		// Check if game name is contained in search query
		baseScore = 0.7
	} else {
		// Check for word-by-word matching
		searchWords := strings.Fields(searchQuery)
		gameWords := strings.Fields(gameName)

		wordMatches := 0
		for _, searchWord := range searchWords {
			for _, gameWord := range gameWords {
				if searchWord == gameWord {
					wordMatches++
					break
				}
			}
		}

		if len(searchWords) > 0 {
			wordScore := float64(wordMatches) / float64(len(searchWords))
			if wordScore > 0.5 {
				baseScore = wordScore * 0.6 // Cap at 0.6 for partial word matches
			}
		}
	}

	// If we have a base score, apply recency bonus and penalties
	if baseScore > 0 {
		// Apply recency bonus (0.0 to 0.2 bonus for recent games)
		recencyBonus := calculateRecencyBonus(game.FirstReleaseDate)
		baseScore += recencyBonus

		// Bonus for main games (not DLC, updates, etc.)
		if game.Category == 0 { // Main Game
			baseScore += 0.1
		}

		// Penalty for very old games (pre-2010)
		if game.FirstReleaseDate != 0 {
			releaseYear := time.Unix(int64(game.FirstReleaseDate), 0).Year()
			if releaseYear < 2010 {
				baseScore *= 0.5 // Heavy penalty for very old games
			}
		}

		// Apply penalties for game packs, collections, and similar titles
		penaltyWords := []string{
			"pack", "collection", "bundle", "double", "triple", "quadruple",
			"complete", "ultimate", "deluxe", "edition", "remastered",
			"remaster", "definitive", "anniversary", "gold", "platinum",
			"+", "plus", "and", "&", "with", "featuring", "including",
		}

		for _, penaltyWord := range penaltyWords {
			if strings.Contains(gameName, penaltyWord) {
				baseScore *= 0.3 // Heavy penalty for pack/collection titles
				break
			}
		}

		// Cap the final score at 1.0
		if baseScore > 1.0 {
			baseScore = 1.0
		}
	}

	return baseScore
}

// calculateRecencyBonus returns a bonus score (0.0 to 0.2) based on how recent the game is
func calculateRecencyBonus(releaseDate int) float64 {
	if releaseDate == 0 {
		return 0.0 // No release date, no bonus
	}

	// Convert Unix timestamp to time
	releaseTime := time.Unix(int64(releaseDate), 0)
	now := time.Now()

	// Calculate years difference (positive for past, negative for future)
	yearsDifference := releaseTime.Sub(now).Hours() / (24 * 365.25)

	// Handle future release dates (upcoming games)
	if yearsDifference > 0 {
		// Future games get maximum bonus if releasing within 1 year
		if yearsDifference <= 1 {
			return 0.2 // Maximum bonus for games releasing soon
		} else if yearsDifference <= 2 {
			// Decreasing bonus for games releasing in 1-2 years
			return 0.2 - (yearsDifference-1)*0.1
		} else {
			// Very distant future games get minimal bonus
			return 0.05
		}
	}

	// Handle past release dates (released games)
	yearsSinceRelease := -yearsDifference // Convert to positive number

	// Give maximum bonus (0.2) for games released in the last 2 years
	// Gradually decrease bonus for older games
	if yearsSinceRelease <= 2 {
		return 0.2
	} else if yearsSinceRelease <= 5 {
		// Linear decrease from 0.2 to 0.1 over 3 years
		return 0.2 - (yearsSinceRelease-2)*0.033
	} else if yearsSinceRelease <= 10 {
		// Linear decrease from 0.1 to 0.05 over 5 years
		return 0.1 - (yearsSinceRelease-5)*0.01
	} else {
		// Very old games get minimal bonus
		return 0.05
	}
}

// getGenres retrieves genre information for given genre IDs
func (ic *IGDBClient) getGenres(ctx context.Context, genreIDs []int) ([]string, error) {
	genres, err := ic.client.Genres.List(genreIDs, igdb.SetFields("name"))
	if err != nil {
		return nil, err
	}

	var genreNames []string
	for _, genre := range genres {
		genreNames = append(genreNames, genre.Name)
	}

	return genreNames, nil
}

// getPlatforms retrieves platform information for given platform IDs
func (ic *IGDBClient) getPlatforms(ctx context.Context, platformIDs []int) ([]string, error) {
	platforms, err := ic.client.Platforms.List(platformIDs, igdb.SetFields("name"))
	if err != nil {
		return nil, err
	}

	var platformNames []string
	for _, platform := range platforms {
		platformNames = append(platformNames, platform.Name)
	}

	return platformNames, nil
}

// formatReleaseDate formats a Unix timestamp to a readable date
func formatReleaseDate(timestamp int64) string {
	if timestamp == 0 {
		return "Unknown"
	}
	return time.Unix(timestamp, 0).Format("2006-01-02")
}

// formatGenres formats a slice of genre names into a comma-separated string
func formatGenres(genres []string) string {
	if len(genres) == 0 {
		return "Unknown"
	}
	return strings.Join(genres, ", ")
}

// formatPlatforms formats a slice of platform names into a comma-separated string
func formatPlatforms(platforms []string) string {
	if len(platforms) == 0 {
		return "Unknown"
	}
	return strings.Join(platforms, ", ")
}

// formatSummary truncates a summary to a reasonable length
func formatSummary(summary string, maxLen int) string {
	if len(summary) <= maxLen {
		return summary
	}
	return summary[:maxLen-3] + "..."
}

// fetchCover fetches the cover image for a game
func (ic *IGDBClient) fetchCover(ctx context.Context, coverID int, info *IGDBGameInfo) error {
	cover, err := ic.client.Covers.Get(coverID, igdb.SetFields("url,image_id,width,height"))
	if err != nil {
		return fmt.Errorf("failed to get cover: %w", err)
	}
	if cover == nil || cover.Image.ImageID == "" {
		return fmt.Errorf("no valid cover image found")
	}

	info.CoverURL = fmt.Sprintf("https://images.igdb.com/igdb/image/upload/t_original/%s.webp", cover.Image.ImageID)
	return nil
}

// fetchScreenshots fetches screenshots in parallel
func (ic *IGDBClient) fetchScreenshots(ctx context.Context, screenshotIDs []int, info *IGDBGameInfo, gameName string) error {
	// Create a channel to collect results
	type screenshotResult struct {
		url string
		err error
	}
	resultChan := make(chan screenshotResult, len(screenshotIDs))

	// Launch goroutines for each screenshot
	for _, id := range screenshotIDs {
		go func(screenshotID int) {
			sc, err := ic.client.Screenshots.Get(screenshotID, igdb.SetFields("url,image_id,width,height"))
			if err != nil {
				resultChan <- screenshotResult{err: fmt.Errorf("failed to get screenshot %d: %w", screenshotID, err)}
				return
			}
			if sc == nil || sc.Image.ImageID == "" {
				resultChan <- screenshotResult{err: fmt.Errorf("no valid screenshot image for ID %d", screenshotID)}
				return
			}

			url := fmt.Sprintf("https://images.igdb.com/igdb/image/upload/t_original/%s.webp", sc.Image.ImageID)
			resultChan <- screenshotResult{url: url}
		}(id)
	}

	// Collect results
	for i := 0; i < len(screenshotIDs); i++ {
		select {
		case result := <-resultChan:
			if result.err != nil {
				log.Printf("Screenshot fetch error for '%s': %v", gameName, result.err)
			} else {
				info.Screenshots = append(info.Screenshots, result.url)
			}
		case <-ctx.Done():
			return fmt.Errorf("timeout while fetching screenshots: %w", ctx.Err())
		}
	}

	return nil
}
