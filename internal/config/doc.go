// Package config loads server configuration from the environment and an
// optional file.
//
// Defaults must be complete enough that the binary boots on a bare machine with
// no configuration present at all.
//
// Imports: standard library only. Nothing from internal/. Every other package
// may import this one, so it must stay a leaf.
package config
