package shift

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/shift"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tips"
)

// The tip pool's refusals (ADR-101).
var (
	ErrAlreadyPooled = errors.New("the shift's tips are already pooled")
	ErrNotStaff      = errors.New("not a staff member at this venue")
)

// The venue settings a pool reads. Unset, the rule is even and every role
// weighs 1.
const (
	SettingTipRule        = "tips.rule"         // even, hours or role_weight
	SettingTipRoleWeights = "tips.role_weights" // JSON: {"manager": 2, "cashier": 1}
)

// TipMember is one staff member's line of the tip report: what they took
// on their own tenders, and their share of the pool.
type TipMember struct {
	StaffID  int64        `json:"id"`
	PublicID string       `json:"staff_id"`
	Name     string       `json:"name"`
	Role     string       `json:"role"`
	InPool   bool         `json:"in_pool"`
	Minutes  int64        `json:"minutes"`
	Weight   int64        `json:"weight"`
	Taken    money.Amount `json:"taken"`
	Share    money.Amount `json:"share"`
}

// TipPool is a shift's tips: the pool, the rule, and each person. Before
// it is pooled it is a preview: the members suggested are everyone who
// took a tip or worked an order in the shift, shared by the rule as far as
// the rule can go without hours.
type TipPool struct {
	ShiftID  int64        `json:"shift_id"`
	Pooled   bool         `json:"pooled"`
	Rule     tips.Rule    `json:"rule"`
	Total    money.Amount `json:"total"`
	Members  []TipMember  `json:"members"`
	PooledBy int64        `json:"pooled_by,omitempty"`
	PooledAt time.Time    `json:"pooled_at,omitempty"`
	// Problem says why the preview has no shares yet: "hours" needs them.
	Problem string `json:"problem,omitempty"`
}

// PoolMember is someone put in the pool, by their public staff ID, and
// their time.
type PoolMember struct {
	Staff   string `json:"staff_id"`
	Minutes int64  `json:"minutes"`
}

// Tips is a shift's tip pool: as pooled, or a preview.
func (s *Service) Tips(ctx context.Context, venueID, shiftID int64) (TipPool, error) {
	q := s.db.Queries()
	row, err := shiftAt(ctx, q, venueID, shiftID)
	if err != nil {
		return TipPool{}, err
	}
	if stored, ok, err := storedPool(ctx, q, row.ID); err != nil || ok {
		return stored, err
	}
	in, err := tipInputs(ctx, q, venueID, row.ID)
	if err != nil {
		return TipPool{}, err
	}
	var suggested []PoolMember
	for _, id := range in.worked {
		suggested = append(suggested, PoolMember{Staff: in.staff[id].PublicID})
	}
	pool, err := in.pool(row.ID, suggested)
	if errors.Is(err, tips.ErrInvalidPool) {
		// Hours not yet entered: show the people and what they took.
		pool, _ = in.pool(row.ID, nil)
		pool.Members = in.members(suggested, nil)
		pool.Problem = strings.TrimPrefix(err.Error(), tips.ErrInvalidPool.Error()+": ")
		return pool, nil
	}
	return pool, err
}

