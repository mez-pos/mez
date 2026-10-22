package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/hours"
	"github.com/mez-pos/mez/internal/domain/shift"
	"github.com/mez-pos/mez/internal/domain/tips"
	svcorder "github.com/mez-pos/mez/internal/service/order"
	svcshift "github.com/mez-pos/mez/internal/service/shift"
)

// RulesInput is how the venue runs its shifts and shares its tips, and when a
// ticket counts as late.
type RulesInput struct {
	// DayStart is when the trading day begins, local time, "04:00". Empty
	// takes the default. A shift keeps the day it opened under: a change
	// reaches the next shift opened.
	DayStart string `json:"day_start"`
	// VarianceThreshold is how far over or short a drawer may be before
	// closing needs a reason, in minor units. 0 asks for a reason on any
	// variance.
	VarianceThreshold int64 `json:"variance_threshold"`
	// TipRule is even, hours or role_weight. Empty takes even.
	TipRule string `json:"tip_rule"`
	// TipRoleWeights is each role's weight under role_weight, keyed by role
	// name. A role not listed weighs 1.
	TipRoleWeights map[string]int64 `json:"tip_role_weights"`
	// TicketLateMinutes is how long an open ticket runs before the till shows
	// its time as late.
	TicketLateMinutes int `json:"ticket_late_minutes"`
	// TipsMaxPercent is the tip ceiling (ADR-261): a tip over this percent of
	// its payment needs a manager. 0 to 1000; 0 asks for a manager on every
	// tip. Left out, it takes the default, as an empty DayStart does: 0 is a
	// ceiling, so it cannot also mean "not given".
	TipsMaxPercent *int `json:"tips_max_percent,omitempty"`
	// OpeningHours is when the venue is open, by weekday, "mon" to "sun"
	// (ADR-360). A day not in it is a day the venue is closed; empty is not
	// set. The till does nothing with them: mez Cloud's alerts read them
	// from the venue's health summary (ADR-361).
	OpeningHours hours.Week `json:"opening_hours"`
}

// Rules is the rules as they stand, and what the screen needs to edit them.
type Rules struct {
	RulesInput
	Currency string
	// MinorPerMajor is how many minor units make one major unit of the
	// currency (100 for rupees), to show an amount the way it is spoken.
	MinorPerMajor int64
	// Roles are the venue's role names, for the weights: a weight names one.
	Roles []string
	// OpenShiftDay is the trading day of the shift now open, or "". That
	// shift keeps its day when DayStart changes.
	OpenShiftDay string
}

// Rules reads how the venue runs its shifts and tips. A key that is unset
// reads as its default, which is what everything reading it does.
func (s *Service) Rules(ctx context.Context, venueID int64) (Rules, error) {
	q := s.db.Queries()
	v, stored, err := rawVenue(ctx, q, venueID)
	if err != nil {
		return Rules{}, err
	}
	value := func(name string) (string, error) {
		k, _ := find(name)
		val := k.Default
		if raw, ok := stored[name]; ok {
			val = strings.TrimSpace(raw)
		}
		if val == "" {
			return "", nil
		}
		if msg := k.Check(val); msg != "" {
			return "", fmt.Errorf("venue %d's %s is %q: %s", venueID, name, val, msg)
		}
		return val, nil
	}
	var out Rules
	out.Currency = v.Currency
	out.MinorPerMajor = maxRoundingStep(v.Currency)
	var raw struct{ dayStart, variance, rule, weights, late, tipMax, opening string }
	for _, f := range []struct {
		into *string
		name string
	}{
		{&raw.dayStart, svcshift.SettingDayStart}, {&raw.variance, svcshift.SettingVarianceThreshold},
		{&raw.rule, svcshift.SettingTipRule}, {&raw.weights, svcshift.SettingTipRoleWeights},
		{&raw.late, svcorder.SettingTicketLateMinutes}, {&raw.tipMax, svcorder.SettingTipMaxPercent},
		{&raw.opening, svcshift.SettingOpeningHours},
	} {
		if *f.into, err = value(f.name); err != nil {
			return Rules{}, err
		}
	}
	out.DayStart, out.TipRule = raw.dayStart, raw.rule
	// The checks above have passed these, so a parse cannot fail.
	out.VarianceThreshold, _ = strconv.ParseInt(raw.variance, 10, 64)
	out.TicketLateMinutes, _ = strconv.Atoi(raw.late)
	tipMax, _ := strconv.Atoi(raw.tipMax)
	out.TipsMaxPercent = &tipMax
	out.TipRoleWeights = map[string]int64{}
	if raw.weights != "" {
		if err := json.Unmarshal([]byte(raw.weights), &out.TipRoleWeights); err != nil {
			return Rules{}, fmt.Errorf("venue %d's tip role weights: %w", venueID, err)
		}
	}
	// The check above has passed the text, so this cannot fail. Never nil:
	// the screen reads it as an object, and a venue with none set is {}.
	out.OpeningHours, _ = hours.Parse(raw.opening)
	if out.OpeningHours == nil {
		out.OpeningHours = hours.Week{}
	}
	if out.Roles, err = roleNames(ctx, q, venueID); err != nil {
		return Rules{}, err
	}
	switch open, err := q.OpenShiftForVenue(ctx, venueID); {
	case err == nil:
		out.OpenShiftDay = open.TradingDay
	case !errors.Is(err, sql.ErrNoRows):
		return Rules{}, fmt.Errorf("reading the open shift of venue %d: %w", venueID, err)
	}
	return out, nil
}

