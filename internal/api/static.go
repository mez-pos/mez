package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// indexFile is what every SPA mount falls back to.
const indexFile = "index.html"

// assetDir is where Vite writes its content-hashed output. Everything under it
// can be cached forever precisely because its name changes when it changes.
const assetDir = "assets/"

// spa serves one built single-page app out of fsys.
//
// The fallback rule is the whole point, and it is narrower than "serve
// index.html for anything missing". A path with no extension is a route the
// router inside the app will handle, so it gets index.html. A path with an
// extension is a file the app asked for by name, so a missing one gets a 404.
//
// Serving index.html for a missing .js would hand the browser HTML with a 200
// and a MIME error in the console, which reads like a bundler problem and is
// one of the more expensive ways to lose an afternoon.
func (s *Server) spa(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := requestedFile(r.URL.Path)

		info, err := fs.Stat(fsys, name)
		switch {
		case err == nil && !info.IsDir():
			setCacheControl(w, name)
			files.ServeHTTP(w, r)

		case path.Ext(name) != "":
			// A named file that is not there. Say so.
			writeError(w, s.log, http.StatusNotFound, CodeNotFound, "no such file")

		default:
			// A route. Let the app deal with it.
			s.serveIndex(w, r, fsys)
		}
	})
}

// requestedFile turns a URL path into a name inside the bundle. Directories and
// the mount root both mean index.html.
func requestedFile(urlPath string) string {
	name := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if name == "" || name == "." || strings.HasSuffix(urlPath, "/") {
		return indexFile
	}
	return name
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	index, err := fs.ReadFile(fsys, indexFile)
	if err != nil {
		// Bundles are checked for index.html at startup, so this is a bundle
		// that changed underneath a running process.
		s.log.Error("bundle has no index.html", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}

	setCacheControl(w, indexFile)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(index); err != nil {
		s.log.Debug("writing index failed", "error", err)
	}
}

// setCacheControl decides how long the browser may keep a file.
//
// index.html names the hashed assets, so a cached copy pins a tablet to the
// build it was opened on: after an update the kitchen display would keep
// loading yesterday's bundle until someone thought to hard-refresh it. The
// hashed assets themselves are immutable by construction.
func setCacheControl(w http.ResponseWriter, name string) {
	if strings.HasPrefix(name, assetDir) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
}

// mountStatic registers every configured bundle on mux.
//
// A mount other than the root is registered as a subtree and has its prefix
// stripped, so the bundle sees the paths it was built with. net/http redirects
// /backoffice to /backoffice/ on its own.
func (s *Server) mountStatic(mux *http.ServeMux) {
	for mount, fsys := range s.bundles {
		handler := s.spa(fsys)
		if mount == "/" {
			mux.Handle("/", handler)
			continue
		}
		mux.Handle(mount, http.StripPrefix(strings.TrimSuffix(mount, "/"), handler))
	}
}
