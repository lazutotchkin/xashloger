package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*.css
var staticFS embed.FS

func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "static fs error", http.StatusInternalServerError)
		})
	}
	return http.FileServer(http.FS(sub))
}
