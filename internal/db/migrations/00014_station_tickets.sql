-- +goose Up
-- Every ticket sent to a station and where it is there (phase 08, Task 4,
-- ADR-090): fired while the kitchen makes it, ready once bumped. An index
-- over the projection, kept in step by the transaction that appends the
-- events. The kitchen display reads a station's tickets from here, open
-- orders and closed alike: a cafe paid at the counter closes the bill
-- before the coffee is made.
CREATE TABLE station_tickets (
    order_id   INTEGER NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    ticket_seq INTEGER NOT NULL,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE RESTRICT,
    station    TEXT    NOT NULL,
    status     TEXT    NOT NULL,
    sent_at    TIMESTAMP NOT NULL,
    ready_at   TIMESTAMP,
    PRIMARY KEY (order_id, ticket_seq),
    CHECK (length(station) > 0),
    CHECK (status IN ('fired', 'ready')),
    CHECK ((status = 'ready') = (ready_at IS NOT NULL)),
    CHECK (sent_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (ready_at IS NULL OR ready_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
) WITHOUT ROWID;

CREATE INDEX station_tickets_board ON station_tickets (venue_id, station, status, sent_at);

-- No backfill. A ticket sent before the display existed is not on any
-- board: a display must not open on a day of old orders, and a row that
-- called them ready would disagree with their orders, which say fired. A
-- bump or recall on one later writes its row as any ticket's is written.

-- +goose Down
DROP INDEX station_tickets_board;
DROP TABLE station_tickets;
