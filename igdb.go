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
	// Add context with timeout for API calls
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Search for the game with a higher limit to get multiple results
	games, err := ic.client.Games.Search(gameName, igdb.SetFields("name,first_release_date,summary,storyline,slug,cover,screenshots,rating,genres,platforms"), igdb.SetLimit(10))
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

	// Get genres if available
	var genreNames []string
	if len(bestGame.Genres) > 0 {
		genres, err := ic.getGenres(ctx, bestGame.Genres)
		if err == nil {
			genreNames = genres
		}
	}

	// Get platforms if available
	var platformNames []string
	if len(bestGame.Platforms) > 0 {
		platforms, err := ic.getPlatforms(ctx, bestGame.Platforms)
		if err == nil {
			platformNames = platforms
		}
	}

	// Convert to GameInfo for compatibility
	gameInfo := &GameInfo{
		Name:        bestGame.Name,
		Summary:     bestGame.Summary,
		Rating:      bestGame.Rating,
		ReleaseDate: formatReleaseDate(int64(bestGame.FirstReleaseDate)),
		Genres:      genreNames,
		Platforms:   platformNames,
	}

	return gameInfo, nil
}

// findBestMatch implements a scoring system to find the best matching game
func findBestMatch(searchQuery string, games []*igdb.Game) *igdb.Game {
	if len(games) == 0 {
		return nil
	}

	var bestGame *igdb.Game
	var bestScore float64

	searchLower := strings.ToLower(strings.TrimSpace(searchQuery))

	for _, game := range games {
		score := calculateMatchScore(searchLower, game)

		if bestGame == nil || score > bestScore {
			bestGame = game
			bestScore = score
		}
	}

	log.Printf("Best match for '%s': '%s' (score: %.2f)", searchQuery, bestGame.Name, bestScore)
	return bestGame
}

// calculateMatchScore returns a score between 0 and 1, where 1 is a perfect match
func calculateMatchScore(searchQuery string, game *igdb.Game) float64 {
	gameName := strings.ToLower(strings.TrimSpace(game.Name))

	// Perfect exact match
	if gameName == searchQuery {
		return 1.0
	}

	// Exact word match (e.g., "subnautica" matches "Subnautica")
	if gameName == searchQuery {
		return 0.95
	}

	// Check if search query is contained in game name
	if strings.Contains(gameName, searchQuery) {
		// Bonus for being at the start of the name
		if strings.HasPrefix(gameName, searchQuery) {
			return 0.9
		}
		return 0.8
	}

	// Check if game name is contained in search query
	if strings.Contains(searchQuery, gameName) {
		return 0.7
	}

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
			return wordScore * 0.6 // Cap at 0.6 for partial word matches
		}
	}

	// Penalize game packs, collections, and similar titles
	penaltyWords := []string{
		"pack", "collection", "bundle", "double", "triple", "quadruple",
		"complete", "ultimate", "deluxe", "edition", "remastered",
		"remaster", "definitive", "anniversary", "gold", "platinum",
		"+", "plus", "and", "&", "with", "featuring", "including",
	}

	for _, penaltyWord := range penaltyWords {
		if strings.Contains(gameName, penaltyWord) {
			return 0.1 // Heavy penalty for pack/collection titles
		}
	}

	// Very low score for no match
	return 0.0
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
