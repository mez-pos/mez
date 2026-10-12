-- +goose Up
-- A sign-in from away is told apart from one at the venue (ADR-314).
--
-- via is how the request arrived: '' on the venue's own network, 'relay'
-- through the relay. On an attempt it decides which count a wrong PIN joins:
-- the counter's window, or the three strikes from away. On a session it says
-- which rules it was opened under: a relayed request is served only on a
-- session opened through the relay.
--
-- Every row there is came from the venue's network or from before this was
-- recorded, and reads as the venue's. A session open through the relay when
-- this runs is asked for its PIN once more.

ALTER TABLE sign_in_attempts ADD COLUMN via TEXT NOT NULL DEFAULT ''
    CHECK (via IN ('', 'relay'));

ALTER TABLE staff_sessions ADD COLUMN via TEXT NOT NULL DEFAULT ''
    CHECK (via IN ('', 'relay'));

-- +goose Down
ALTER TABLE staff_sessions DROP COLUMN via;
ALTER TABLE sign_in_attempts DROP COLUMN via;
