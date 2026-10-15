// Command pairstub stands in for mez Cloud's routes that a venue calls, so
// that pairing a venue and bringing one back can be walked through on one
// machine before there is a mez Cloud (Phase 14, Tasks 4c and 7; the
// contracts are ADR-318 and ADR-332).
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
// With -rotated it has rotated its key (ADR-345): it hands out the first
// key at pairing and signs with a second, presenting the link from one to
// the other. The venue's first sign-in from away puts the second key in
// its cloud.json.
//
// And it is one account, for "I had mez before": the phone number of
// -phone, whose one-time code is always -otp and is sent nowhere, owning one
// venue: the one that last paired with it, or the one of -venue-id until
// one has. Handing that venue over makes the pairing code good once more.
//
// With -bucket-fake it is the bucket too (internal/backup/s3fake), in
// memory, on an address of its own: a venue paired to it backs up there, and
// one brought back is restored from there with a key that only reads. That
// is the whole of "The PC died" on one machine: set a venue up and pair it,
// let it back up, move its data directory away, start mez on an empty one,
// and bring the venue back. Without it the bucket named is the one of the
// -bucket flags, which by default is not there: what pairing and a restore
// each say of that is worth seeing once.
//
//	RELAY_SECRET=... go run ./build/dev/pairstub -bucket-fake 127.0.0.1:7803
//	bin/mez -data-dir <scratch> -cloud-url http://127.0.0.1:7802
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/byjoellobo/mez/internal/backup/s3fake"
	"github.com/byjoellobo/mez/internal/cloud"
	"github.com/byjoellobo/mez/internal/relay"
)

// bucketSecretEnv is where the bucket's secret is read from, as `mez
// restore` reads one: not from a flag.
const bucketSecretEnv = "PAIRSTUB_BUCKET_SECRET"

// stub is the one account, its one venue and its one code.
type stub struct {
	keys *relay.Keys
	pub  ed25519.PublicKey
	priv ed25519.PrivateKey
	// signer signs assertions and chain is presented with them: priv and
	// none, or with -rotated a second key and the link to it.
	signer ed25519.PrivateKey
	chain  []string

	relayURL, venue, domain, code, owner string
	phone, otp                           string
	// bucket is what a paired venue is told to back up to, and readBucket
	// what one being brought back is told to restore from: the same place,
	// and with -bucket-fake a key that cannot write to it.
	bucket, readBucket map[string]string

	mu sync.Mutex
	// venueID and venueName are the account's one venue.
	venueID, venueName string
	spent              bool
	attempt            string // the last one asked for with the account's number
	session            string
}

