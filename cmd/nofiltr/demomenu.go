package main

import (
	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

// demoMenuCurrency is the currency the demo menu is priced in. Its prices
// are paise; in any other currency they would be nonsense, so setup refuses
// the demo menu for a venue that is not in rupees.
const demoMenuCurrency = "INR"

// SAC 996331 is "services provided by restaurants, cafes and similar
// eating facilities", which is how GST sees nearly everything a cafe rings
// up. It is taxed at 5% without input credit.
const sacRestaurant = "996331"

// demoMenu is a small South Indian cafe: coffee and cold drinks from the
// bar, breakfast and cake from the kitchen. It exists so a new install can
// ring up a sale in its first minute, and so the terminal has a realistic
// menu to be designed against: required and optional modifiers, a default,
// a negative price delta, two stations and two courses.
func demoMenu() db.MenuSpec {
	const food, bar, kitchen = "Restaurant service 5%", "bar", "kitchen"
	item := func(name string, rupees int64, station string, course int64, groups ...string) db.ItemSpec {
		return db.ItemSpec{
			Name: name, Price: rupees * 100, HSN: sacRestaurant, TaxClass: food,
			Station: station, Course: course, Groups: groups,
		}
	}
	return db.MenuSpec{
		Stations: []db.StationSpec{{Code: bar, Name: "Bar"}, {Code: kitchen, Name: "Kitchen"}},
		Courses:  []db.CourseSpec{{Number: 1, Name: "First"}, {Number: 2, Name: "Dessert"}},
		TaxClasses: []db.TaxClassSpec{
			{Name: food, Rate: tax.RateFromBP(500)},
		},
		Groups: []db.ModifierGroupSpec{
			{Name: "Size", MinSelect: 1, MaxSelect: 1, Modifiers: []db.ModifierSpec{
				{Name: "Regular", IsDefault: true}, {Name: "Large", Price: 4000},
			}},
			{Name: "Milk", MinSelect: 1, MaxSelect: 1, Modifiers: []db.ModifierSpec{
				{Name: "Dairy", IsDefault: true}, {Name: "Oat", Price: 5000},
				{Name: "Almond", Price: 5000}, {Name: "No milk", Price: -2000},
			}},
			{Name: "Sugar", MinSelect: 1, MaxSelect: 1, Modifiers: []db.ModifierSpec{
				{Name: "Regular sugar", IsDefault: true}, {Name: "Less sugar"}, {Name: "No sugar"},
			}},
			{Name: "Extras", MaxSelect: 3, Modifiers: []db.ModifierSpec{
				{Name: "Extra shot", Price: 3000}, {Name: "Vanilla syrup", Price: 2500},
				{Name: "Hazelnut syrup", Price: 2500},
			}},
			{Name: "Add on", MaxSelect: 2, Modifiers: []db.ModifierSpec{
				{Name: "Extra chutney", Price: 1500}, {Name: "Ghee", Price: 2000},
			}},
			{Name: "Ice cream", MaxSelect: 1, Modifiers: []db.ModifierSpec{
				{Name: "Scoop of vanilla", Price: 6000},
			}},
		},
		Categories: []db.CategorySpec{
			{Name: "Coffee", Items: []db.ItemSpec{
				item("Filter coffee", 60, bar, 1, "Size", "Sugar"),
				item("Espresso", 120, bar, 1, "Extras"),
				item("Americano", 150, bar, 1, "Size", "Extras"),
				item("Cappuccino", 190, bar, 1, "Size", "Milk", "Extras"),
				item("Flat white", 210, bar, 1, "Milk", "Extras"),
				item("Latte", 200, bar, 1, "Size", "Milk", "Extras"),
			}},
			{Name: "Tea and cold", Items: []db.ItemSpec{
				item("Masala chai", 70, bar, 1, "Size", "Sugar"),
				item("Iced tea", 160, bar, 1, "Sugar"),
				item("Cold coffee", 220, bar, 1, "Milk", "Sugar", "Extras"),
				item("Fresh lime soda", 120, bar, 1, "Sugar"),
			}},
			{Name: "Breakfast", Items: []db.ItemSpec{
				item("Idli vada", 110, kitchen, 1, "Add on"),
				item("Masala dosa", 140, kitchen, 1, "Add on"),
				item("Upma", 90, kitchen, 1, "Add on"),
				item("Egg bhurji toast", 180, kitchen, 1),
				item("Avocado toast", 320, kitchen, 1),
			}},
			{Name: "Cakes", Items: []db.ItemSpec{
				item("Chocolate brownie", 150, kitchen, 2, "Ice cream"),
				item("Banana walnut cake", 170, kitchen, 2),
				item("Lemon tart", 190, kitchen, 2),
			}},
		},
	}
}
