package api

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"

	"github.com/byjoellobo/mez/internal/service/menus"
)

// The menu from a spreadsheet (phase 10, Task 7b, ADR-288). Both routes take
// the CSV file itself as the body, as the photo route takes a JPEG.
//
// Neither stores an idempotency key. An import adds only what the menu does
// not have, so the same file sent twice adds nothing the second time.

// CodeMenuImportRefused is an import of a file with a problem in it. The
// preview of the same file says what the problems are.
const CodeMenuImportRefused ErrorCode = "menu_import_refused"

func (s *Server) mountMenuImport(handle func(pattern string, h http.HandlerFunc)) {
	handle("POST /api/v1/menu/import/preview", s.previewMenuImport)
	handle("POST /api/v1/menu/import", s.importMenu)
}

// previewMenuImport answers what a file would add. A file full of problems
// is still a 200: saying what is wrong with it is the answer.
func (s *Server) previewMenuImport(w http.ResponseWriter, r *http.Request) {
	data, ok := s.importFile(w, r)
	if !ok {
		return
	}
	plan, err := s.menuEditor.PreviewImport(r.Context(), Meta(r).VenueID, data)
	if err != nil {
		s.writeMenuEditError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, plan)
}

func (s *Server) importMenu(w http.ResponseWriter, r *http.Request) {
	data, ok := s.importFile(w, r)
	if !ok {
		return
	}
	done, err := s.menuEditor.ImportMenu(r.Context(), Meta(r).VenueID, data)
	if errors.Is(err, menus.ErrImportRefused) {
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeMenuImportRefused, importRefusal(len(done.Problems)), "file")
		return
	}
	if err != nil {
		s.writeMenuEditError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, done)
}

func importRefusal(problems int) string {
	if problems == 1 {
		return "The file has a problem, so nothing was imported. Check it again to see what."
	}
	return fmt.Sprintf("The file has %d problems, so nothing was imported. Check it again to see them.", problems)
}

// importFile reads the request's body as a CSV file of at most
// menus.MaxImportBytes. It has answered when it returns false.
func (s *Server) importFile(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	// A browser adds a charset; the type is what is checked.
	if ct, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || ct != "text/csv" {
		writeFieldError(w, s.log, http.StatusUnsupportedMediaType, CodeInvalidMenu, "Send the file as text/csv.", "file")
		return nil, false
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, menus.MaxImportBytes+1))
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig), len(data) > menus.MaxImportBytes:
		writeFieldError(w, s.log, http.StatusRequestEntityTooLarge, CodeInvalidMenu,
			fmt.Sprintf("That file is over %d KB. Import it in parts.", menus.MaxImportBytes>>10), "file")
		return nil, false
	case err != nil:
		writeError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, "the file could not be read")
		return nil, false
	}
	return data, true
}
