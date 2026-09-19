// Package auth decides who is at the other end of a request.
//
// A device pairs once: the back office makes a short code, the device redeems
// it for a long-lived token, and every request after that carries the token.
// Staff then sign in with a PIN on a paired device (phase 04, task 3b), and
// what they may do is a permission check (task 3c).
//
// Secrets are never stored. A device token is 256 random bits, kept only as
// its SHA-256; a pairing code likewise. PINs, which are short, get Argon2id.
//
// Imports: internal/db and internal/domain. Never internal/api.
package auth
