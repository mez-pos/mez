package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/domain/order"
)

// The message types, in both directions. They are a contract: a terminal
// switches on them, and they are generated into TypeScript.
const (
	// From the client.
	MsgHello     MessageType = "hello"     // first message: where I am, what I want
	MsgSubscribe MessageType = "subscribe" // change my topics

	// From the server.
	MsgWelcome      MessageType = "welcome"       // your position and what you are subscribed to
	MsgEvent        MessageType = "event"         // one event
	MsgCaughtUp     MessageType = "caught_up"     // you have everything up to this position
	MsgMenuChanged  MessageType = "menu_changed"  // the venue's menu has a new version
	MsgPrintChanged MessageType = "print_changed" // the venue's print jobs moved
	MsgFloorChanged MessageType = "floor_changed" // the venue's floor moved other than by an order event
	MsgError        MessageType = "error"         // what you sent could not be used
)

// MessageType names a message in either direction.
type MessageType string

// ClientMessage is anything a client sends to the hub. Only a handful of
// fields are ever set; the type says which.
type ClientMessage struct {
	Type MessageType `json:"type"`

	// LastEventID is the position the client has. Zero with Replay false
	// means "only what happens next", which is what a screen opening fresh
	// wants; a reconnecting one sends the last id it saw.
	LastEventID int64 `json:"last_event_id,omitempty"`

	// FromStart asks for the venue's whole stream. A kitchen display
	// rebuilding itself uses it.
	FromStart bool `json:"from_start,omitempty"`

	Topics []string `json:"topics,omitempty"`
}

// WelcomeMessage answers the hello, and a later subscribe.
type WelcomeMessage struct {
	Type     MessageType `json:"type"`
	VenueID  int64       `json:"venue_id"`
	DeviceID int64       `json:"device_id"`
	From     int64       `json:"from_event_id"`
	Latest   int64       `json:"latest_event_id"`
	Topics   []string    `json:"topics"`

	// MenuVersion is the venue's menu as of this welcome. A client whose
	// cached menu is another version fetches it again: a reconnect is when
	// a menu_changed missed while offline is caught.
	MenuVersion int64 `json:"menu_version"`
	// PrintVersion is the venue's print jobs as of this welcome.
	PrintVersion int64 `json:"print_version"`
	// FloorVersion is the venue's floor as of this welcome.
	FloorVersion int64 `json:"floor_version"`
}

// EventMessage is one event on the wire, with the little about its order
// that a screen needs to file it.
type EventMessage struct {
	Type    MessageType `json:"type"`
	ID      int64       `json:"id"`
	OrderID string      `json:"order_id"`
	Status  string      `json:"order_status"`
	TableID int64       `json:"table_id,omitempty"`
	Station string      `json:"station,omitempty"`
	Event   order.Event `json:"event"`
}

// MenuChangedMessage says the venue's menu is at a new version. It goes to
// every connection, whatever its topics: a kitchen watching one station
// still needs that station's new items. It carries the version and not the
// menu, which the client fetches from GET /api/v1/menu.
type MenuChangedMessage struct {
	Type    MessageType `json:"type"`
	Version int64       `json:"version"`
}

// PrintChangedMessage says the venue's print jobs have moved: queued,
// printed, failed, retried. Like the menu's, it goes to every connection
// and carries a version, not the jobs, which the client reads from
// GET /api/v1/print-jobs (ADR-081).
type PrintChangedMessage struct {
	Type    MessageType `json:"type"`
	Version int64       `json:"version"`
}

// FloorChangedMessage says the venue's floor has changed in a way its
// orders' events do not show: the plan edited, a table cleared. The client
// reads the floor again from GET /api/v1/floor/live (ADR-086). A change an
// order makes, seating or settling, arrives as that order's event.
type FloorChangedMessage struct {
	Type    MessageType `json:"type"`
	Version int64       `json:"version"`
}

// CaughtUpMessage says the client has everything up to a position.
type CaughtUpMessage struct {
	Type MessageType `json:"type"`
	ID   int64       `json:"id"`
}

