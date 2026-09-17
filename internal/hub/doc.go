// Package hub fans domain events out to subscribed devices over WebSocket.
//
// On connect a client sends its last seen sequence number, the hub replays the
// gap from SQLite and then streams live, so a kitchen tablet losing wifi is a
// non-event rather than an incident.
//
// Imports: internal/domain for the event types. Never internal/api.
package hub