// roleNames is the venue's roles, by name.
func roleNames(ctx context.Context, q *gen.Queries, venueID int64) ([]string, error) {
	rows, err := q.RolesForVenue(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("reading the roles of venue %d: %w", venueID, err)
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return names, nil
}

// UpdateRules saves the shift and tip rules, or nothing. The values are
// checked together here, then saved through Set, which checks each alone and
// writes them in one transaction.
//
// A weight names a role, and a role can be removed between the check and the
// save: its weight is then kept and never used, as when a role is renamed.
func (s *Service) UpdateRules(ctx context.Context, venueID int64, in RulesInput) error {
	q := s.db.Queries()
	if _, err := q.VenueByID(ctx, venueID); errors.Is(err, sql.ErrNoRows) {
		return ErrNoVenue
	} else if err != nil {
		return fmt.Errorf("reading venue %d: %w", venueID, err)
	}
	roles, err := roleNames(ctx, q, venueID)
	if err != nil {
		return err
	}
	values, err := cleanRules(in, roles)
	if err != nil {
		return err
	}
	return s.Set(ctx, venueID, values)
}

// cleanRules checks the rules and says what to store. A value that is the
// default is not stored, so the key reads as unset and follows the default.
func cleanRules(in RulesInput, roles []string) (map[string]string, error) {
	in.DayStart = strings.TrimSpace(in.DayStart)
	in.TipRule = strings.TrimSpace(in.TipRule)
	if in.DayStart == "" {
		in.DayStart = shift.FormatDayStart(shift.DefaultDayStart)
	}
	if in.TipRule == "" {
		in.TipRule = string(tips.Even)
	}
	tipMax := strconv.Itoa(svcorder.DefaultTipMaxPercent)
	if in.TipsMaxPercent != nil {
		tipMax = strconv.Itoa(*in.TipsMaxPercent)
	}
	weights := ""
	if len(in.TipRoleWeights) > 0 {
		// A map marshals with its keys in order, so the same weights are
		// always stored as the same text.
		body, err := json.Marshal(in.TipRoleWeights)
		if err != nil {
			return nil, &Invalid{"tip_role_weights", "Those weights cannot be saved."}
		}
		weights = string(body)
	}

	opening, err := in.OpeningHours.Format()
	if err != nil {
		var e *hours.Error
		msg := "The hours are not written as a week of opening and closing times."
		if errors.As(err, &e) {
			msg = e.Message
		}
		return nil, &Invalid{"opening_hours", msg}
	}

	checks := []struct{ field, key, value string }{
		{"day_start", svcshift.SettingDayStart, in.DayStart},
		{"opening_hours", svcshift.SettingOpeningHours, opening},
		{"variance_threshold", svcshift.SettingVarianceThreshold, strconv.FormatInt(in.VarianceThreshold, 10)},
		{"tip_rule", svcshift.SettingTipRule, in.TipRule},
		{"tip_role_weights", svcshift.SettingTipRoleWeights, weights},
		{"ticket_late_minutes", svcorder.SettingTicketLateMinutes, strconv.Itoa(in.TicketLateMinutes)},
		{"tips_max_percent", svcorder.SettingTipMaxPercent, tipMax},
	}
	for _, c := range checks {
		if c.value == "" {
			continue
		}
		k, _ := find(c.key)
		if msg := k.Check(c.value); msg != "" {
			return nil, &Invalid{c.field, msg}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(in.TipRoleWeights)) {
		if !slices.Contains(roles, name) {
			return nil, &Invalid{"tip_role_weights", fmt.Sprintf("%q is not a role here. The roles are: %s.", name, strings.Join(roles, ", "))}
		}
	}
	if tips.Rule(in.TipRule) == tips.ByRoleWeight && len(in.TipRoleWeights) == 0 {
		return nil, &Invalid{"tip_role_weights", "Give at least one role a weight, or choose another tip rule."}
	}

	ifNot := func(value, fallback string) string {
		if value == fallback {
			return ""
		}
		return value
	}
	return map[string]string{
		svcshift.SettingDayStart:          ifNot(in.DayStart, shift.FormatDayStart(shift.DefaultDayStart)),
		svcshift.SettingVarianceThreshold: ifNot(strconv.FormatInt(in.VarianceThreshold, 10), strconv.FormatInt(svcshift.DefaultVarianceThreshold, 10)),
		svcshift.SettingTipRule:           ifNot(in.TipRule, string(tips.Even)),
		svcshift.SettingTipRoleWeights:    weights,
		svcshift.SettingOpeningHours:      opening,
		svcorder.SettingTicketLateMinutes: ifNot(strconv.Itoa(in.TicketLateMinutes), strconv.Itoa(svcorder.DefaultLateMinutes)),
		svcorder.SettingTipMaxPercent:     ifNot(tipMax, strconv.Itoa(svcorder.DefaultTipMaxPercent)),
	}, nil
}
