// Package hub fans domain events out to subscribed devices over WebSocket.
//
// On connect a client sends its last seen sequence number, the hub replays the
// gap from SQLite and then streams live, so a kitchen tablet losing wifi is a
// non-event rather than an incident.
//
// A publish is only a nudge. Each connection reads the venue's stream from
// SQLite for itself, so replay and live streaming are one code path and a
// slow device falls behind on disk rather than in memory.
//
// Imports: internal/db for the stream and internal/domain for the event
// types. Never internal/api.
package hub
