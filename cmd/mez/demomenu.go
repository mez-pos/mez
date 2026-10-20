package main

import (
	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/domain/menu"
	"github.com/mez-pos/mez/internal/domain/tax"
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
// menu to be designed against: required and optional modifiers, defaults,
// a free choice, two stations and two courses. No modifier costs less than
// nothing: order.LineAdded refuses a negative modifier price (ADR-071). Every
// item has the description and dietary mark its card shows, with a few marked
// popular and gluten free, so a screen built on the demo looks finished with no
// photo (ADR-147).
func demoMenu() db.MenuSpec {
	const food, bar, kitchen = "Restaurant service 5%", "bar", "kitchen"
	item := func(name string, rupees int64, station string, course int64, groups ...string) db.ItemSpec {
		return db.ItemSpec{
			Name: name, Price: rupees * 100, HSN: sacRestaurant, TaxClass: food,
			Station: station, Course: course, Groups: groups,
		}
	}
	// dish dresses an item with what its card shows.
	dish := func(it db.ItemSpec, description string, diet menu.Diet, glutenFree, popular bool) db.ItemSpec {
		it.Description, it.Diet, it.GlutenFree, it.Popular = description, diet, glutenFree, popular
		return it
	}
	const (
		veg, vegan, nonVeg = menu.DietVeg, menu.DietVegan, menu.DietNonVeg
	)
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
				{Name: "Almond", Price: 5000}, {Name: "No milk"},
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
			// Required with no default: the till has to ask, so the modifier
			// sheet opens on the tap rather than ringing a guess up.
			{Name: "Bread", MinSelect: 1, MaxSelect: 1, Modifiers: []db.ModifierSpec{
				{Name: "Sourdough"}, {Name: "Multigrain"}, {Name: "Gluten free", Price: 4000},
			}},
			{Name: "Ice cream", MaxSelect: 1, Modifiers: []db.ModifierSpec{
				{Name: "Scoop of vanilla", Price: 6000},
			}},
		},
		Categories: []db.CategorySpec{
			{Name: "Coffee", Items: []db.ItemSpec{
				dish(item("Filter coffee", 60, bar, 1, "Size", "Sugar"), "Strong South Indian decoction with frothed milk.", veg, true, true),
				dish(item("Espresso", 120, bar, 1, "Extras"), "A short, intense double shot.", vegan, true, false),
				dish(item("Americano", 150, bar, 1, "Size", "Extras"), "Espresso lengthened with hot water.", vegan, true, false),
				dish(item("Cappuccino", 190, bar, 1, "Size", "Milk", "Extras"), "Espresso under steamed milk and a deep foam.", veg, true, true),
				dish(item("Flat white", 210, bar, 1, "Milk", "Extras"), "Two shots, silky milk, no foam to speak of.", veg, true, false),
				dish(item("Latte", 200, bar, 1, "Size", "Milk", "Extras"), "A gentle shot in a lot of steamed milk.", veg, true, false),
			}},
			{Name: "Tea and cold", Items: []db.ItemSpec{
				dish(item("Masala chai", 70, bar, 1, "Size", "Sugar"), "Ginger, cardamom and strong tea simmered in milk.", veg, true, true),
				dish(item("Iced tea", 160, bar, 1, "Sugar"), "Black tea over ice with lemon.", vegan, true, false),
				dish(item("Cold coffee", 220, bar, 1, "Milk", "Sugar", "Extras"), "Blended with milk and ice, thick and sweet.", veg, true, false),
				dish(item("Fresh lime soda", 120, bar, 1, "Sugar"), "Lime, soda and a pinch of salt, sweet or salted.", vegan, true, false),
			}},
			{Name: "Breakfast", Items: []db.ItemSpec{
				dish(item("Idli vada", 110, kitchen, 1, "Add on"), "Soft idlis and a crisp vada, with sambar and chutney.", veg, true, true),
				dish(item("Masala dosa", 140, kitchen, 1, "Add on"), "A crisp rice crepe around spiced potato.", veg, true, true),
				dish(item("Upma", 90, kitchen, 1, "Add on"), "Roasted semolina with mustard, curry leaf and cashew.", veg, false, false),
				dish(item("Egg bhurji toast", 180, kitchen, 1), "Spiced scrambled eggs on buttered toast.", nonVeg, false, false),
				dish(item("Avocado toast", 320, kitchen, 1, "Bread"), "Smashed avocado, chilli and lime on your choice of bread.", vegan, false, false),
			}},
			{Name: "Cakes", Items: []db.ItemSpec{
				dish(item("Chocolate brownie", 150, kitchen, 2, "Ice cream"), "Dense, warm and not too sweet.", veg, false, true),
				dish(item("Banana walnut cake", 170, kitchen, 2), "Ripe banana, toasted walnut, a spiced crumb.", veg, false, false),
				dish(item("Lemon tart", 190, kitchen, 2), "Sharp lemon curd in a buttery shell.", veg, false, false),
			}},
		},
	}
}
