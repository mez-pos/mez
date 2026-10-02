package api

import (
	"net/http"

	"github.com/byjoellobo/mez/internal/domain/export"
)

// csvResponse is a report as a CSV file (phase 10, Task 5b, ADR-119). Like
// a shift's exports it is JSON, not the file itself, so the back office
// reads it with the same signed-in client as everything else and saves
// Text as Filename.
type csvResponse struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Text        string `json:"text"`
}

// byteOrderMark opens the text. Without it a spreadsheet reads a UTF-8
// file as the machine's own code page, and an item named in Devanagari
// arrives as noise.
const byteOrderMark = "\xEF\xBB\xBF"

// writeCSV answers with a table as a CSV file.
func (s *Server) writeCSV(w http.ResponseWriter, r *http.Request, filename string, t export.Table) {
	body, err := t.CSV()
	if err != nil {
		s.log.Error("writing a CSV failed", "request_id", RequestID(r.Context()), "filename", filename, "error", err)
		writeError(w, s.log, http.StatusInternalServerError, CodeInternal, "internal error")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, s.log, http.StatusOK, csvResponse{
		Filename: filename, ContentType: "text/csv; charset=utf-8", Text: byteOrderMark + string(body),
	})
}
