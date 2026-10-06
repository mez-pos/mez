package settings

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"testing"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	svcshift "github.com/byjoellobo/mez/internal/service/shift"
)

// stockRules is what a venue that has set none of them reads as.
func stockRules() RulesInput {
	return RulesInput{
		DayStart: "04:00", VarianceThreshold: 0, TipRule: "even", TipRoleWeights: map[string]int64{}, TicketLateMinutes: 20,
		TipsMaxPercent: percent(50),
	}
}

func percent(n int) *int { return &n }

func rulesWith(f func(*RulesInput)) RulesInput {
	in := stockRules()
	f(&in)
	return in
}

// addRoles gives venue 1 roles of these names, and returns the first one's id.
func addRoles(t *testing.T, d *db.DB, names ...string) int64 {
	t.Helper()
	ctx := context.Background()
	var first int64
	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		for i, n := range names {
			r, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: 1, Name: n, Permissions: "[]", CreatedAt: now})
			if err != nil {
				return err
			}
			if i == 0 {
				first = r.ID
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return first
}

func TestRulesOfANewVenueAreTheDefaults(t *testing.T) {
	s, d := newService(t)
	addRoles(t, d, "Manager", "Cashier")
	got, err := s.Rules(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := Rules{RulesInput: stockRules(), Currency: "INR", MinorPerMajor: 100, Roles: []string{"Cashier", "Manager"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if _, err := s.Rules(context.Background(), 99); !errors.Is(err, ErrNoVenue) {
		t.Fatalf("an unknown venue: %v", err)
	}
}

// TestRuleDefaultsAreWhatTheReadersUse: the registry's defaults are the
// readers' own constants, so an unset key means one thing everywhere.
func TestRuleDefaultsAreWhatTheReadersUse(t *testing.T) {
	got := map[string]string{}
	for _, k := range keys {
		got[k.Name] = k.Default
	}
	want := map[string]string{
		svcshift.SettingDayStart:          "04:00",
		svcshift.SettingVarianceThreshold: "0",
		svcshift.SettingTipRule:           "even",
		"terminal.ticket_late_minutes":    "20",
		"tips.max_percent":                "50",
	}
	for name, def := range want {
		if got[name] != def {
			t.Errorf("%s defaults to %q, want %q", name, got[name], def)
		}
	}
}

func TestUpdateRulesSavesAndReadsBack(t *testing.T) {
	s, d := newService(t)
	addRoles(t, d, "Manager", "Cashier")
	ctx := context.Background()
	in := RulesInput{
		DayStart: "05:30", VarianceThreshold: 5000, TipRule: "role_weight",
		TipRoleWeights: map[string]int64{"Manager": 3, "Cashier": 1}, TicketLateMinutes: 12,
		// No tip at all without a manager: 0 is a ceiling, not "unset".
		TipsMaxPercent: percent(0),
	}
	if err := s.UpdateRules(ctx, 1, in); err != nil {
		t.Fatal(err)
	}
	got, err := s.Rules(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.RulesInput, in) {
		t.Fatalf("got  %+v\nwant %+v", got.RulesInput, in)
	}

	// The same weights are stored as the same text, whatever order they came in.
	o, err := s.Read(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	stored := map[string]string{}
	for _, e := range o.Entries {
		if e.Set {
			stored[e.Key] = e.Value
		}
	}
	wantStored := map[string]string{
		"shift.day_start": "05:30", "shift.variance_threshold_minor": "5000", "tips.rule": "role_weight",
		"tips.role_weights": `{"Cashier":1,"Manager":3}`, "terminal.ticket_late_minutes": "12",
		"tips.max_percent": "0",
	}
	if !maps.Equal(stored, wantStored) {
		t.Fatalf("stored %v, want %v", stored, wantStored)
	}
}

func TestUpdateRulesBackToDefaultsClearsTheKeys(t *testing.T) {
	s, d := newService(t)
	addRoles(t, d, "Manager")
	ctx := context.Background()
	if err := s.UpdateRules(ctx, 1, RulesInput{
		DayStart: "05:30", VarianceThreshold: 5000, TipRule: "role_weight",
		TipRoleWeights: map[string]int64{"Manager": 2}, TicketLateMinutes: 12, TipsMaxPercent: percent(200),
	}); err != nil {
		t.Fatal(err)
	}
	// Empty text takes the default, as the numbering form's does.
	if err := s.UpdateRules(ctx, 1, RulesInput{TicketLateMinutes: 20}); err != nil {
		t.Fatal(err)
	}
	o, err := s.Read(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range o.Entries {
		switch e.Key {
		case "shift.day_start", "shift.variance_threshold_minor", "tips.rule", "tips.role_weights", "terminal.ticket_late_minutes",
			"tips.max_percent":
			if e.Set {
				t.Errorf("%s is still set to %q", e.Key, e.Value)
			}
		}
	}
	got, err := s.Rules(ctx, 1)
	if err != nil || !reflect.DeepEqual(got.RulesInput, stockRules()) {
		t.Fatalf("back to defaults: %+v, %v", got.RulesInput, err)
	}
}

func TestUpdateRulesRefusals(t *testing.T) {
	tests := []struct {
		name  string
		in    RulesInput
		field string // "" is accepted
	}{
		{"a day start that is not a time", rulesWith(func(in *RulesInput) { in.DayStart = "dawn" }), "day_start"},
		{"a day start of 24:00", rulesWith(func(in *RulesInput) { in.DayStart = "24:00" }), "day_start"},
		{"a day start without its zero", rulesWith(func(in *RulesInput) { in.DayStart = "4:00" }), "day_start"},
		{"midnight", rulesWith(func(in *RulesInput) { in.DayStart = "00:00" }), ""},
		{"a negative threshold", rulesWith(func(in *RulesInput) { in.VarianceThreshold = -1 }), "variance_threshold"},
		{"a threshold past a million", rulesWith(func(in *RulesInput) { in.VarianceThreshold = 1_000_000_01 }), "variance_threshold"},
		{"the largest threshold", rulesWith(func(in *RulesInput) { in.VarianceThreshold = 1_000_000_00 }), ""},
		{"a tip rule that does not exist", rulesWith(func(in *RulesInput) { in.TipRule = "coin_toss" }), "tip_rule"},
		{"tips by hours", rulesWith(func(in *RulesInput) { in.TipRule = "hours" }), ""},
		{"a weight of 0", rulesWith(func(in *RulesInput) { in.TipRoleWeights = map[string]int64{"Manager": 0} }), "tip_role_weights"},
		{"a weight of 101", rulesWith(func(in *RulesInput) { in.TipRoleWeights = map[string]int64{"Manager": 101} }), "tip_role_weights"},
		{"a weight for a role that is not here", rulesWith(func(in *RulesInput) { in.TipRoleWeights = map[string]int64{"Barista": 2} }), "tip_role_weights"},
		{"a role in the wrong case", rulesWith(func(in *RulesInput) { in.TipRoleWeights = map[string]int64{"manager": 2} }), "tip_role_weights"},
		{"role weights with no weights", rulesWith(func(in *RulesInput) { in.TipRule = "role_weight" }), "tip_role_weights"},
		{"weights kept while tips are even", rulesWith(func(in *RulesInput) { in.TipRoleWeights = map[string]int64{"Manager": 2} }), ""},
		{"no minutes at all", rulesWith(func(in *RulesInput) { in.TicketLateMinutes = 0 }), "ticket_late_minutes"},
		{"601 minutes", rulesWith(func(in *RulesInput) { in.TicketLateMinutes = 601 }), "ticket_late_minutes"},
		{"a minute", rulesWith(func(in *RulesInput) { in.TicketLateMinutes = 1 }), ""},
		// The tip ceiling (ADR-261): 0 is no tip without a manager, 1000 ten times the payment.
		{"a tip ceiling of 0", rulesWith(func(in *RulesInput) { in.TipsMaxPercent = percent(0) }), ""},
		{"a tip ceiling of 1000", rulesWith(func(in *RulesInput) { in.TipsMaxPercent = percent(1000) }), ""},
		{"a tip ceiling left out takes the default", rulesWith(func(in *RulesInput) { in.TipsMaxPercent = nil }), ""},
		{"a negative tip ceiling", rulesWith(func(in *RulesInput) { in.TipsMaxPercent = percent(-1) }), "tips_max_percent"},
		{"a tip ceiling of 1001", rulesWith(func(in *RulesInput) { in.TipsMaxPercent = percent(1001) }), "tips_max_percent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, d := newService(t)
			addRoles(t, d, "Manager", "Cashier")
			ctx := context.Background()
			err := s.UpdateRules(ctx, 1, tt.in)
			var inv *Invalid
			switch {
			case tt.field == "" && err != nil:
				t.Fatal(err)
			case tt.field != "" && (!errors.As(err, &inv) || inv.Field != tt.field):
				t.Fatalf("want a refusal of %q, got %v", tt.field, err)
			}
			if tt.field == "" {
				return
			}
			// Nothing was saved.
			got, err := s.Rules(ctx, 1)
			if err != nil || !reflect.DeepEqual(got.RulesInput, stockRules()) {
				t.Fatalf("after the refusal: %+v, %v", got.RulesInput, err)
			}
		})
	}
}

func TestUpdateRulesIsAllOrNothing(t *testing.T) {
	s, d := newService(t)
	addRoles(t, d, "Manager")
	ctx := context.Background()
	// The day start and threshold are fine; the weight names no role.
	err := s.UpdateRules(ctx, 1, RulesInput{
		DayStart: "06:00", VarianceThreshold: 100, TipRule: "role_weight",
		TipRoleWeights: map[string]int64{"Chef": 2}, TicketLateMinutes: 10,
	})
	if err == nil {
		t.Fatal("want a refusal")
	}
	o, _ := s.Read(ctx, 1)
	for _, e := range o.Entries {
		if e.Set {
			t.Errorf("%s was saved by a refused update: %q", e.Key, e.Value)
		}
	}
	if err := s.UpdateRules(ctx, 99, stockRules()); !errors.Is(err, ErrNoVenue) {
		t.Fatalf("an unknown venue: %v", err)
	}
}

func TestRulesNameTheOpenShiftsDay(t *testing.T) {
	s, d := newService(t)
	ctx := context.Background()
	got, err := s.Rules(ctx, 1)
	if err != nil || got.OpenShiftDay != "" {
		t.Fatalf("no shift open: %q, %v", got.OpenShiftDay, err)
	}
	role := addRoles(t, d, "Manager")
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		st, err := q.CreateStaff(ctx, gen.CreateStaffParams{
			PublicID: "stf_RULES001", VenueID: 1, RoleID: role, Name: "Priya", PinHash: "x", Active: true, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		_, err = q.CreateShift(ctx, gen.CreateShiftParams{
			PublicID: "shf_RULES001", VenueID: 1, TradingDay: "2026-10-01", OpenedBy: st.ID, OpenedAt: now,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, err = s.Rules(ctx, 1); err != nil || got.OpenShiftDay != "2026-10-01" {
		t.Fatalf("a shift open: %q, %v", got.OpenShiftDay, err)
	}
	// Changing the day start does not touch the open shift's day.
	if err := s.UpdateRules(ctx, 1, rulesWith(func(in *RulesInput) { in.DayStart = "07:00" })); err != nil {
		t.Fatal(err)
	}
	if got, err = s.Rules(ctx, 1); err != nil || got.OpenShiftDay != "2026-10-01" || got.DayStart != "07:00" {
		t.Fatalf("after the change: %+v, %v", got, err)
	}
}

func TestRulesRefuseAStoredValueNothingCouldRead(t *testing.T) {
	s, d := newService(t)
	ctx := context.Background()
	if err := d.WriteTx(ctx, func(q *gen.Queries) error {
		return q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: svcshift.SettingDayStart, Value: "dawn", UpdatedAt: now})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rules(ctx, 1); err == nil {
		t.Fatal("a day start of dawn read as fine")
	}
	// It can still be fixed.
	if err := s.UpdateRules(ctx, 1, stockRules()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rules(ctx, 1); err != nil {
		t.Fatal(err)
	}
}
