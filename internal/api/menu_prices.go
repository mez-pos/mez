package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/byjoellobo/mez/internal/service/menus"
)

// Prices from a file (phase 10, Task 9b, ADR-296). Both routes take the CSV
// file as the body, as the import's do, and answer the same shape.

// The refusals of a file of prices.
const (
	// CodeMenuPricesRefused is a file with a problem in it. The preview of
	// the same file says what the problems are.
	CodeMenuPricesRefused ErrorCode = "menu_prices_refused"
	// CodeMenuPricesStale is a file applied against a menu that has changed
	// since it was checked. The client checks it again.
	CodeMenuPricesStale ErrorCode = "menu_prices_stale"
)

func (s *Server) mountMenuPrices(handle func(pattern string, h http.HandlerFunc)) {
	handle("POST /api/v1/menu/prices/preview", s.previewMenuPrices)
	handle("POST /api/v1/menu/prices", s.applyMenuPrices)
}

// previewMenuPrices answers what a file would change. A file full of
// problems is still a 200: saying what is wrong with it is the answer.
func (s *Server) previewMenuPrices(w http.ResponseWriter, r *http.Request) {
	data, ok := s.importFile(w, r)
	if !ok {
		return
	}
	plan, err := s.menuEditor.PreviewPrices(r.Context(), Meta(r).VenueID, data)
	if err != nil {
		s.writeMenuEditError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, plan)
}

// applyMenuPrices changes the prices. `version` in the query is the menu
// version the preview answered: the owner agreed to that table of old and
// new, and it is refused if the menu has moved since.
func (s *Server) applyMenuPrices(w http.ResponseWriter, r *http.Request) {
	version, err := strconv.ParseInt(r.URL.Query().Get("version"), 10, 64)
	if err != nil || version < 0 {
		writeFieldError(w, s.log, http.StatusBadRequest, CodeInvalidRequest, "version must be the menu version the file was checked at", "version")
		return
	}
	data, ok := s.importFile(w, r)
	if !ok {
		return
	}
	done, err := s.menuEditor.ApplyPrices(r.Context(), Meta(r).VenueID, data, version)
	switch {
	case errors.Is(err, menus.ErrPricesRefused):
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeMenuPricesRefused, pricesRefusal(len(done.Problems)), "file")
	case errors.Is(err, menus.ErrPricesStale):
		writeError(w, s.log, http.StatusConflict, CodeMenuPricesStale, "The menu changed since this file was checked. Check it again.")
	case err != nil:
		s.writeMenuEditError(w, r, err)
	default:
		writeJSON(w, s.log, http.StatusOK, done)
	}
}

func pricesRefusal(problems int) string {
	if problems == 1 {
		return "The file has a problem, so no price was changed. Check it again to see what."
	}
	return fmt.Sprintf("The file has %d problems, so no price was changed. Check it again to see them.", problems)
}
