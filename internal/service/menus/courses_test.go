package menus

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

// course makes a course of the venue through the service.
func (w *world) course(t *testing.T, venue int64, name string) int64 {
	t.Helper()
	id, err := w.s.CreateCourse(context.Background(), venue, CourseInput{Name: name})
	if err != nil {
		t.Fatalf("creating course %q: %v", name, err)
	}
	return id
}

// numbered is the editor's courses as id by number, in order.
func numbered(e Editor) []int64 {
	var out []int64
	for _, c := range e.Courses {
		out = append(out, c.ID)
	}
	return out
}

func courseByID(e Editor, id int64) Course {
	i := slices.IndexFunc(e.Courses, func(c Course) bool { return c.ID == id })
	if i < 0 {
		return Course{}
	}
	return e.Courses[i]
}

// openTicket leaves a ticket open at the venue: the courses cannot be
// renumbered under it.
func (w *world) openTicket(t *testing.T, venue int64) {
	t.Helper()
	if _, err := w.d.Writer().ExecContext(context.Background(),
		`INSERT INTO orders (public_id, venue_id, channel, status, last_seq, opened_at, updated_at)
		 VALUES ('ord_COURSE01', ?, 'counter', 'open', 1, ?, ?)`, venue, now, now); err != nil {
		t.Fatal(err)
	}
}

func TestCreateACourse(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	before := w.version(t, plain)
	starters := w.course(t, plain, "  Starters ")
	desserts := w.course(t, plain, "Desserts")
	if w.version(t, plain) != before+2 || !slices.Equal(w.nudged, []int64{plain, plain}) {
		t.Errorf("version %d (was %d), nudged %v", w.version(t, plain), before, w.nudged)
	}
	e := w.editor(t, plain)
	if got := courseByID(e, starters); got.Name != "Starters" || got.Number != 2 || !got.Active {
		t.Errorf("starters = %+v: trimmed, and numbered after the last", got)
	}
	if got := courseByID(e, desserts); got.Number != 3 {
		t.Errorf("desserts = %+v", got)
	}
	if m, err := w.d.Menu(ctx, plain); err != nil || len(m.Courses) != 3 {
		t.Errorf("the till's courses: %+v, %v", m.Courses, err)
	}
	// Another venue numbers its own.
	if id := w.course(t, gst, "Starters"); courseByID(w.editor(t, gst), id).Number != 2 {
		t.Error("another venue's numbering is its own")
	}
	for _, name := range []string{"", "  ", strings.Repeat("a", MaxNameLength+1)} {
		if _, err := w.s.CreateCourse(ctx, plain, CourseInput{Name: name}); fieldOf(err) != "name" {
			t.Errorf("name %q: %v", name, err)
		}
	}
}

func TestRenameACourse(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	mains := w.refs[plain].course
	if err := w.s.RenameCourse(ctx, plain, mains, CourseInput{Name: " Main course "}); err != nil {
		t.Fatal(err)
	}
	if got := courseByID(w.editor(t, plain), mains); got.Name != "Main course" || got.Number != 1 {
		t.Errorf("mains = %+v: the name changes, the number stays", got)
	}
	if err := w.s.RenameCourse(ctx, plain, mains, CourseInput{}); fieldOf(err) != "name" {
		t.Errorf("an empty name: %v", err)
	}
	if err := w.s.RenameCourse(ctx, plain, 999, CourseInput{Name: "X"}); !errors.Is(err, ErrCourseNotFound) {
		t.Errorf("a missing course: %v", err)
	}
	if err := w.s.RenameCourse(ctx, plain, w.refs[gst].course, CourseInput{Name: "X"}); !errors.Is(err, ErrCourseNotFound) {
		t.Errorf("another venue's course: %v", err)
	}
}

func TestOrderCourses(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	mains := w.refs[plain].course
	starters := w.course(t, plain, "Starters")
	desserts := w.course(t, plain, "Desserts")

	if err := w.s.OrderCourses(ctx, plain, []int64{starters, mains, desserts}); err != nil {
		t.Fatal(err)
	}
	e := w.editor(t, plain)
	if !slices.Equal(numbered(e), []int64{starters, mains, desserts}) {
		t.Fatalf("order = %v, want starters, mains, desserts", numbered(e))
	}
	for i, c := range e.Courses {
		if c.Number != int64(i)+1 {
			t.Errorf("%s is number %d, want %d", c.Name, c.Number, i+1)
		}
	}

	for _, stale := range [][]int64{
		{starters, mains},                     // one missing
		{starters, mains, desserts, desserts}, // one twice
		{starters, mains, w.refs[gst].course}, // another venue's
		nil,
	} {
		if err := w.s.OrderCourses(ctx, plain, stale); !errors.Is(err, ErrStaleOrder) {
			t.Errorf("OrderCourses(%v) = %v, want ErrStaleOrder", stale, err)
		}
	}
}

