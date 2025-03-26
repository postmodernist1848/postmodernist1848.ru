//go:build dev

package resources

import (
	"html/template"
	"os"
)

func LogTemplate() *template.Template {
	return template.Must(template.ParseFiles("resources/template/log.html.tmpl"))
}

func ContentsTemplate() *template.Template {
	return template.Must(template.ParseFiles("resources/template/contents.html.tmpl"))
}

var FS = os.DirFS("resources/")