func main() {
	listen := flag.String("listen", "127.0.0.1:7802", "address to listen on")
	s := &stub{}
	flag.StringVar(&s.relayURL, "relay", "http://127.0.0.1:7801", "the relay a paired venue is sent to")
	flag.StringVar(&s.venue, "venue", "dev-cafe", "the venue's name on the relay")
	flag.StringVar(&s.domain, "domain", "mezpos.test", "the domain its hostname is under")
	flag.StringVar(&s.code, "code", "WXYZ2345", "the one pairing code accepted, once")
	flag.StringVar(&s.owner, "owner", "Priya Menon", "the account's name")
	flag.StringVar(&s.phone, "phone", "+919876500000", "the account's phone number, for \"I had mez before\"")
	flag.StringVar(&s.otp, "otp", "123456", "the one-time code that number is always \"sent\"")
	flag.StringVar(&s.venueID, "venue-id", "", "the public id of the venue the account lists until one pairs; none lists no venue until then")
	flag.StringVar(&s.venueName, "venue-name", "Dev Cafe", "that venue's name in the account's list")
	rotated := flag.Bool("rotated", false, "sign assertions with a second key, handed on from the one given at pairing by a link (ADR-345)")
	fake := flag.String("bucket-fake", "", "serve a bucket in memory on this address (127.0.0.1:7803, say) and name it, in place of the -bucket flags' one")
	bucket := flag.String("bucket", "mez-dev", "the bucket a paired venue backs up to and a recovered one restores from")
	bucketPath := flag.String("bucket-path", "", "the venue's folder in it (default v/<venue>)")
	// A bucket that is not there, unless told: the walk shows what pairing
	// says when the backup cannot be started.
	bucketEndpoint := flag.String("bucket-endpoint", "http://127.0.0.1:9", "the bucket's service")
	bucketKeyID := flag.String("bucket-key-id", "dev", "the bucket's access key id; its secret is read from "+bucketSecretEnv)
	flag.Parse()

	if *bucketPath == "" {
		*bucketPath = "v/" + s.venue
	}
	secret := os.Getenv(bucketSecretEnv)
	if secret == "" {
		secret = "dev-secret-not-a-real-one"
	}
	s.bucket = map[string]string{
		"bucket": *bucket, "path": *bucketPath, "endpoint": *bucketEndpoint,
		"access_key_id": *bucketKeyID, "secret_access_key": secret,
	}
	s.readBucket = s.bucket
	if *fake != "" {
		// It takes anyone's word for which key they hold, so it is this
		// machine's alone, like the rest of this.
		if host, _, err := net.SplitHostPort(*fake); err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
			log.Fatalf("-bucket-fake %s: it must be an address on this machine, like 127.0.0.1:7803", *fake)
		}
		// Two keys to the one bucket, as mez Cloud will mint them: the
		// venue's writes, and a restore's only reads (ADR-332).
		const writer, reader = "dev-writer", "dev-reader"
		s.bucket = map[string]string{"bucket": *bucket, "path": *bucketPath, "endpoint": "http://" + *fake, "access_key_id": writer, "secret_access_key": secret}
		s.readBucket = map[string]string{"bucket": *bucket, "path": *bucketPath, "endpoint": "http://" + *fake, "access_key_id": reader, "secret_access_key": secret}
		held := s3fake.New(*bucket, writer).WithReader(reader)
		go func() { log.Fatal(http.ListenAndServe(*fake, held)) }()
		log.Printf("pairstub: a bucket in memory at http://%s, forgotten when this stops", *fake)
	}

	var err error
	if s.keys, err = relay.NewKeys(os.Getenv("RELAY_SECRET"), nil); err != nil {
		log.Fatalf("RELAY_SECRET: %v", err)
	}
	if s.pub, s.priv, err = ed25519.GenerateKey(rand.Reader); err != nil {
		log.Fatal(err)
	}
	s.signer = s.priv
	if *rotated {
		next, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			log.Fatal(err)
		}
		link, err := cloud.SignKeyLink(s.priv, next)
		if err != nil {
			log.Fatal(err)
		}
		s.signer, s.chain = priv, []string{link}
	}

	http.HandleFunc("GET /dev/assertion", s.assertion)
	http.HandleFunc("POST /v1/pair", s.pair)
	http.HandleFunc("POST /v1/recover/start", s.recoverStart)
	http.HandleFunc("POST /v1/recover/verify", s.recoverVerify)
	http.HandleFunc("POST /v1/recover/grant", s.recoverGrant)
	log.Printf("pairstub: listening on %s; the code is %s, once; the account is %s and its one-time code is %s", *listen, s.code, s.phone, s.otp)
	log.Fatal(http.ListenAndServe(*listen, nil))
}

func (s *stub) assertion(w http.ResponseWriter, r *http.Request) {
	a, err := cloud.NewAssertion(r.URL.Query().Get("venue"), time.Now(), rand.Reader)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	signed, err := cloud.SignAssertion(s.signer, a)
	if err != nil {
		http.Error(w, err.Error()+"; pass ?venue=<the venue's public id>", http.StatusBadRequest)
		return
	}
	log.Printf("signed an assertion for venue %s, good until %s, with %d links", a.Audience, a.ExpiresAt.Format(time.TimeOnly), len(s.chain))
	fmt.Fprintln(w, cloud.PresentAssertion(signed, s.chain...))
}

