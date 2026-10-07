package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/byjoellobo/mez/internal/domain/export"
	domstock "github.com/byjoellobo/mez/internal/domain/stock"
	"github.com/byjoellobo/mez/internal/service/stock"
)

// Ingredients from a spreadsheet (phase 10, Task 10a, ADR-298): the menu
// import's three routes, for the store. Each takes the CSV file as its
// body. Neither write stores an idempotency key: an import adds only what
// is not there, so the same file sent twice adds nothing the second time.

// CodeStockImportRefused is an import of a file with a problem in it. The
// preview of the same file says what the problems are.
const CodeStockImportRefused ErrorCode = "stock_import_refused"

// stockItemsTemplateName is the file the ingredients template is saved as.
const stockItemsTemplateName = "mez-ingredients-template.csv"

func (s *Server) mountStockImport(handle func(pattern string, h http.HandlerFunc)) {
	handle("GET /api/v1/stock/import/template", s.stockItemsTemplate)
	handle("POST /api/v1/stock/import/preview", s.previewStockItems)
	handle("POST /api/v1/stock/import", s.importStockItems)
}

// stockItemsTemplate answers the file to fill in: the columns, and an
// example counted each way.
func (s *Server) stockItemsTemplate(w http.ResponseWriter, r *http.Request) {
	rows, err := s.stock.ItemsTemplate(r.Context(), Meta(r).VenueID)
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	s.writeCSV(w, r, stockItemsTemplateName, export.Table{Header: domstock.ItemColumns, Rows: rows})
}

// previewStockItems answers what a file would add. A file full of problems
// is still a 200: saying what is wrong with it is the answer.
func (s *Server) previewStockItems(w http.ResponseWriter, r *http.Request) {
	data, ok := s.importFile(w, r, CodeInvalidStock)
	if !ok {
		return
	}
	plan, err := s.stock.PreviewItems(r.Context(), Meta(r).VenueID, data)
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, plan)
}

func (s *Server) importStockItems(w http.ResponseWriter, r *http.Request) {
	data, ok := s.importFile(w, r, CodeInvalidStock)
	if !ok {
		return
	}
	done, err := s.stock.ImportItems(r.Context(), Meta(r).VenueID, data)
	if errors.Is(err, stock.ErrImportRefused) {
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeStockImportRefused, stockImportRefusal(len(done.Problems)), "file")
		return
	}
	if err != nil {
		s.writeStockError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, done)
}

func stockImportRefusal(problems int) string {
	if problems == 1 {
		return "The file has a problem, so nothing was imported. Check it again to see what."
	}
	return fmt.Sprintf("The file has %d problems, so nothing was imported. Check it again to see them.", problems)
}
