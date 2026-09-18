package invoice

import (
	"cmp"
	"slices"
)

// Audit checks the gapless guarantee over a set of issued invoices: for every
// Key, the numbers are exactly 1, 2, ..., n, each once. It returns the first
// fault as an *AuditError wrapping ErrGap or ErrDuplicate, checking keys and
// numbers in a fixed order so the same set always reports the same fault.
//
// Tests use it as the statement of the property. A Z-report or a tax export
// can run it over a day's or a year's invoices before trusting them.
func Audit(issued []Issued) error {
	byKey := map[Key][]int64{}
	for _, in := range issued {
		byKey[in.Key] = append(byKey[in.Key], in.Seq)
	}
	keys := make([]Key, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, compareKeys)

	for _, k := range keys {
		seqs := byKey[k]
		slices.Sort(seqs)
		want := int64(1)
		for _, got := range seqs {
			switch {
			case got < want:
				return &AuditError{Key: k, Seq: got, Err: ErrDuplicate}
			case got > want:
				return &AuditError{Key: k, Seq: want, Err: ErrGap}
			}
			want++
		}
	}
	return nil
}

func compareKeys(a, b Key) int {
	return cmp.Or(
		cmp.Compare(a.VenueID, b.VenueID),
		cmp.Compare(a.Series, b.Series),
		cmp.Compare(a.FYStart, b.FYStart),
	)
}
