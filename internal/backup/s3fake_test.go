package backup_test

import (
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeS3 is enough of S3 for Litestream to back up to and restore from:
// put, get (with a range), head, list and delete, on one bucket, for one
// access key. It is not S3. It does not check signatures, only that the
// request was signed by the key it knows, and it has no multipart upload,
// so nothing over the SDK's 5 MB part goes through it.
//
// It is here because the tests have to run on a venue's kind of machine,
// with no Docker and no internet, and what they are testing is this
// package's wiring: the keys it writes under, what it says when refused,
// what it does when nothing answers.
type fakeS3 struct {
	bucket, accessKey string

	mu      sync.Mutex
	objects map[string]fakeObject
	// requests counts what was asked, by method and whether it named a key.
	requests map[string]int
	// asked is every request: its method (LIST for a listing) and the key
	// or prefix it was about.
	asked []string
}

type fakeObject struct {
	body     []byte
	meta     map[string]string
	modified time.Time
}

func newFakeS3(t *testing.T, bucket, accessKey string) (*fakeS3, *httptest.Server) {
	t.Helper()
	f := &fakeS3{bucket: bucket, accessKey: accessKey, objects: map[string]fakeObject{}, requests: map[string]int{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

// keys lists what the bucket holds, sorted.
func (f *fakeS3) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, 0, len(f.objects))
	for k := range f.objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// plant puts an object in the bucket behind the backup's back.
func (f *fakeS3) plant(key string, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = fakeObject{body: body, meta: map[string]string{}, modified: time.Now().UTC()}
}

// countPrefix counts the requests of one method about keys under prefix.
func (f *fakeS3) countPrefix(method, prefix string) int {
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

func (f *fakeS3) count(what string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[what]
}

func s3Error(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>%s</Message></Error>`, code, code)
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	if !strings.Contains(r.Header.Get("Authorization"), "Credential="+f.accessKey+"/") {
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

func (f *fakeS3) put(w http.ResponseWriter, r *http.Request, key string) {
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
	f.objects[key] = fakeObject{body: body, meta: meta, modified: time.Now().UTC()}
	w.Header().Set("ETag", `"`+strconv.Itoa(len(body))+`"`)
	w.WriteHeader(http.StatusOK)
}

func (f *fakeS3) get(w http.ResponseWriter, r *http.Request, key string) {
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
}

type listContent struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	Size         int    `xml:"Size"`
}

func (f *fakeS3) list(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	out := listResult{Name: f.bucket, Prefix: prefix, MaxKeys: 1000}
	keys := make([]string, 0, len(f.objects))
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		obj := f.objects[k]
		out.Contents = append(out.Contents, listContent{Key: k, LastModified: obj.modified.Format("2006-01-02T15:04:05.000Z"), Size: len(obj.body)})
	}
	out.KeyCount = len(out.Contents)
	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(out)
}

func (f *fakeS3) deleteMany(w http.ResponseWriter, r *http.Request) {
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

// newServerOn serves the stand-in on a listener the test already holds,
// for a bucket that comes back at the address it could not be reached at.
func newServerOn(t *testing.T, l net.Listener, f *fakeS3) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(f)
	srv.Listener.Close()
	srv.Listener = l
	srv.Start()
	return srv
}
