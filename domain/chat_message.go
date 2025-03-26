package domain

type ChatMessage struct {
	Author string `json:"author"`
	Text   string `json:"text"`
}
