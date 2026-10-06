// Package tunnel keeps an outbound WebSocket open to the relay so guest phones
// can reach a server that has no public address.
//
// Outbound only, reconnecting with backoff. It needs no port forwarding, no
// static IP, and it works behind CGNAT. When it is down the counter keeps
// selling; see ADR-008.
//
// A guest's request arrives as frames (internal/relay/wire) and is served by
// calling the server's own handler in this process, never by dialling it. A
// guest reaches the guest app's mount and nothing else: every path is put
// under it before the handler sees it, and a path that could be read as a
// way out (a dot segment, an empty one, a backslash, an encoded slash) is
// refused rather than cleaned. See ADR-202.
//
// Imports: internal/relay/wire. It is handed an http.Handler and does not
// know what is behind it.
package tunnel
