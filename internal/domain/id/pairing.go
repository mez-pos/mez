package id

// PairingCodeLength is how many symbols a device pairing code has. Forty
// bits: typed once, by a person reading it off the back office screen, and
// alive for minutes, which is far too short to guess one online.
const PairingCodeLength = 8

// NewPairingCode returns a pairing code from crypto/rand.
func NewPairingCode() (string, error) {
	return NewGenerator(nil).PairingCode()
}

// PairingCode draws a pairing code from this generator's entropy source.
// It is not checked against the blocklist: nobody calls it out across a
// room, and eight symbols spell little.
func (g *Generator) PairingCode() (string, error) {
	return g.symbols(PairingCodeLength)
}
