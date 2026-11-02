package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNoProof is a device this server holds no pairing of to prove: never
// paired here, revoked, or not a device at all.
var ErrNoProof = errors.New("auth: no pairing to prove")

// Prove answers a tablet asking whether this server holds the pairing of the
// device publicID names, before it carries that pairing to this address
// (ADR-417): HMAC-SHA256 over message, keyed by what is stored of the
// device's token, its SHA-256 in hex. The tablet makes the same from the
// token it holds. The key never leaves this package, and what is answered
// does not give it away.
func (d *Devices) Prove(ctx context.Context, publicID string, message []byte) ([]byte, error) {
	if publicID == "" {
		return nil, ErrNoProof
	}
	row, err := d.db.Queries().DeviceProofKey(ctx, publicID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoProof
	}
	if err != nil {
		return nil, fmt.Errorf("auth: finding a device to prove: %w", err)
	}
	if row.RevokedAt.Valid || row.TokenHash == "" {
		return nil, ErrNoProof
	}
	mac := hmac.New(sha256.New, []byte(row.TokenHash))
	mac.Write(message)
	return mac.Sum(nil), nil
}
