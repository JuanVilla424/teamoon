package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var embeddedFiles embed.FS

func staticFS() http.FileSystem {
	sub, _ := fs.Sub(embeddedFiles, "static")
	return http.FS(sub)
}

func staticContent(name string) ([]byte, error) {
	return embeddedFiles.ReadFile("static/" + name)
}
