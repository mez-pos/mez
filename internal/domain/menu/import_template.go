package menu

import (
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/sheet"
)

// exampleCategory is the category the examples go in at a venue with no
// category to name.
const exampleCategory = "Food"

// exampleHSN is the code on the examples where every item needs one: the SAC
// code for a restaurant's service, which is what most of a cafe's menu is
// sold under.
const exampleHSN = "996331"

// ImportTemplate is the example rows of the file a venue fills in to import
// its menu (ADR-290), a cell for each of ImportColumns, which is the header.
//
// The rows name what the venue has: its first category, tax class, station
// and course still in use. So the template imports as it comes, and shows
// the owner the names their own rows must use. At a venue with no tax
// class, station or course those cells are empty and the import says what
// to add first.
func ImportTemplate(v ImportVenue) [][]string {
	category := firstActive(v.Categories)
	if category == "" {
		category = exampleCategory
	}
	taxClass, station, course := firstActive(v.TaxClasses), firstActive(v.Stations), firstActive(v.Courses)
	// Empty at a venue with no groups yet: the choices are imported first.
	choices := firstActive(v.Groups)
	hsn := ""
	if v.NeedsHSN {
		hsn = exampleHSN
	}
	// A price with decimals only where the currency has them.
	second := "80.50"
	if money.Exponent(v.Currency) == 0 {
		second = "80"
	}
	return [][]string{
		{category, sheet.ExamplePrefix + " Masala dosa", "120", taxClass, hsn, station, course, "Crisp rice crepe with spiced potato", "veg", "yes", "yes", choices},
		{category, sheet.ExamplePrefix + " Filter coffee", second, taxClass, hsn, station, course, "", "", "no", "no", ""},
	}
}

// firstActive is the name of the first choice still in use, or "".
func firstActive(choices []ImportChoice) string {
	for _, c := range choices {
		if c.Active {
			return c.Name
		}
	}
	return ""
}
