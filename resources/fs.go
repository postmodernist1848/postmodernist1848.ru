//go:build !dev

package resources

import (
	"embed"
	"html/template"
)

//go:embed template/log.html.tmpl
var logTemplateString string
var logTemplate = template.Must(template.New("log").Parse(logTemplateString))

func LogTemplate() *template.Template {
	return logTemplate
}

//go:embed template/contents.html.tmpl
var contentsTemplateString string
var contentsTemplate = template.Must(template.New("contents").Parse(contentsTemplateString))

func ContentsTemplate() *template.Template {
	return contentsTemplate
}

//go:embed articles assets static contents index.html
var FS embed.FS
