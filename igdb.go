package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// IGDBClient handles IGDB API operations
type IGDBClient struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	accessToken  string
}

// GameInfo represents game information from IGDB
type GameInfo struct {
	Name        string
	Summary     string
	ReleaseDate string
	Rating      float64
	Genres      []string
	Platforms   []string
}

// IGDBGame represents a game from IGDB API
type IGDBGame struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	Summary          string  `json:"summary"`
	FirstReleaseDate int64   `json:"first_release_date"`
	Rating           float64 `json:"rating"`
	Genres           []int   `json:"genres"`
	Platforms        []int   `json:"platforms"`
}

// IGDBGenre represents a genre from IGDB API
type IGDBGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// IGDBPlatform represents a platform from IGDB API
type IGDBPlatform struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// IGDBTokenResponse represents the OAuth token response from IGDB
type IGDBTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// NewIGDBClient creates a new IGDB client
func NewIGDBClient(clientID, clientSecret string) (*IGDBClient, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	igdbClient := &IGDBClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   client,
	}

	// Get access token
	token, err := igdbClient.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %v", err)
	}

	igdbClient.accessToken = token
	return igdbClient, nil
}

// getAccessToken retrieves an access token from IGDB
func (ic *IGDBClient) getAccessToken() (string, error) {
	url := "https://id.twitch.tv/oauth2/token"
	data := fmt.Sprintf("client_id=%s&client_secret=%s&grant_type=client_credentials", ic.clientID, ic.clientSecret)

	resp, err := ic.httpClient.Post(url, "application/x-www-form-urlencoded", strings.NewReader(data))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResp IGDBTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	return tokenResp.AccessToken, nil
}

// SearchGame searches for a game by name and returns game information
func (ic *IGDBClient) SearchGame(gameName string) (*GameInfo, error) {
	// Search for games
	games, err := ic.searchGames(gameName)
	if err != nil {
		return nil, err
	}

	if len(games) == 0 {
		return nil, fmt.Errorf("no games found for: %s", gameName)
	}

	game := games[0]

	// Get additional details
	gameInfo := &GameInfo{
		Name:        game.Name,
		Summary:     game.Summary,
		Rating:      game.Rating,
		ReleaseDate: formatReleaseDate(game.FirstReleaseDate),
		Genres:      []string{},
		Platforms:   []string{},
	}

	// Get genres if available
	if len(game.Genres) > 0 {
		genres, err := ic.getGenres(game.Genres)
		if err == nil {
			gameInfo.Genres = genres
		}
	}

	// Get platforms if available
	if len(game.Platforms) > 0 {
		platforms, err := ic.getPlatforms(game.Platforms)
		if err == nil {
			gameInfo.Platforms = platforms
		}
	}

	return gameInfo, nil
}

// searchGames searches for games using IGDB API
func (ic *IGDBClient) searchGames(gameName string) ([]IGDBGame, error) {
	url := "https://api.igdb.com/v4/games"
	query := fmt.Sprintf(`search "%s"; fields id,name,summary,first_release_date,rating,genres,platforms; limit 1;`, gameName)

	req, err := http.NewRequest("POST", url, strings.NewReader(query))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Client-ID", ic.clientID)
	req.Header.Set("Authorization", "Bearer "+ic.accessToken)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := ic.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IGDB API returned status %d", resp.StatusCode)
	}

	var games []IGDBGame
	if err := json.NewDecoder(resp.Body).Decode(&games); err != nil {
		return nil, err
	}

	return games, nil
}

// getGenres retrieves genre information for given genre IDs
func (ic *IGDBClient) getGenres(genreIDs []int) ([]string, error) {
	url := "https://api.igdb.com/v4/genres"

	// Convert int slice to string slice for the query
	var idStrings []string
	for _, id := range genreIDs {
		idStrings = append(idStrings, fmt.Sprintf("%d", id))
	}

	query := fmt.Sprintf(`fields name; where id = (%s);`, strings.Join(idStrings, ","))

	req, err := http.NewRequest("POST", url, strings.NewReader(query))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Client-ID", ic.clientID)
	req.Header.Set("Authorization", "Bearer "+ic.accessToken)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := ic.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IGDB API returned status %d", resp.StatusCode)
	}

	var genres []IGDBGenre
	if err := json.NewDecoder(resp.Body).Decode(&genres); err != nil {
		return nil, err
	}

	var genreNames []string
	for _, genre := range genres {
		genreNames = append(genreNames, genre.Name)
	}

	return genreNames, nil
}

// getPlatforms retrieves platform information for given platform IDs
func (ic *IGDBClient) getPlatforms(platformIDs []int) ([]string, error) {
	url := "https://api.igdb.com/v4/platforms"

	// Convert int slice to string slice for the query
	var idStrings []string
	for _, id := range platformIDs {
		idStrings = append(idStrings, fmt.Sprintf("%d", id))
	}

	query := fmt.Sprintf(`fields name; where id = (%s);`, strings.Join(idStrings, ","))

	req, err := http.NewRequest("POST", url, strings.NewReader(query))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Client-ID", ic.clientID)
	req.Header.Set("Authorization", "Bearer "+ic.accessToken)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := ic.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IGDB API returned status %d", resp.StatusCode)
	}

	var platforms []IGDBPlatform
	if err := json.NewDecoder(resp.Body).Decode(&platforms); err != nil {
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
