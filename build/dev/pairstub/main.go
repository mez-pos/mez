// Command pairstub stands in for mez Cloud's one pairing route, so that
// pairing a venue can be walked through on one machine before there is a
// mez Cloud to pair with (Phase 14, Task 4c; the contract is ADR-318).
//
// It knows one code, good once, and answers it with a key it mints for a
// relay whose secret it is given. It is a developer's tool: it keeps
// nothing, checks nothing else, and is not built into any release.
//
// It also signs what mez Cloud signs to let the owner in (ADR-323, ADR-324),
// with the key whose public half it hands out at pairing:
//
//	curl 'http://127.0.0.1:7802/dev/assertion?venue=ven_...'
//
// answers an assertion good for five minutes and once, for the venue with
// that public id (`GET /api/v1/venue` says it). Whoever can reach this
// address is that venue's owner, so it listens on this machine only.
//
//	RELAY_SECRET=... go run ./build/dev/pairstub \
//	    -listen 127.0.0.1:7802 -relay http://127.0.0.1:7801 -venue dev-cafe -code WXYZ2345
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/relay"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:7802", "address to listen on")
	relayURL := flag.String("relay", "http://127.0.0.1:7801", "the relay a paired venue is sent to")
	venue := flag.String("venue", "dev-cafe", "the venue's name on the relay")
	domain := flag.String("domain", "mezpos.test", "the domain its hostname is under")
	code := flag.String("code", "WXYZ2345", "the one pairing code accepted, once")
	owner := flag.String("owner", "Priya Menon", "the account's name")
	flag.Parse()

	keys, err := relay.NewKeys(os.Getenv("RELAY_SECRET"), nil)
	if err != nil {
		log.Fatalf("RELAY_SECRET: %v", err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("GET /dev/assertion", func(w http.ResponseWriter, r *http.Request) {
		a, err := cloud.NewAssertion(r.URL.Query().Get("venue"), time.Now(), rand.Reader)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		signed, err := cloud.SignAssertion(priv, a)
		if err != nil {
			http.Error(w, err.Error()+"; pass ?venue=<the venue's public id>", http.StatusBadRequest)
			return
		}
		log.Printf("signed an assertion for venue %s, good until %s", a.Audience, a.ExpiresAt.Format(time.TimeOnly))
		fmt.Fprintln(w, signed)
	})

	var mu sync.Mutex
	spent := false
	http.HandleFunc("POST /v1/pair", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Code  string `json:"code"`
			Venue struct {
				ID, Name, Version string
			} `json:"venue"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
			http.Error(w, `{"error":{"code":"invalid_request"}}`, http.StatusBadRequest)
			return
		}
		mu.Lock()
		good := req.Code == *code && !spent
		if good {
			spent = true
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if !good {
			log.Printf("refused code %q from venue %q (%s)", req.Code, req.Venue.Name, req.Venue.ID)
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"code":"pairing_code_invalid"}}`)
			return
		}
		key, err := keys.Issue(rand.Reader, *venue, true)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("paired venue %q (%s, %s) as %s", req.Venue.Name, req.Venue.ID, req.Venue.Version, *venue)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"venue": map[string]string{"slug": *venue, "hostname": *venue + "." + *domain},
			"relay": map[string]string{"url": *relayURL, "key": key},
			// A bucket that is not there: the walk shows what pairing says
			// when the backup cannot be started.
			"backup": map[string]string{
				"bucket": "mez-dev", "path": "v/" + *venue, "endpoint": "http://127.0.0.1:9",
				"access_key_id": "dev", "secret_access_key": "dev-secret-not-a-real-one",
			},
			"cloud": map[string]string{"public_key": base64.StdEncoding.EncodeToString(pub)},
			"owner": map[string]string{"name": *owner},
		})
	})
	log.Printf("pairstub: listening on %s; the code is %s, once", *listen, *code)
	log.Fatal(http.ListenAndServe(*listen, nil))
}