// PoolTips splits a closed shift's tips among the members by the venue's
// rule, and records it. It is done once; the same pool retried by the same
// person gets it back, anything else is ErrAlreadyPooled.
func (s *Service) PoolTips(ctx context.Context, meta Meta, shiftID int64, members []PoolMember) (TipPool, error) {
	if err := meta.validate(); err != nil {
		return TipPool{}, err
	}
	now := s.clock.Now().UTC()
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		row, err := shiftAt(ctx, q, meta.VenueID, shiftID)
		if err != nil {
			return err
		}
		if shift.Status(row.Status) != shift.StatusClosed {
			return fmt.Errorf("%w: %s", ErrShiftStillOpen, row.PublicID)
		}
		in, err := tipInputs(ctx, q, meta.VenueID, row.ID)
		if err != nil {
			return err
		}
		if stored, ok, err := storedPool(ctx, q, row.ID); err != nil {
			return err
		} else if ok {
			if stored.PooledBy == meta.ActorID && samePool(stored, members) {
				return nil
			}
			return fmt.Errorf("%w: %s", ErrAlreadyPooled, row.PublicID)
		}
		pool, err := in.pool(row.ID, members)
		if err != nil {
			return err
		}
		pool.Pooled, pool.PooledBy, pool.PooledAt = true, meta.ActorID, now
		body, err := json.Marshal(pool)
		if err != nil {
			return fmt.Errorf("encoding the tip pool: %w", err)
		}
		return q.CreateTipPool(ctx, gen.CreateTipPoolParams{
			ShiftID: row.ID, Rule: string(pool.Rule), Total: pool.Total.Minor, Body: string(body), PooledBy: meta.ActorID, PooledAt: now,
		})
	})
	if err != nil {
		return TipPool{}, fmt.Errorf("pooling the tips: %w", err)
	}
	s.onChange(meta.VenueID)
	return s.Tips(ctx, meta.VenueID, shiftID)
}

// tipInputs is what a pool is made from: the tips taken, the venue's rule
// and weights, and who's who.
type tipInput struct {
	currency string
	rule     tips.Rule
	weights  map[string]int64
	taken    map[int64]money.Amount
	staff    map[int64]gen.StaffRolesForVenueRow
	byPublic map[string]int64
	worked   []int64 // who took a tip or worked an order in the shift, by ID
}

func tipInputs(ctx context.Context, q *gen.Queries, venueID, shiftID int64) (tipInput, error) {
	v, err := venueOf(ctx, q, venueID)
	if err != nil {
		return tipInput{}, err
	}
	in := tipInput{currency: v.currency, staff: map[int64]gen.StaffRolesForVenueRow{}, byPublic: map[string]int64{}}
	if in.rule, in.weights, err = tipSettings(ctx, q, venueID); err != nil {
		return tipInput{}, err
	}
	orders, err := stampedOrders(ctx, q, shiftID)
	if err != nil {
		return tipInput{}, err
	}
	if in.taken, err = tips.Taken(v.currency, shiftID, orders); err != nil {
		return tipInput{}, err
	}
	rows, err := q.StaffRolesForVenue(ctx, venueID)
	if err != nil {
		return tipInput{}, fmt.Errorf("reading the staff: %w", err)
	}
	for _, r := range rows {
		in.staff[r.ID] = r
		in.byPublic[r.PublicID] = r.ID
	}
	for id := range in.taken {
		in.worked = append(in.worked, id)
	}
	for _, events := range orders {
		for _, se := range events {
			if se.ShiftID == shiftID && se.Event.ActorID > 0 {
				in.worked = append(in.worked, se.Event.ActorID)
			}
		}
	}
	slices.Sort(in.worked)
	in.worked = slices.Compact(in.worked)
	in.worked = slices.DeleteFunc(in.worked, func(id int64) bool { _, ok := in.staff[id]; return !ok })
	return in, nil
}

// pool splits the tips taken among members. Everyone who took a tip is on
// the report, in the pool or not.
func (in tipInput) pool(shiftID int64, members []PoolMember) (TipPool, error) {
	total, err := money.New(0, in.currency)
	if err != nil {
		return TipPool{}, err
	}
	for _, t := range in.taken {
		if total, err = total.Add(t); err != nil {
			return TipPool{}, err
		}
	}
	out := TipPool{ShiftID: shiftID, Rule: in.rule, Total: total}
	if members == nil {
		return out, nil
	}
	split := make([]tips.Member, len(members))
	for i, m := range members {
		id, ok := in.byPublic[m.Staff]
		if !ok {
			return TipPool{}, fmt.Errorf("%w: %q", ErrNotStaff, m.Staff)
		}
		split[i] = tips.Member{StaffID: id, Minutes: m.Minutes, Weight: in.weightOf(in.staff[id].RoleName)}
	}
	shares, err := tips.Split(total, in.rule, split)
	if err != nil {
		return TipPool{}, err
	}
	out.Members = in.members(members, shares)
	return out, nil
}

