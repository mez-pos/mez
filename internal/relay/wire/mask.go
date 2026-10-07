package wire

import "strings"

// masked is what a code is written as in a log.
const masked = "[code]"

// MaskCodes is a guest's path as a log may carry it. The segment after t,
// c or p is a table's, a counter's or a pickup code, and a code is the
// guest's only credential (ADR-205): whoever reads the log must not come
// away holding one. It reads the path by its shape alone, so it is the same
// rule for a venue at its own hostname (/order/api/t/<code>/...) and for one
// under a prefix (/v/<venue>/t/<code>/...), and for a mistyped path with a
// real code in it. A path with no such segment comes back unchanged.
func MaskCodes(path string) string {
	segs := strings.Split(path, "/")
	changed := false
	for i := 0; i+1 < len(segs); i++ {
		if (segs[i] == "t" || segs[i] == "c" || segs[i] == "p") && segs[i+1] != "" {
			segs[i+1] = masked
			changed = true
			i++ // the code itself is never read as a marker
		}
	}
	if !changed {
		return path
	}
	return strings.Join(segs, "/")
}
