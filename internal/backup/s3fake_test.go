package backup_test

import (
	"net"
	"net/http/httptest"
	"testing"

	"github.com/mez-pos/mez/internal/backup/s3fake"
)

// fakeS3 is the stand-in bucket these tests back up to (internal/backup/s3fake).
type fakeS3 = s3fake.Bucket

func newFakeS3(t *testing.T, bucket, accessKey string) (*fakeS3, *httptest.Server) {
	t.Helper()
	f := s3fake.New(bucket, accessKey)
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
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
