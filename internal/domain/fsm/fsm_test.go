package fsm

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type light string

const (
	red    light = "red"
	green  light = "green"
	amber  light = "amber"
	broken light = "broken"
)

func lights() Machine[light] {
	return Must("light", []light{red, green, amber, broken}, map[light][]light{
		red:   {green, broken},
		green: {amber, broken},
		amber: {red, broken},
	})
}

func TestTransition(t *testing.T) {
	m := lights()
	tests := []struct {
		name     string
		from, to light
		ok       bool
	}{
		{"legal", red, green, true},
		{"legal to a terminal state", amber, broken, true},
		{"skipping a state", red, amber, false},
		{"backwards", green, red, false},
		{"out of a terminal state", broken, red, false},
		{"staying put", red, red, false},
		{"from an unknown state", "blue", red, false},
		{"to an unknown state", red, "blue", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m.Transition(tt.from, tt.to)
			if tt.ok {
				if err != nil || got != tt.to {
					t.Fatalf("Transition(%q, %q) = %q, %v; want %q, nil", tt.from, tt.to, got, err, tt.to)
				}
				return
			}
			if !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("Transition(%q, %q) err = %v, want ErrInvalidTransition", tt.from, tt.to, err)
			}
			if got != tt.from {
				t.Errorf("a refused move returned %q, want the unchanged %q", got, tt.from)
			}
			var te *TransitionError
			if !errors.As(err, &te) || te.Machine != "light" || te.From != string(tt.from) || te.To != string(tt.to) {
				t.Errorf("error = %#v", err)
			}
			if !strings.Contains(err.Error(), `"`+string(tt.to)+`"`) {
				t.Errorf("message %q does not name the target state", err.Error())
			}
		})
	}
}

func TestNewRefusesABrokenTable(t *testing.T) {
	tests := []struct {
		name   string
		states []light
		edges  map[light][]light
	}{
		{"move from an unknown state", []light{red}, map[light][]light{green: {red}}},
		{"move to an unknown state", []light{red}, map[light][]light{red: {green}}},
		{"a state that moves to itself", []light{red}, map[light][]light{red: {red}}},
		{"a state listed twice", []light{red, green, red}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New("light", tt.states, tt.edges); err == nil {
				t.Error("New accepted a broken table")
			}
		})
	}
}

func TestMustPanicsOnABrokenTable(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Must did not panic on a broken table")
		}
	}()
	Must("light", []light{red}, map[light][]light{red: {green}})
}

func TestAccessorsDoNotLeakTheTable(t *testing.T) {
	edges := map[light][]light{red: {green}}
	states := []light{red, green}
	m := Must("light", states, edges)

	// Changing what New was given, or what an accessor returned, must not
	// change the machine.
	edges[red][0] = red
	states[1] = amber
	m.Next(red)[0] = amber
	m.States()[0] = broken

	if !m.Can(red, green) || m.Can(red, amber) {
		t.Error("the machine's moves changed after it was built")
	}
	if want := []light{red, green}; !reflect.DeepEqual(m.States(), want) {
		t.Errorf("States() = %v, want %v", m.States(), want)
	}
	if m.Name() != "light" || m.Next(green) != nil {
		t.Errorf("Name() = %q, Next(green) = %v", m.Name(), m.Next(green))
	}
}