func TestOrderCoursesWhileTicketsAreOpen(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	mains := w.refs[plain].course
	starters := w.course(t, plain, "Starters")
	w.openTicket(t, plain)
	before := w.version(t, plain)

	err := w.s.OrderCourses(ctx, plain, []int64{starters, mains})
	if !errors.Is(err, ErrTicketsOpen) {
		t.Fatalf("a renumbering under an open ticket: %v, want ErrTicketsOpen", err)
	}
	e := w.editor(t, plain)
	if !slices.Equal(numbered(e), []int64{mains, starters}) || e.Version != before {
		t.Errorf("a refusal changed something: %v, version %d (was %d)", numbered(e), e.Version, before)
	}
	// The order it already has changes no number, so nothing is in the way.
	if err := w.s.OrderCourses(ctx, plain, []int64{mains, starters}); err != nil {
		t.Errorf("the same order: %v", err)
	}
	// Courses can still be added and renamed, and retired, with tickets open.
	if _, err := w.s.CreateCourse(ctx, plain, CourseInput{Name: "Desserts"}); err != nil {
		t.Errorf("adding a course: %v", err)
	}
}

// A gap left by hand (or by an older version) is closed by an ordering, and
// a closed ticket is no obstacle.
func TestOrderCoursesClosesGaps(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	mains := w.refs[plain].course
	starters := w.course(t, plain, "Starters")
	if _, err := w.d.Writer().ExecContext(ctx, `UPDATE courses SET number = 7 WHERE id = ?`, starters); err != nil {
		t.Fatal(err)
	}
	if err := w.s.OrderCourses(ctx, plain, []int64{mains, starters}); err != nil {
		t.Fatal(err)
	}
	if got := courseByID(w.editor(t, plain), starters).Number; got != 2 {
		t.Errorf("starters is number %d, want 2", got)
	}
}

func TestRetireACourseNeedsToSayWhereItsItemsGo(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	mains := w.refs[plain].course
	starters := w.course(t, plain, "Starters")
	cat := w.category(t, plain, "Food")
	dosa := w.create(t, plain, w.item(plain, cat, "Dosa", 12000, ""))
	retired := w.course(t, plain, "Cheese")
	if _, err := w.s.RetireCourse(ctx, plain, retired, RetireInput{}); err != nil {
		t.Fatal(err)
	}
	w.nudged = nil
	before := w.version(t, plain)

	tests := []struct {
		name string
		in   RetireInput
	}{
		{"naming nowhere", RetireInput{}},
		{"itself", RetireInput{MoveTo: mains}},
		{"a retired course", RetireInput{MoveTo: retired}},
		{"a course that is not there", RetireInput{MoveTo: 999}},
		{"another venue's course", RetireInput{MoveTo: w.refs[gst].course}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := w.s.RetireCourse(ctx, plain, mains, tt.in)
			if fieldOf(err) != "move_to" {
				t.Fatalf("err = %v, want a refusal of move_to", err)
			}
		})
	}
	e := w.editor(t, plain)
	if !courseByID(e, mains).Active || e.Version != before || len(w.nudged) != 0 {
		t.Errorf("a refused retire changed something: version %d (was %d), nudged %v", e.Version, before, w.nudged)
	}

	moved, err := w.s.RetireCourse(ctx, plain, mains, RetireInput{MoveTo: starters})
	if err != nil || moved.ItemsMoved != 1 {
		t.Fatalf("moved %+v, %v", moved, err)
	}
	e = w.editor(t, plain)
	if courseByID(e, mains).Active || courseByID(e, starters).Number != 2 {
		t.Errorf("courses = %+v: mains retired, the others keep their numbers", e.Courses)
	}
	if i := slices.IndexFunc(e.Items, func(it Item) bool { return it.ID == dosa }); e.Items[i].CourseID != starters {
		t.Errorf("dosa is in course %d, want starters %d", e.Items[i].CourseID, starters)
	}
	// The till reads the menu, retired course gone, dosa in its new one.
	m, err := w.d.Menu(ctx, plain)
	if err != nil {
		t.Fatalf("the till's menu after retiring a course: %v", err)
	}
	if len(m.Courses) != 1 || len(m.Items) != 1 || m.Items[0].Course != 2 {
		t.Errorf("the till's menu: courses %+v, items %+v; dosa is in course 2", m.Courses, m.Items)
	}
}

