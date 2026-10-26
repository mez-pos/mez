package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// maxBodyBytes bounds a request body. Nothing a till sends is near this; a
// body that is, is a mistake or an attack, and either way is not read.
const maxBodyBytes = 64 << 10

// decodeJSON reads exactly one JSON object into dst. Unknown fields are an
// error: a client sending a field the server ignores is a client that thinks
// it asked for something it did not get.
// bodyReadTimeout is how long a request body may take to arrive once its
// headers have. A client that trickles a body holds a handler otherwise.
const bodyReadTimeout = 30 * time.Second

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// Best effort: a writer that cannot set one (a test's recorder) is left
	// alone.
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(bodyReadTimeout))
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("the body is not the JSON this endpoint takes: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("the body has more than one JSON value")
	}
	return nil
}
