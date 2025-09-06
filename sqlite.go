package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// DB schema: processed_posts(post_id TEXT PRIMARY KEY)
func initDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS processed_posts (post_id TEXT PRIMARY KEY)`)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func isPostProcessed(db *sql.DB, postID string) (bool, error) {
	var id string
	err := db.QueryRow(`SELECT post_id FROM processed_posts WHERE post_id = ?`, postID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func markPostProcessed(db *sql.DB, postID string) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO processed_posts (post_id) VALUES (?)`, postID)
	return err
}
