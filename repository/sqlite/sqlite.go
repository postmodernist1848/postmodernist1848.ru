package sqlite

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"log"
	"postmodernist1848.ru/domain"
)

var Database *sql.DB

func init() {
	var err error
	Database, err = sql.Open("sqlite3", "database.db")
	if err != nil {
		log.Fatal("Failed to open sqlite database: ", err)
	}
}

func GetNotes() ([]domain.Note, error) {
	rows, err := Database.Query(`SELECT date, html FROM note`)
	if err != nil {
		return nil, fmt.Errorf("failed to query note table: %w", err)
	}
	var logs []domain.Note
	for rows.Next() {
		var date string
		var HTML string
		err = rows.Scan(&date, &HTML)
		if err != nil {
			return nil, fmt.Errorf("failed to read note table row: %w", err)
		}
		logs = append(logs, domain.Note{date, template.HTML(HTML)})
	}
	return logs, nil
}

func RewriteNotes(logs []domain.Note) error {
	tx, err := Database.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec(`DELETE FROM note`)
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO note(date, html) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	for _, l := range logs {
		_, err = stmt.Exec(l.Date, l.HTML)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func GetChatMessages() ([]domain.ChatMessage, error) {
	row, err := Database.Query("SELECT * FROM message ORDER BY id")
	if err != nil {
		return nil, err
	}
	var chatMessages []domain.ChatMessage
	for row.Next() {
		var id int
		var author, text string
		err = row.Scan(&id, &author, &text)
		if err != nil {
			return nil, err
		}
		chatMessages = append(chatMessages, domain.ChatMessage{Author: author, Text: text})
	}
	return chatMessages, nil
}

func InsertChatMessage(message domain.ChatMessage) error {
	_, err := Database.Exec(`INSERT INTO message(author, text) VALUES (?, ?)`,
		message.Author, message.Text)
	return err
}
