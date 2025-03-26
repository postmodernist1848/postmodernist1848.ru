package domain

import "html/template"

type Note = struct {
	Date string        `json:"date"`
	HTML template.HTML `json:"html"`
}
