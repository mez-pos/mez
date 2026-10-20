// Package firstrun makes the venue on a machine that has none (phase 12,
// Task 3a, ADR-266): the venue, its default roles, its owner and the code
// that pairs the first device. `mez setup` and POST /api/v1/setup both end
// here, so a venue made in a terminal and one made by the wizard cannot
// differ.
//
// It decides nothing about who may ask. The command is trusted because it
// is run at the machine; the API lets in only a browser on the machine.
package firstrun

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/id"
	svcorder "github.com/mez-pos/mez/internal/service/order"
	"github.com/mez-pos/mez/internal/service/settings"
)

var (
	// ErrAlreadySetUp means the database already has a venue. Setting up
	// twice would leave two venues on a machine that serves one.
	ErrAlreadySetUp = errors.New("this database is already set up")

	// ErrNoCode means the venue was made and its first pairing code was not.
	// Saying so is better than implying nothing happened, because a second
	// run will now refuse: `mez pair` at the machine makes the code.
	ErrNoCode = errors.New("the venue was created but its first pairing code was not")
)

// Invalid is an answer that cannot be used, and the field it is about. The
// fields are the API's: name, currency, timezone, fy_start_month,
// tax_profile, tax_registration, tax_pricing, owner_name, owner_pin,
// device_name, device_kind.
type Invalid struct {
	Field   string
	Message string
}

func (e *Invalid) Error() string { return e.Message }

// The tax profiles a venue can start with, and the two ways it quotes its
// prices. They are the settings service's own, so that what first run
// writes is what the back office's tax section reads.
const (
	TaxNone       = settings.ProfileNone
	TaxGSTIndia   = settings.ProfileGST
	TaxVATUK      = settings.ProfileVAT
	TaxSalesTaxUS = settings.ProfileSales

	PricingInclusive = settings.Inclusive
	PricingExclusive = settings.Exclusive
)

// gstinLength is how long a GSTIN is. Its first two digits are the state.
const gstinLength = 15

// The longest names first run takes. The venue's are the settings
// service's own limits, so a name the wizard took can be saved again in
// the back office.
const (
	maxVenueName    = 60
	maxLegalName    = 120
	maxOwnerName    = 60
	maxRegistration = 40
)

// printable reports whether s has no control characters in it. Every name
// here is printed on receipts, and a receipt printer reads ESC and GS as
// commands: a name carrying one could open the drawer.
func printable(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsFunc(s, unicode.IsControl)
}

// validGSTIN reports whether s has a GSTIN's shape: fifteen capital
// letters and digits, the first two the state's digits.
func validGSTIN(s string) bool {
	if len(s) != gstinLength || s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' {
		return false
	}
	return strings.Trim(s, "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ") == ""
}

// Answers is everything first run asks.
type Answers struct {
	VenueName string
	// LegalName is the registered name for invoices; empty means VenueName.
	LegalName string
	Currency  string
	Timezone  string
	// FYStartMonth is 1 to 12: 4 in India, 1 in much of the world.
	FYStartMonth    int
	TaxProfile      string
	TaxRegistration string
	// TaxPricing is inclusive, exclusive, or empty for the profile's own.
	TaxPricing string

	OwnerName string
	OwnerPIN  string

	DeviceName string
	DeviceKind auth.DeviceKind
}

// Result is what first run made.
type Result struct {
	VenueID       int64
	VenuePublicID string
	OwnerID       int64
	OwnerPublicID string
	Code          auth.PairingCode
}

// Extra writes more into the venue inside first run's transaction, so that
// what it adds exists with the venue or not at all. roles is the default
// roles' IDs by name. The demo cafe is one.
type Extra func(ctx context.Context, q *gen.Queries, venueID int64, roles map[string]int64, now time.Time) error

// Codes is what first run needs of device pairing. *auth.Devices is it.
type Codes interface {
	CreatePairingCode(ctx context.Context, n auth.NewCode) (auth.PairingCode, error)
}

// Clock is where the service gets the time.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Service is first run.
type Service struct {
	db    *db.DB
	codes Codes
	clock Clock
}

// New returns a Service.
func New(d *db.DB, codes Codes, clock Clock) (*Service, error) {
	if d == nil || codes == nil {
		return nil, errors.New("firstrun: a database and device pairing are required")
	}
	if clock == nil {
		clock = systemClock{}
	}
	return &Service{db: d, codes: codes, clock: clock}, nil
}

// Needed reports whether the venue is still to be made.
func (s *Service) Needed(ctx context.Context) (bool, error) {
	venues, err := s.db.Queries().ListVenues(ctx)
	if err != nil {
		return false, fmt.Errorf("firstrun: reading the venues: %w", err)
	}
	return len(venues) == 0, nil
}

