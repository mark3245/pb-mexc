package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

type Settings struct {
	TimeInterval int     `json:"time_interval"`
	PriceChange  float64 `json:"price_change"`
	MinVolume    int     `json:"min_volume"`
}

type BlacklistEntry struct {
	Symbol    string    `json:"symbol"`
	ExpiresAt time.Time `json:"expires_at"`
}

func New(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := createTables(db); err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS blacklist (
			symbol TEXT PRIMARY KEY,
			expires_at DATETIME NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		INSERT OR IGNORE INTO settings (key, value) VALUES 
		('time_interval', '5'),
		('price_change', '2.0'),
		('min_volume', '5000')
	`)
	return err
}

func (d *Database) GetSettings() (*Settings, error) {
	rows, err := d.db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := &Settings{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}

		switch key {
		case "time_interval":
			if _, err := fmt.Sscanf(value, "%d", &settings.TimeInterval); err != nil {
				return nil, err
			}
		case "price_change":
			if _, err := fmt.Sscanf(value, "%f", &settings.PriceChange); err != nil {
				return nil, err
			}
		case "min_volume":
			if _, err := fmt.Sscanf(value, "%d", &settings.MinVolume); err != nil {
				return nil, err
			}
		}
	}

	return settings, nil
}

func (d *Database) UpdateSettings(settings *Settings) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("UPDATE settings SET value = ? WHERE key = ?",
		fmt.Sprintf("%d", settings.TimeInterval), "time_interval")
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE settings SET value = ? WHERE key = ?",
		fmt.Sprintf("%.2f", settings.PriceChange), "price_change")
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE settings SET value = ? WHERE key = ?",
		fmt.Sprintf("%d", settings.MinVolume), "min_volume")
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (d *Database) AddToBlacklist(symbol string, duration time.Duration) error {
	expiresAt := time.Now().Add(duration)
	_, err := d.db.Exec("INSERT OR REPLACE INTO blacklist (symbol, expires_at) VALUES (?, ?)",
		symbol, expiresAt)
	return err
}

func (d *Database) RemoveFromBlacklist(symbol string) error {
	_, err := d.db.Exec("DELETE FROM blacklist WHERE symbol = ?", symbol)
	return err
}

func (d *Database) GetBlacklist() ([]BlacklistEntry, error) {
	rows, err := d.db.Query("SELECT symbol, expires_at FROM blacklist WHERE expires_at > ? ORDER BY expires_at",
		time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []BlacklistEntry
	for rows.Next() {
		var entry BlacklistEntry
		if err := rows.Scan(&entry.Symbol, &entry.ExpiresAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (d *Database) IsBlacklisted(symbol string) (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM blacklist WHERE symbol = ? AND expires_at > ?",
		symbol, time.Now()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *Database) CleanupExpiredBlacklist() error {
	_, err := d.db.Exec("DELETE FROM blacklist WHERE expires_at <= ?", time.Now())
	return err
}