func (s *stub) pair(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code  string `json:"code"`
		Venue struct {
			ID, Name, Version string
		} `json:"venue"`
	}
	if !question(w, r, &req) {
		return
	}
	s.mu.Lock()
	good := req.Code == s.code && !s.spent
	if good {
		s.spent = true
		// The venue that pairs is the account's venue from here on.
		if req.Venue.ID != "" {
			s.venueID, s.venueName = req.Venue.ID, req.Venue.Name
		}
	}
	s.mu.Unlock()
	if !good {
		log.Printf("refused code %q from venue %q (%s)", req.Code, req.Venue.Name, req.Venue.ID)
		refuse(w, http.StatusNotFound, "pairing_code_invalid")
		return
	}
	key, err := s.keys.Issue(rand.Reader, s.venue, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("paired venue %q (%s, %s) as %s", req.Venue.Name, req.Venue.ID, req.Venue.Version, s.venue)
	answer(w, map[string]any{
		"venue":  map[string]string{"slug": s.venue, "hostname": s.venue + "." + s.domain},
		"relay":  map[string]string{"url": s.relayURL, "key": key},
		"backup": s.bucket,
		"cloud":  map[string]string{"public_key": base64.StdEncoding.EncodeToString(s.pub)},
		"owner":  map[string]string{"name": s.owner},
	})
}

// recoverStart answers every number alike, as the cloud will: only the log
// here says whether it was the account's.
func (s *stub) recoverStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
	}
	if !question(w, r, &req) {
		return
	}
	attempt := "att_" + token()
	if req.Phone == s.phone {
		s.mu.Lock()
		s.attempt = attempt
		s.mu.Unlock()
		log.Printf("a one-time code was asked for by %s: it is %s", req.Phone, s.otp)
	} else {
		log.Printf("a one-time code was asked for by %s, which is not the account's (%s): none is sent", req.Phone, s.phone)
	}
	answer(w, map[string]string{"attempt": attempt})
}

func (s *stub) recoverVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Attempt string `json:"attempt"`
		Code    string `json:"code"`
	}
	if !question(w, r, &req) {
		return
	}
	s.mu.Lock()
	good := s.attempt != "" && req.Attempt == s.attempt && req.Code == s.otp
	session := ""
	if good {
		// An attempt is good once, and a new sign-in ends the one before.
		s.attempt, s.session = "", "ses_"+token()
		session = s.session
	}
	venueID, venueName := s.venueID, s.venueName
	s.mu.Unlock()
	if !good {
		log.Printf("refused one-time code %q", req.Code)
		refuse(w, http.StatusForbidden, "otp_invalid")
		return
	}
	venues := []map[string]any{}
	if venueID != "" {
		// When it was last backed up is not looked for: a minute ago, always.
		venues = append(venues, map[string]any{"id": venueID, "name": venueName, "last_backup_at": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)})
	}
	log.Printf("%s signed in to recover; %d venue(s) listed", s.owner, len(venues))
	answer(w, map[string]any{
		"session": session, "expires_at": time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339),
		"owner": map[string]string{"name": s.owner}, "venues": venues,
	})
}

func (s *stub) recoverGrant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Venue struct {
			ID string `json:"id"`
		} `json:"venue"`
	}
	if !question(w, r, &req) {
		return
	}
	bearer, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	s.mu.Lock()
	signedIn := s.session != "" && bearer == s.session
	theirs := req.Venue.ID != "" && req.Venue.ID == s.venueID
	if signedIn && theirs {
		// The venue is to be paired afresh: its code is good once more.
		s.spent = false
	}
	s.mu.Unlock()
	switch {
	case !signedIn:
		refuse(w, http.StatusUnauthorized, "session_invalid")
	case !theirs:
		refuse(w, http.StatusNotFound, "venue_not_found")
	default:
		log.Printf("handed over venue %s: its bucket, and the pairing code %s, good once more", req.Venue.ID, s.code)
		answer(w, map[string]any{
			"backup": s.readBucket, "pairing_code": s.code,
			"expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		})
	}
}

// question reads a request's JSON, and answers 400 itself when it is not.
func question(w http.ResponseWriter, r *http.Request, into any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(into); err != nil {
		refuse(w, http.StatusBadRequest, "invalid_request")
		return false
	}
	return true
}

func answer(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func refuse(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code}})
}

func token() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		log.Fatal(err)
	}
	return hex.EncodeToString(raw)
}