// Run makes the venue, its roles, its owner and the first pairing code.
//
// Everything but the code is one transaction, and the check that no venue
// exists is inside it, so two of these racing cannot both win. The same
// check is made once before, so that a machine already set up says so
// without first judging answers it will not use.
func (s *Service) Run(ctx context.Context, answers Answers, extras ...Extra) (Result, error) {
	needed, err := s.Needed(ctx)
	if err != nil {
		return Result{}, err
	}
	if !needed {
		return Result{}, ErrAlreadySetUp
	}
	a, err := Check(answers)
	if err != nil {
		return Result{}, err
	}
	pinHash, err := auth.HashPIN(a.OwnerPIN)
	if err != nil {
		return Result{}, fmt.Errorf("firstrun: the owner's PIN: %w", err)
	}

	var res Result
	if res.VenuePublicID, err = id.NewPublic("ven"); err != nil {
		return Result{}, fmt.Errorf("firstrun: %w", err)
	}
	if res.OwnerPublicID, err = id.NewPublic("stf"); err != nil {
		return Result{}, fmt.Errorf("firstrun: %w", err)
	}

	now := s.clock.Now().UTC()
	err = s.db.WriteTx(ctx, func(q *gen.Queries) error {
		existing, err := q.ListVenues(ctx)
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			return fmt.Errorf("%w: it has %d venue(s) already", ErrAlreadySetUp, len(existing))
		}
		venue, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID:                res.VenuePublicID,
			Name:                    a.VenueName,
			LegalName:               a.LegalName,
			Currency:                a.Currency,
			Timezone:                a.Timezone,
			FinancialYearStartMonth: int64(a.FYStartMonth),
			TaxProfile:              a.TaxProfile,
			TaxRegistrationID:       a.TaxRegistration,
			CashRoundingStepMinor:   1,
			CreatedAt:               now,
			UpdatedAt:               now,
		})
		if err != nil {
			return err
		}
		res.VenueID = venue.ID

		roles, err := createRoles(ctx, q, venue.ID, now)
		if err != nil {
			return err
		}
		owner, err := q.CreateStaff(ctx, gen.CreateStaffParams{
			PublicID: res.OwnerPublicID, VenueID: venue.ID, RoleID: roles[ownerRole],
			Name: a.OwnerName, PinHash: pinHash, Active: true, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		res.OwnerID = owner.ID
		if err := writeSettings(ctx, q, venue.ID, a, now); err != nil {
			return err
		}
		for _, extra := range extras {
			if err := extra(ctx, q, venue.ID, roles, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}

	res.Code, err = s.codes.CreatePairingCode(ctx, auth.NewCode{
		VenueID: res.VenueID, CreatedBy: res.OwnerID, DeviceName: a.DeviceName, DeviceKind: a.DeviceKind,
	})
	if err != nil {
		return res, fmt.Errorf("%w: %w", ErrNoCode, err)
	}
	return res, nil
}

// Check judges the answers and returns them as they will be written:
// trimmed, the currency in capitals, the timezone by its own name, the
// legal name filled in. A refusal is an *Invalid naming the field.
func Check(a Answers) (Answers, error) {
	fail := func(field, format string, args ...any) (Answers, error) {
		return Answers{}, &Invalid{Field: field, Message: fmt.Sprintf(format, args...)}
	}

	a.VenueName = strings.TrimSpace(a.VenueName)
	if a.VenueName == "" {
		return fail("name", "the venue needs a name")
	}
	if utf8.RuneCountInString(a.VenueName) > maxVenueName || !printable(a.VenueName) {
		return fail("name", "the venue's name is up to %d characters of plain text", maxVenueName)
	}
	if a.LegalName = strings.TrimSpace(a.LegalName); a.LegalName == "" {
		a.LegalName = a.VenueName
	}
	if utf8.RuneCountInString(a.LegalName) > maxLegalName || !printable(a.LegalName) {
		return fail("legal_name", "the legal name is up to %d characters of plain text", maxLegalName)
	}
	currency := strings.ToUpper(strings.TrimSpace(a.Currency))
	if len(currency) != 3 || strings.Trim(currency, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
		return fail("currency", "currency %q is not a three letter code", a.Currency)
	}
	a.Currency = currency

	a.Timezone = strings.TrimSpace(a.Timezone)
	loc, err := time.LoadLocation(a.Timezone)
	if err != nil {
		return fail("timezone", "timezone %q: %v", a.Timezone, err)
	}
	if a.Timezone == "" || loc.String() == "Local" {
		// time.Local prints as "Local", which is not a zone: stored, the
		// venue's days and invoice times would follow whichever machine or
		// container ran the binary, and a browser cannot interpret it.
		return fail("timezone", "name an IANA timezone such as Asia/Kolkata: the machine's own zone is not a setting a venue can keep")
	}
	a.Timezone = loc.String()

	if a.FYStartMonth < 1 || a.FYStartMonth > 12 {
		return fail("fy_start_month", "the financial year cannot start in month %d", a.FYStartMonth)
	}

	// Trimmed once, here, so every check and every write sees the same
	// value: a GSTIN with a leading space must not pass a length check and
	// then yield a state code of " 2".
	a.TaxRegistration = strings.TrimSpace(a.TaxRegistration)
	if utf8.RuneCountInString(a.TaxRegistration) > maxRegistration || !printable(a.TaxRegistration) {
		return fail("tax_registration", "a tax registration is up to %d characters of plain text", maxRegistration)
	}
	switch a.TaxProfile {
	case TaxNone:
	case TaxGSTIndia:
		// The state code is cut from it, so its shape is held to here.
		a.TaxRegistration = strings.ToUpper(a.TaxRegistration)
		if !validGSTIN(a.TaxRegistration) {
			return fail("tax_registration", "GST needs the venue's %d character GSTIN, which begins with the state's two digits", gstinLength)
		}
	case TaxVATUK:
		if a.TaxRegistration == "" {
			return fail("tax_registration", "VAT needs the venue's VAT number")
		}
	case TaxSalesTaxUS:
		// The jurisdiction rates are a list, so they are set in the back
		// office and not asked for here.
	default:
		return fail("tax_profile", "tax profile %q is not one of %s, %s, %s, %s",
			a.TaxProfile, TaxNone, TaxGSTIndia, TaxVATUK, TaxSalesTaxUS)
	}
	if a.TaxPricing != "" && a.TaxPricing != PricingInclusive && a.TaxPricing != PricingExclusive {
		return fail("tax_pricing", "tax pricing %q is not %s or %s", a.TaxPricing, PricingInclusive, PricingExclusive)
	}

	if a.OwnerName = strings.TrimSpace(a.OwnerName); a.OwnerName == "" {
		return fail("owner_name", "the owner needs a name")
	}
	if utf8.RuneCountInString(a.OwnerName) > maxOwnerName || !printable(a.OwnerName) {
		return fail("owner_name", "the owner's name is up to %d characters of plain text", maxOwnerName)
	}
	if err := checkOwnerPIN(a.OwnerPIN); err != nil {
		return fail("owner_pin", "the owner's PIN: %v", err)
	}

	switch a.DeviceKind {
	case auth.KindTerminal, auth.KindKitchen, auth.KindBackOffice:
	default:
		return fail("device_kind", "a first device cannot be a %q", a.DeviceKind)
	}
	var device *auth.InvalidDevice
	if err := auth.CheckDeviceName(a.DeviceName); errors.As(err, &device) {
		return fail("device_name", "the first device: %s", device.Message)
	} else if err != nil {
		return Answers{}, err
	}
	return a, nil
}

// ownerRole is the default role the first staff member is given.
const ownerRole = "owner"

// checkOwnerPIN holds the owner's PIN to the rule the back office holds
// every PIN to (ADR-260): the owner holds every permission, staff.manage
// among them, so theirs is six to eight digits.
func checkOwnerPIN(pin string) error {
	owner := auth.PermissionSet{}
	for _, p := range auth.Permissions() {
		owner[p] = struct{}{}
	}
	return auth.CheckPINFor(pin, owner)
}

// createRoles writes the default roles and returns their IDs by name.
func createRoles(ctx context.Context, q *gen.Queries, venueID int64, now time.Time) (map[string]int64, error) {
	out := map[string]int64{}
	for _, template := range auth.DefaultRoles() {
		keys := make([]string, len(template.Permissions))
		for i, p := range template.Permissions {
			keys[i] = string(p)
		}
		set, err := auth.NewPermissionSet(keys)
		if err != nil {
			return nil, err
		}
		raw, err := set.JSON()
		if err != nil {
			return nil, err
		}
		role, err := q.CreateRole(ctx, gen.CreateRoleParams{
			VenueID: venueID, Name: template.Name, Permissions: raw, CreatedAt: now,
		})
		if err != nil {
			return nil, err
		}
		out[template.Name] = role.ID
	}
	if _, ok := out[ownerRole]; !ok {
		return nil, fmt.Errorf("firstrun: the default roles have no %q", ownerRole)
	}
	return out, nil
}

// writeSettings stores what the order service reads beside the venue row:
// how prices are quoted, and the GST state, which is the first two digits
// of the GSTIN and so is never asked for twice.
func writeSettings(ctx context.Context, q *gen.Queries, venueID int64, a Answers, now time.Time) error {
	settings := map[string]string{}
	if a.TaxPricing != "" {
		settings[svcorder.SettingTaxPricing] = a.TaxPricing
	}
	if a.TaxProfile == TaxGSTIndia {
		settings[svcorder.SettingTaxStateCode] = a.TaxRegistration[:2]
	}
	for key, value := range settings {
		if err := q.SetSetting(ctx, gen.SetSettingParams{
			VenueID: venueID, Key: key, Value: value, UpdatedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}
