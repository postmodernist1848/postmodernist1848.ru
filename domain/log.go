package domain

import "html/template"

type Log = struct {
	Date string        `json:"date"`
	HTML template.HTML `json:"html"`
}
