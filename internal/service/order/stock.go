package order

import (
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/stock"
)

// closes reports whether a command's events close the order: the moment
// its stock is taken (ADR-112).
func closes(appended []order.Event) bool {
	for _, e := range appended {
		if e.Type == order.TypeOrderClosed {
			return true
		}
	}
	return false
}

// soldLines is what an order used from stock: every line not voided, with
// its modifiers. A comped line was made, so it counts; a voided one was
// not.
func soldLines(st order.State) []stock.SoldLine {
	out := make([]stock.SoldLine, 0, len(st.Lines))
	for _, l := range st.Lines {
		if l.Status == order.LineStatusVoided {
			continue
		}
		mods := make([]int64, len(l.Modifiers))
		for i, m := range l.Modifiers {
			mods[i] = m.ModifierID
		}
		out = append(out, stock.SoldLine{ItemID: l.ItemID, ModifierIDs: mods, Qty: l.Qty})
	}
	return out
}
