# Zamunda RSS Jackett

A Go application that processes RSS feeds from torrent sites, extracts game names, fetches detailed information from IGDB (Internet Game Database), and sends formatted notifications to Matrix rooms.

## Features

- 🔍 **RSS Feed Processing**: Parses RSS feeds and extracts game names from torrent titles
- 🎮 **IGDB Integration**: Fetches detailed game information including ratings, genres, platforms, and release dates
- 💬 **Matrix Notifications**: Sends beautifully formatted messages to Matrix rooms
- 🛡️ **Error Handling**: Robust error handling with fallback notifications
- ⚙️ **Configurable**: Easy configuration via environment variables

## Prerequisites

- Go 1.21 or later
- IGDB API credentials (free at [api.igdb.com](https://api.igdb.com/))
- Matrix account and access token
- RSS feed URL (e.g., from Jackett)

## Installation

1. Clone the repository:
```bash
git clone <your-repo-url>
cd zamunda-rss-jackett
```

2. Install dependencies:
```bash
go mod tidy
```

3. Copy the example configuration:
```bash
cp config.example.env .env
```

4. Edit `.env` with your configuration (see Configuration section below)

## Configuration

Create a `.env` file with the following variables:

### RSS Configuration
- `RSS_URL`: The URL of your RSS feed (e.g., from Jackett)

### Matrix Configuration
- `MATRIX_HOMESERVER`: Your Matrix homeserver URL (e.g., `https://matrix.example.com`)
- `MATRIX_USER_ID`: Your Matrix user ID (e.g., `@your-bot:example.com`)
- `MATRIX_ACCESS_TOKEN`: Your Matrix access token
- `MATRIX_ROOM_ID`: The room ID where messages should be sent (e.g., `!room-id:example.com`)

### IGDB Configuration
- `IGDB_CLIENT_ID`: Your IGDB API client ID
- `IGDB_CLIENT_SECRET`: Your IGDB API client secret

### Getting Matrix Access Token

1. Log into your Matrix client (Element, etc.)
2. Go to Settings → Help & About → Advanced
3. Click "Access Token" to reveal your token

### Getting IGDB API Credentials

1. Visit [api.igdb.com](https://api.igdb.com/)
2. Sign up for a free account
3. Create a new application
4. Note down your Client ID and Client Secret

## Usage

Run the application:
```bash
go run main.go
```

The application will:
1. Fetch the RSS feed
2. Extract game names from each item
3. Query IGDB for detailed game information
4. Send formatted messages to your Matrix room

## Example Output

The bot will send messages like this to your Matrix room:

```
🎮 **Cyberpunk 2077**
📅 Release Date: 2020-12-10
⭐ Rating: 76.0/100
🎯 Genres: Action, Role-playing (RPG)
🖥️ Platforms: PC, PlayStation 4, Xbox One
📝 Summary: Cyberpunk 2077 is an open-world, action-adventure story set in Night City, a megalopolis obsessed with power, glamour and ceaseless body modification...
🔗 Download: https://your-torrent-link.com
```

## Game Name Extraction

The application uses intelligent pattern matching to extract game names from torrent titles. It handles common patterns like:

- `Game Name [Release Info]`
- `Game Name (Release Info)`
- `Game Name - Release Info`
- `Game Name v1.0`
- `Game Name PC`
- `Game Name REPACK`

## Error Handling

- If IGDB lookup fails, a basic notification is still sent
- Rate limiting is implemented to avoid API limits
- Comprehensive logging for debugging

## Building

To build the application:

```bash
go build -o zamunda-rss-jackett main.go
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
