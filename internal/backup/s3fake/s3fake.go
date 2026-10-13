// Package s3fake is a stand-in for an S3 bucket, in memory, for the tests of
// the backup and for walking through pairing and "I had mez before" on one
// machine (build/dev/pairstub). It is here, and not in a test file, because
// both need it: the tests have to run on a venue's kind of machine, with no
// Docker and no internet, and what they test is this repository's wiring,
// the keys it writes under, what it says when refused, what it does when
// nothing answers. It is built into no release.
package s3fake

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Bucket is enough of S3 for Litestream to back up to and restore from:
// put, get (with a range), head, list and delete, on one bucket. It is not
// S3. It does not check signatures, only that the request was signed by a
// key it knows, and it has no multipart upload, so nothing over the part
// size the backup sets (16 MB) goes through it.
//
// It keeps everything in memory, and forgets it when the process ends.
type Bucket struct {
	bucket, accessKey string
	// readOnly, when set, is a second key that may look and may not write.
	readOnly string

	mu      sync.Mutex
	objects map[string]object
	// requests counts what was asked, by method and whether it named a key.
	requests map[string]int
	// asked is every request: its method (LIST for a listing) and the key
	// or prefix it was about.
	asked []string
}

// New is an empty bucket of that name, written and read with accessKey.
func New(bucket, accessKey string) *Bucket {
	return &Bucket{bucket: bucket, accessKey: accessKey, objects: map[string]object{}, requests: map[string]int{}}
}

// WithReader gives the bucket a second key, which reads and lists and is
// refused anything that would change it: what a restore is handed (ADR-332).
// Set it before the bucket is served.
func (f *Bucket) WithReader(accessKey string) *Bucket {
	f.readOnly = accessKey
	return f
}

type object struct {
	body     []byte
	meta     map[string]string
	modified time.Time
}

// Keys lists what the bucket holds, sorted.
func (f *Bucket) Keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, 0, len(f.objects))
	for k := range f.objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Plant puts an object in the bucket behind the backup's back.
func (f *Bucket) Plant(key string, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = object{body: body, meta: map[string]string{}, modified: time.Now().UTC()}
}

// CountPrefix counts the requests of one method about keys under prefix.
func (f *Bucket) CountPrefix(method, prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, a := range f.asked {
		if strings.HasPrefix(a, method+" "+prefix) {
			n++
		}
	}
	return n
}

// Count is how many requests there have been of one kind: "PUT object",
// "GET bucket".
func (f *Bucket) Count(what string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[what]
}

func s3Error(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>%s</Message></Error>`, code, code)
}

func (f *Bucket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Path style: /bucket/key.
	bucket, key, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")

	f.mu.Lock()
	defer f.mu.Unlock()
	if key == "" {
		f.requests[r.Method+" bucket"]++
		if r.Method == http.MethodGet {
			f.asked = append(f.asked, "LIST "+r.URL.Query().Get("prefix"))
		}
	} else {
		f.requests[r.Method+" object"]++
		f.asked = append(f.asked, r.Method+" "+key)
	}

	signedBy := func(key string) bool {
		return key != "" && strings.Contains(r.Header.Get("Authorization"), "Credential="+key+"/")
	}
	switch {
	case signedBy(f.accessKey):
	case signedBy(f.readOnly):
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			s3Error(w, http.StatusForbidden, "AccessDenied")
			return
		}
	default:
		s3Error(w, http.StatusForbidden, "InvalidAccessKeyId")
		return
	}
	if bucket != f.bucket {
		s3Error(w, http.StatusNotFound, "NoSuchBucket")
		return
	}

	switch {
	case key == "" && r.Method == http.MethodGet:
		f.list(w, r)
	case key == "" && r.Method == http.MethodPost && r.URL.Query().Has("delete"):
		f.deleteMany(w, r)
	case key != "" && r.Method == http.MethodPut:
		f.put(w, r, key)
	case key != "" && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		f.get(w, r, key)
	default:
		s3Error(w, http.StatusNotImplemented, "NotImplemented")
	}
}

func (f *Bucket) put(w http.ResponseWriter, r *http.Request, key string) {
	if r.URL.Query().Has("uploads") || r.URL.Query().Has("uploadId") || strings.Contains(r.Header.Get("Content-Encoding"), "aws-chunked") {
		s3Error(w, http.StatusNotImplemented, "NotImplemented")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s3Error(w, http.StatusBadRequest, "IncompleteBody")
		return
	}
	meta := map[string]string{}
	for name, values := range r.Header {
		if lower := strings.ToLower(name); strings.HasPrefix(lower, "x-amz-meta-") {
			meta[lower] = values[0]
		}
	}
	f.objects[key] = object{body: body, meta: meta, modified: time.Now().UTC()}
	w.Header().Set("ETag", `"`+strconv.Itoa(len(body))+`"`)
	w.WriteHeader(http.StatusOK)
}

func (f *Bucket) get(w http.ResponseWriter, r *http.Request, key string) {
	obj, ok := f.objects[key]
	if !ok {
		s3Error(w, http.StatusNotFound, "NoSuchKey")
		return
	}
	for name, value := range obj.meta {
		w.Header().Set(name, value)
	}
	w.Header().Set("ETag", `"`+strconv.Itoa(len(obj.body))+`"`)
	// ServeContent answers a Range the way S3 does, and HEAD with no body.
	http.ServeContent(w, r, "", obj.modified, strings.NewReader(string(obj.body)))
}

type listResult struct {
	XMLName     xml.Name      `xml:"ListBucketResult"`
	Name        string        `xml:"Name"`
	Prefix      string        `xml:"Prefix"`
	KeyCount    int           `xml:"KeyCount"`
	MaxKeys     int           `xml:"MaxKeys"`
	IsTruncated bool          `xml:"IsTruncated"`
	Contents    []listContent `xml:"Contents"`
	Folders     []listFolder  `xml:"CommonPrefixes"`
}

type listFolder struct {
	Prefix string `xml:"Prefix"`
}

type listContent struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	Size         int    `xml:"Size"`
}

func (f *Bucket) list(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	out := listResult{Name: f.bucket, Prefix: prefix, MaxKeys: 1000}
	keys := make([]string, 0, len(f.objects))
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	delimiter := r.URL.Query().Get("delimiter")
	folders := map[string]bool{}
	for _, k := range keys {
		// With a delimiter, what lies beyond the next one is a "folder",
		// named once and not listed.
		if rest := strings.TrimPrefix(k, prefix); delimiter != "" && strings.Contains(rest, delimiter) {
			folder := prefix + rest[:strings.Index(rest, delimiter)+len(delimiter)]
			if !folders[folder] {
				folders[folder] = true
				out.Folders = append(out.Folders, listFolder{Prefix: folder})
			}
			continue
		}
		obj := f.objects[k]
		out.Contents = append(out.Contents, listContent{Key: k, LastModified: obj.modified.Format("2006-01-02T15:04:05.000Z"), Size: len(obj.body)})
	}
	out.KeyCount = len(out.Contents)
	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(out)
}

func (f *Bucket) deleteMany(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Objects []struct {
			Key string `xml:"Key"`
		} `xml:"Object"`
	}
	if err := xml.NewDecoder(r.Body).Decode(&in); err != nil {
		s3Error(w, http.StatusBadRequest, "MalformedXML")
		return
	}
	for _, o := range in.Objects {
		delete(f.objects, o.Key)
	}
	w.Header().Set("Content-Type", "application/xml")
	io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><DeleteResult></DeleteResult>`)
}