// ErrorMessage says what the client sent could not be used.
type ErrorMessage struct {
	Type    MessageType `json:"type"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
}

// conn is one connected device.
type conn struct {
	hub    *Hub
	ws     *websocket.Conn
	client Client

	// woken has room for one nudge. More than one pending nudge says
	// nothing extra: the reader will go to the database and find whatever
	// is there.
	woken chan struct{}

	mu     sync.Mutex
	topics map[string]struct{}
	cursor int64

	// menu, print and floor are the versions this connection last told its
	// client about. Only the writing goroutine touches them.
	menu  int64
	print int64
	floor int64
}

// wake tells the connection there may be more to read, without blocking.
func (c *conn) wake() {
	select {
	case c.woken <- struct{}{}:
	default:
	}
}

// run reads the client's hello, then streams until something ends it.
func (c *conn) run(ctx context.Context) error {
	hello, err := c.readHello(ctx)
	if err != nil {
		return err
	}
	latest, err := c.hub.source.VenueLastEventID(ctx, c.client.VenueID)
	if err != nil {
		return err
	}
	c.menu, err = c.hub.source.MenuVersion(ctx, c.client.VenueID)
	if err != nil {
		return err
	}
	c.print, err = c.hub.source.PrintVersion(ctx, c.client.VenueID)
	if err != nil {
		return err
	}
	c.floor, err = c.hub.source.FloorVersion(ctx, c.client.VenueID)
	if err != nil {
		return err
	}

	from := hello.LastEventID
	switch {
	case hello.FromStart:
		from = 0
	case from <= 0:
		// No position: start from now, so a screen opening fresh is not
		// buried under the day's history.
		from = latest
	case from > latest:
		// A client remembering further than the database goes: a restored
		// backup, or another venue's id. Sending it nothing would leave it
		// silent forever, so it starts again from the top.
		c.hub.log.Warn("a device is ahead of the stream, replaying it whole",
			"venue_id", c.client.VenueID, "device_id", c.client.DeviceID, "client_at", from, "stream_at", latest)
		from = 0
	}
	c.setCursor(from)
	c.setTopics(hello.Topics)

	if err := c.hub.writeJSON(ctx, c.ws, WelcomeMessage{
		Type: MsgWelcome, VenueID: c.client.VenueID, DeviceID: c.client.DeviceID,
		From: from, Latest: latest, Topics: c.topicList(), MenuVersion: c.menu, PrintVersion: c.print,
		FloorVersion: c.floor,
	}); err != nil {
		return err
	}

	// One goroutine reads the client's control messages; this one writes.
	// The read loop is also what notices a dead connection, since a pong
	// arrives there.
	errs := make(chan error, 1)
	go func() { errs <- c.readLoop(ctx) }()

	ping := time.NewTicker(c.hub.pingEvery)
	defer ping.Stop()

	for {
		if err := c.drain(ctx); err != nil {
			return err
		}
		if err := c.checkVersions(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errs:
			return err
		case <-c.woken:
		case <-ping.C:
			// A ping doubles as a check that the socket is still there,
			// and the drain above covers a nudge that was missed.
			pingCtx, cancel := context.WithTimeout(ctx, c.hub.writeTimeout)
			err := c.ws.Ping(pingCtx)
			cancel()
			if err != nil {
				return fmt.Errorf("hub: pinging device %d: %w", c.client.DeviceID, err)
			}
		}
	}
}

// readHello waits for the first message, which must be a hello.
func (c *conn) readHello(ctx context.Context) (ClientMessage, error) {
	msg, err := c.read(ctx)
	if err != nil {
		return ClientMessage{}, err
	}
	if msg.Type != MsgHello {
		_ = c.hub.writeJSON(ctx, c.ws, ErrorMessage{Type: MsgError, Code: "expected_hello", Message: "say hello first"})
		return ClientMessage{}, fmt.Errorf("hub: first message was %q, not a hello", msg.Type)
	}
	return msg, nil
}

// readLoop handles everything the client says after the hello.
func (c *conn) readLoop(ctx context.Context) error {
	for {
		msg, err := c.read(ctx)
		if err != nil {
			return err
		}
		switch msg.Type {
		case MsgSubscribe:
			c.setTopics(msg.Topics)
			if err := c.hub.writeJSON(ctx, c.ws, WelcomeMessage{
				Type: MsgWelcome, VenueID: c.client.VenueID, DeviceID: c.client.DeviceID,
				From: c.at(), Latest: c.at(), Topics: c.topicList(),
			}); err != nil {
				return err
			}
		default:
			if err := c.hub.writeJSON(ctx, c.ws, ErrorMessage{
				Type: MsgError, Code: "unknown_message", Message: "this build does not understand " + string(msg.Type),
			}); err != nil {
				return err
			}
		}
	}
}

func (c *conn) read(ctx context.Context) (ClientMessage, error) {
	_, raw, err := c.ws.Read(ctx)
	if err != nil {
		return ClientMessage{}, err
	}
	var msg ClientMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		_ = c.hub.writeJSON(ctx, c.ws, ErrorMessage{Type: MsgError, Code: "invalid_message", Message: "that is not a message this endpoint takes"})
		return ClientMessage{}, fmt.Errorf("hub: decoding a client message: %w", err)
	}
	return msg, nil
}

// drain sends everything the venue has after this connection's position,
// in batches, until there is nothing left.
func (c *conn) drain(ctx context.Context) error {
	for {
		events, err := c.hub.source.VenueEventsAfter(ctx, c.client.VenueID, c.at(), c.hub.batch)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			return nil
		}
		for _, e := range events {
			if c.wants(e) {
				if err := c.hub.writeJSON(ctx, c.ws, toMessage(e)); err != nil {
					return err
				}
			}
			c.setCursor(e.ID)
		}
		if err := c.hub.writeJSON(ctx, c.ws, CaughtUpMessage{Type: MsgCaughtUp, ID: c.at()}); err != nil {
			return err
		}
		if len(events) < c.hub.batch {
			return nil
		}
	}
}

// checkVersions tells the client when the venue's menu, print jobs or floor
// have moved on. It runs after every drain, so like an event, a change whose
// nudge was missed is caught on the next ping.
func (c *conn) checkVersions(ctx context.Context) error {
	src := c.hub.source
	if err := c.checkVersion(ctx, &c.menu, src.MenuVersion, func(v int64) any {
		return MenuChangedMessage{Type: MsgMenuChanged, Version: v}
	}); err != nil {
		return err
	}
	if err := c.checkVersion(ctx, &c.print, src.PrintVersion, func(v int64) any {
		return PrintChangedMessage{Type: MsgPrintChanged, Version: v}
	}); err != nil {
		return err
	}
	return c.checkVersion(ctx, &c.floor, src.FloorVersion, func(v int64) any {
		return FloorChangedMessage{Type: MsgFloorChanged, Version: v}
	})
}

// checkVersion reads one of the venue's versions and, if it differs from the
// one the client was last told, tells it and remembers the new one in told.
func (c *conn) checkVersion(ctx context.Context, told *int64,
	read func(context.Context, int64) (int64, error), message func(int64) any) error {
	version, err := read(ctx, c.client.VenueID)
	if err != nil {
		return err
	}
	if version == *told {
		return nil
	}
	if err := c.hub.writeJSON(ctx, c.ws, message(version)); err != nil {
		return err
	}
	*told = version
	return nil
}

// wants reports whether this connection is subscribed to an event.
func (c *conn) wants(e db.StreamEvent) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, all := c.topics[TopicVenue]; all {
		return true
	}
	for _, t := range topicsOf(e) {
		if _, ok := c.topics[t]; ok {
			return true
		}
	}
	return false
}

// topicsOf is where an event belongs: its order, its table, and the station
// it names, if it names one.
func topicsOf(e db.StreamEvent) []string {
	topics := []string{prefixOrder + e.OrderPublicID}
	if e.TableID > 0 {
		topics = append(topics, prefixTable+strconv.FormatInt(e.TableID, 10))
	}
	if station := stationOf(e.Event); station != "" {
		topics = append(topics, prefixStation+station)
	}
	return topics
}

// stationOf is the station an event concerns. A ticket is sent to one and
// bumped and recalled there, and a line belongs to one; nothing else does.
func stationOf(e order.Event) string {
	switch p := e.Payload.(type) {
	case order.TicketSent:
		return p.Station
	case order.TicketBumped:
		return p.Station
	case order.TicketRecalled:
		return p.Station
	case order.LineAdded:
		return p.Station
	default:
		return ""
	}
}

func toMessage(e db.StreamEvent) EventMessage {
	return EventMessage{
		Type: MsgEvent, ID: e.ID, OrderID: e.OrderPublicID, Status: e.OrderStatus,
		TableID: e.TableID, Station: stationOf(e.Event), Event: e.Event,
	}
}

func (c *conn) at() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cursor
}

func (c *conn) setCursor(id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cursor = id
}

// setTopics replaces the subscription. An empty list means the venue topic,
// which is everything: a client that asks for nothing in particular gets
// the lot rather than silence.
func (c *conn) setTopics(topics []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.topics = map[string]struct{}{}
	for _, t := range topics {
		if t = strings.TrimSpace(t); t != "" {
			c.topics[t] = struct{}{}
		}
	}
	if len(c.topics) == 0 {
		c.topics[TopicVenue] = struct{}{}
	}
}

func (c *conn) topicList() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.topics))
	for t := range c.topics {
		out = append(out, t)
	}
	slices.Sort(out)
	return out
}
