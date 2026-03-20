package admin

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	adminui "SuperBotGo/web/admin"
)

// RegisterStaticRoutes registers the embedded Admin UI SPA on the given mux.
//
// Static assets (js, css, images, fonts) are served as-is with the correct
// Content-Type derived by http.FileServer.  All other paths under /admin/ that
// do not correspond to an existing file are served index.html so that the
// React SPA router can handle client-side navigation.
func RegisterStaticRoutes(mux *http.ServeMux) {

	subFS, err := fs.Sub(adminui.DistFS, "dist")
	if err != nil {
		panic("admin: failed to sub-tree embedded dist FS: " + err.Error())
	}

	handler := spaHandler{
		fs:         subFS,
		fileServer: http.FileServer(http.FS(subFS)),
	}

	mux.Handle("/admin/", http.StripPrefix("/admin", &handler))
}

// spaHandler serves static files from the embedded filesystem and falls back
// to index.html for paths that do not match an existing file (SPA routing).
type spaHandler struct {
	fs         fs.FS
	fileServer http.Handler
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	reqPath := path.Clean(r.URL.Path)
	if reqPath == "/" || reqPath == "." {

		h.fileServer.ServeHTTP(w, r)
		return
	}

	fsPath := strings.TrimPrefix(reqPath, "/")

	f, err := h.fs.Open(fsPath)
	if err == nil {
		stat, statErr := f.(fs.File).Stat()
		f.Close()
		if statErr == nil && !stat.IsDir() {

			h.fileServer.ServeHTTP(w, r)
			return
		}
	}

	r.URL.Path = "/"
	h.fileServer.ServeHTTP(w, r)
}
