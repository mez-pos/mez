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
// Every guest path is put under mount, unless it is there already: "/t/abc"
// becomes "/order/t/abc", "/order/assets/app.js" stays as it is, and
// "/api/v1/orders" becomes "/order/api/v1/orders", which is not the staff
// API and never will be. There is no list of what a guest may reach, to be
// kept in step with the routes: the guest app's mount is the whole of it,
// and what a guest needs from the server is served from under there.
func guestRequest(ctx context.Context, head wire.RequestHead, body []byte, mount string) (*http.Request, error) {
	u, err := url.ParseRequestURI(head.Path)
	if err != nil || u.Host != "" || u.Scheme != "" || !climbsNowhere(u.Path) || encodesSeparator(u.RawPath) {
		return nil, errNoSuchPath
	}
	// A path already under the mount is left there. The guest app names its
	// own files and its API by the paths the server has them at, "/order/...",
	// so that the same bundle works on the venue's wifi and through the
	// relay; putting those under the mount again would look for
	// "/order/order/...". Either way the request lands under the mount.
	if !strings.HasPrefix(u.Path, mount) {
		base := strings.TrimSuffix(mount, "/")
		if u.RawPath != "" {
			u.RawPath = base + u.RawPath
		}
		u.Path = base + u.Path
	}

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

// encodesSeparator reports whether an escaped path hides a slash or a
// backslash as %2F or %5C. The mount check below reads the decoded path,
// but net/http routes on the escaped one, segment by segment, so
// "/order%2Fx" would pass the check as "/order/x" and then miss every
// pattern under the mount and fall to whatever is at "/". No guest URL
// needs an encoded separator, so one is refused whole.
func encodesSeparator(rawPath string) bool {
	if rawPath == "" {
		return false
	}
	upper := strings.ToUpper(rawPath)
	return strings.Contains(upper, "%2F") || strings.Contains(upper, "%5C")
}

// remoteAddr is the guest's address as the relay reported it, in the form
// net/http gives one. The server logs it; it is not evidence of anything.
func remoteAddr(forwardedFor string) string {
	if ip := net.ParseIP(strings.TrimSpace(forwardedFor)); ip != nil {
		return net.JoinHostPort(ip.String(), "0")
	}
	return "0.0.0.0:0"
}