// members is the report's lines: the pool's members with their shares,
// then anyone who took a tip and is not in it.
func (in tipInput) members(members []PoolMember, shares []tips.Share) []TipMember {
	zero := money.Amount{Currency: in.currency}
	line := func(id, minutes int64, pooled bool) TipMember {
		r := in.staff[id]
		taken, ok := in.taken[id]
		if !ok {
			taken = zero
		}
		return TipMember{
			StaffID: id, PublicID: r.PublicID, Name: r.Name, Role: r.RoleName, InPool: pooled, Minutes: minutes,
			Weight: in.weightOf(r.RoleName), Taken: taken, Share: zero,
		}
	}
	out := []TipMember{}
	for i, m := range members {
		l := line(in.byPublic[m.Staff], m.Minutes, true)
		if i < len(shares) {
			l.Share = shares[i].Share
		}
		out = append(out, l)
	}
	for _, id := range slices.Sorted(maps.Keys(in.taken)) {
		if !slices.ContainsFunc(members, func(m PoolMember) bool { return in.byPublic[m.Staff] == id }) {
			out = append(out, line(id, 0, false))
		}
	}
	return out
}

func (in tipInput) weightOf(role string) int64 {
	if w, ok := in.weights[role]; ok {
		return w
	}
	return 1
}

func tipSettings(ctx context.Context, q *gen.Queries, venueID int64) (tips.Rule, map[string]int64, error) {
	rule := tips.Even
	row, err := q.Setting(ctx, gen.SettingParams{VenueID: venueID, Key: SettingTipRule})
	switch {
	case err == nil:
		rule = tips.Rule(strings.TrimSpace(row.Value))
		if !slices.Contains(tips.Rules(), rule) {
			return "", nil, fmt.Errorf("%w: %s is %q, not one of %v", ErrVenueConfig, SettingTipRule, row.Value, tips.Rules())
		}
	case !errors.Is(err, sql.ErrNoRows):
		return "", nil, fmt.Errorf("reading %s: %w", SettingTipRule, err)
	}
	weights := map[string]int64{}
	row, err = q.Setting(ctx, gen.SettingParams{VenueID: venueID, Key: SettingTipRoleWeights})
	switch {
	case err == nil:
		if err := json.Unmarshal([]byte(row.Value), &weights); err != nil {
			return "", nil, fmt.Errorf("%w: %s is not a JSON object of role weights: %v", ErrVenueConfig, SettingTipRoleWeights, err)
		}
		for role, w := range weights {
			if w < 1 || w > 100 {
				return "", nil, fmt.Errorf("%w: %s gives %q a weight of %d, not 1 to 100", ErrVenueConfig, SettingTipRoleWeights, role, w)
			}
		}
	case !errors.Is(err, sql.ErrNoRows):
		return "", nil, fmt.Errorf("reading %s: %w", SettingTipRoleWeights, err)
	}
	return rule, weights, nil
}

func storedPool(ctx context.Context, q *gen.Queries, shiftID int64) (TipPool, bool, error) {
	row, err := q.TipPool(ctx, shiftID)
	if errors.Is(err, sql.ErrNoRows) {
		return TipPool{}, false, nil
	}
	if err != nil {
		return TipPool{}, false, fmt.Errorf("reading shift %d's tip pool: %w", shiftID, err)
	}
	var p TipPool
	if err := json.Unmarshal([]byte(row.Body), &p); err != nil {
		return TipPool{}, false, fmt.Errorf("decoding shift %d's tip pool: %w", shiftID, err)
	}
	return p, true, nil
}

// samePool reports whether members are the stored pool's, in order.
func samePool(p TipPool, members []PoolMember) bool {
	var in []PoolMember
	for _, m := range p.Members {
		if m.InPool {
			in = append(in, PoolMember{Staff: m.PublicID, Minutes: m.Minutes})
		}
	}
	return slices.Equal(in, members)
}
