package admin

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	adminui "SuperBotGo/web/admin"
)

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