func TestRetiringACourseMovesRetiredItemsToo(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	mains := w.refs[plain].course
	starters := w.course(t, plain, "Starters")
	cat := w.category(t, plain, "Food")
	dosa := w.create(t, plain, w.item(plain, cat, "Dosa", 12000, ""))
	if err := w.s.SetItemActive(ctx, plain, dosa, false); err != nil {
		t.Fatal(err)
	}
	moved, err := w.s.RetireCourse(ctx, plain, mains, RetireInput{MoveTo: starters})
	if err != nil || moved.ItemsMoved != 1 {
		t.Fatalf("moved %+v, %v: a retired item counts", moved, err)
	}
	if err := w.s.SetItemActive(ctx, plain, dosa, true); err != nil {
		t.Fatalf("bringing dosa back: %v", err)
	}
	if _, err := w.d.Menu(ctx, plain); err != nil {
		t.Errorf("the till's menu with dosa back: %v", err)
	}
}

func TestACourseThatCarriesNothingRetiresAlone(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	cheese := w.course(t, plain, "Cheese")
	if moved, err := w.s.RetireCourse(ctx, plain, cheese, RetireInput{}); err != nil || moved.ItemsMoved != 0 {
		t.Fatalf("an empty course: %+v, %v", moved, err)
	}
	if moved, err := w.s.RetireCourse(ctx, plain, cheese, RetireInput{}); err != nil || moved.ItemsMoved != 0 {
		t.Errorf("retiring it again: %+v, %v", moved, err)
	}
	if _, err := w.s.RetireCourse(ctx, plain, 999, RetireInput{}); !errors.Is(err, ErrCourseNotFound) {
		t.Errorf("a missing course: %v", err)
	}
	if _, err := w.s.RetireCourse(ctx, plain, w.refs[gst].course, RetireInput{}); !errors.Is(err, ErrCourseNotFound) {
		t.Errorf("another venue's course: %v", err)
	}
}

func TestTheLastCourseInServiceStays(t *testing.T) {
	w := newWorld(t)
	_, err := w.s.RetireCourse(context.Background(), plain, w.refs[plain].course, RetireInput{})
	if fieldOf(err) != "id" {
		t.Fatalf("err = %v, want the only course refused", err)
	}
	if !courseByID(w.editor(t, plain), w.refs[plain].course).Active {
		t.Error("the only course was retired")
	}
}

func TestRestoreACourse(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	cheese := w.course(t, plain, "Cheese")
	if _, err := w.s.RetireCourse(ctx, plain, cheese, RetireInput{}); err != nil {
		t.Fatal(err)
	}
	if m, _ := w.d.Menu(ctx, plain); len(m.Courses) != 1 {
		t.Fatalf("retired, it is still on the till: %+v", m.Courses)
	}
	before := w.version(t, plain)
	if err := w.s.RestoreCourse(ctx, plain, cheese); err != nil {
		t.Fatal(err)
	}
	if !courseByID(w.editor(t, plain), cheese).Active || w.version(t, plain) != before+1 {
		t.Error("the course did not come back, or the menu version did not move")
	}
	if m, _ := w.d.Menu(ctx, plain); len(m.Courses) != 2 {
		t.Errorf("restored, it is not on the till: %+v", m.Courses)
	}
	if err := w.s.RestoreCourse(ctx, plain, 999); !errors.Is(err, ErrCourseNotFound) {
		t.Errorf("a missing course: %v", err)
	}
}

// An item cannot name a course that is retired, when it is made or when it
// comes back.
func TestItemsAvoidRetiredCourses(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	cheese := w.course(t, plain, "Cheese")
	cat := w.category(t, plain, "Food")
	dosa := w.create(t, plain, w.item(plain, cat, "Dosa", 12000, ""))
	if err := w.s.SetItemActive(ctx, plain, dosa, false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.d.Writer().ExecContext(ctx, `UPDATE items SET course_id = ? WHERE id = ?`, cheese, dosa); err != nil {
		t.Fatal(err)
	}
	if _, err := w.s.RetireCourse(ctx, plain, cheese, RetireInput{MoveTo: w.refs[plain].course}); err != nil {
		t.Fatal(err)
	}
	// The retire moved dosa on; put it back on the retired course by hand,
	// as an older version could have left it.
	if _, err := w.d.Writer().ExecContext(ctx, `UPDATE items SET course_id = ? WHERE id = ?`, cheese, dosa); err != nil {
		t.Fatal(err)
	}
	if err := w.s.SetItemActive(ctx, plain, dosa, true); fieldOf(err) != "course_id" {
		t.Errorf("bringing back an item in a retired course: %v", err)
	}
	in := w.item(plain, cat, "Brie", 30000, "")
	in.CourseID = cheese
	if _, err := w.s.CreateItem(ctx, plain, in); fieldOf(err) != "course_id" {
		t.Errorf("a new item in a retired course: %v", err)
	}
}
