package tunnel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/byjoellobo/mez/internal/relay/wire"
)

// errNoSuchPath is a guest path that is not one.
var errNoSuchPath = errors.New("tunnel: not a path a guest can ask for")

// guestRequest builds the request the server sees for a guest's frames.
//
// Every guest path is put under mount: "/t/abc" becomes "/order/t/abc", and
// "/api/v1/orders" becomes "/order/api/v1/orders", which is not the staff
// API and never will be. There is no list of what a guest may reach, to be
// kept in step with the routes: the guest app's mount is the whole of it,
// and what a guest needs from the server is served from under there.
func guestRequest(ctx context.Context, head wire.RequestHead, body []byte, mount string) (*http.Request, error) {
	u, err := url.ParseRequestURI(head.Path)
	if err != nil || u.Host != "" || u.Scheme != "" || !climbsNowhere(u.Path) {
		return nil, errNoSuchPath
	}
	base := strings.TrimSuffix(mount, "/")
	if u.RawPath != "" {
		u.RawPath = base + u.RawPath
	}
	u.Path = base + u.Path

	header := head.Header
	if header == nil {
		header = http.Header{}
	}
	req := &http.Request{
		Method:        head.Method,
		URL:           u,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Host:          header.Get("X-Forwarded-Host"),
		RequestURI:    u.RequestURI(),
		RemoteAddr:    remoteAddr(header.Get("X-Forwarded-For")),
	}
	return req.WithContext(ctx), nil
}

// climbsNowhere reports whether a path stays where it is put. One with a
// ".." in it, or an empty or backslashed segment, could be read by something
// further in as a way out from under the mount, so it is refused whole and
// not cleaned up: a guest's phone never sends one.
func climbsNowhere(p string) bool {
	if !strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00") {
		return false
	}
	segments := strings.Split(p[1:], "/")
	for i, seg := range segments {
		if seg == ".." || seg == "." {
			return false
		}
		// A trailing slash is a directory; an empty segment anywhere else
		// is "//".
		if seg == "" && i != len(segments)-1 {
			return false
		}
	}
	return true
}

// remoteAddr is the guest's address as the relay reported it, in the form
// net/http gives one. The server logs it; it is not evidence of anything.
func remoteAddr(forwardedFor string) string {
	if ip := net.ParseIP(strings.TrimSpace(forwardedFor)); ip != nil {
		return net.JoinHostPort(ip.String(), "0")
	}
	return "0.0.0.0:0"
}
