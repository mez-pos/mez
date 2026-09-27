package order

import (
	"errors"
	"testing"
)

func atTable(n int64) OrderOpened {
	return OrderOpened{Channel: ChannelCounter, Currency: "INR", TableID: n, Covers: 2, OpenedBy: 11}
}

func TestTableMoves(t *testing.T) {
	tests := []struct {
		name    string
		events  []Payload
		table   int64
		joined  []int64
		wantErr error
	}{
		{"moved", []Payload{atTable(4), OrderTableChanged{From: 4, To: 7}}, 7, nil, nil},
		{"a counter ticket seated", []Payload{opened(), OrderTableChanged{From: 0, To: 3}}, 3, nil, nil},
		{"moved from where it is not", []Payload{atTable(4), OrderTableChanged{From: 5, To: 7}}, 0, nil, ErrTableMismatch},
		{"moved to where it is", []Payload{atTable(4), OrderTableChanged{From: 4, To: 4}}, 0, nil, ErrTableAlreadyAt},
		{"moved to no table", []Payload{atTable(4), OrderTableChanged{From: 4, To: 0}}, 0, nil, ErrInvalidPayload},
		{"moved onto a table it joined", []Payload{atTable(4), TablesJoined{Tables: []int64{5}}, OrderTableChanged{From: 4, To: 5}}, 0, nil, ErrTableAlreadyAt},
		{"joined", []Payload{atTable(4), TablesJoined{Tables: []int64{5, 6}}}, 4, []int64{5, 6}, nil},
		{"joined twice over", []Payload{atTable(4), TablesJoined{Tables: []int64{5}}, TablesJoined{Tables: []int64{6}}}, 4, []int64{5, 6}, nil},
		{"joined with no table", []Payload{opened(), TablesJoined{Tables: []int64{5}}}, 0, nil, ErrNoTable},
		{"joined to itself", []Payload{atTable(4), TablesJoined{Tables: []int64{4}}}, 0, nil, ErrTableAlreadyAt},
		{"the same table twice", []Payload{atTable(4), TablesJoined{Tables: []int64{5, 5}}}, 0, nil, ErrTableAlreadyAt},
		{"joined nothing", []Payload{atTable(4), TablesJoined{}}, 0, nil, ErrInvalidPayload},
		{"joined a table that is none", []Payload{atTable(4), TablesJoined{Tables: []int64{-1}}}, 0, nil, ErrInvalidPayload},
		{"parted", []Payload{atTable(4), TablesJoined{Tables: []int64{5, 6}}, TablesParted{Tables: []int64{5}}}, 4, []int64{6}, nil},
		{"parted to one table", []Payload{atTable(4), TablesJoined{Tables: []int64{5}}, TablesParted{Tables: []int64{5}}}, 4, nil, nil},
		{"parted what was never joined", []Payload{atTable(4), TablesParted{Tables: []int64{5}}}, 0, nil, ErrTableNotJoined},
		{"parted nothing", []Payload{atTable(4), TablesJoined{Tables: []int64{5}}, TablesParted{}}, 0, nil, ErrInvalidPayload},
		{"moved while held", []Payload{atTable(4), OrderHeld{}, OrderTableChanged{From: 4, To: 7}}, 0, nil, ErrOrderHeld},
		{"moved once closed", []Payload{atTable(4), closed, OrderTableChanged{From: 4, To: 7}}, 0, nil, ErrOrderClosed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Project(stream(tt.events...), untaxed)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if s.TableID != tt.table || len(s.Joined) != len(tt.joined) {
				t.Fatalf("at %d + %v, want %d + %v", s.TableID, s.Joined, tt.table, tt.joined)
			}
			for i := range tt.joined {
				if s.Joined[i] != tt.joined[i] {
					t.Errorf("joined = %v, want %v", s.Joined, tt.joined)
				}
			}
		})
	}
}

func TestTablesListsEveryTable(t *testing.T) {
	s := mustProject(t, atTable(4), TablesJoined{Tables: []int64{5, 6}})
	if got := s.Tables(); len(got) != 3 || got[0] != 4 || got[2] != 6 {
		t.Errorf("Tables() = %v", got)
	}
	if got := mustProject(t, opened()).Tables(); got != nil {
		t.Errorf("a counter ticket's tables = %v", got)
	}
}

// A resumed projection carries the joined tables without sharing them.
func TestResumeClonesTheJoinedTables(t *testing.T) {
	events := stream(atTable(4), TablesJoined{Tables: []int64{5}}, TablesJoined{Tables: []int64{6}})
	head, err := Project(events[:2], untaxed)
	if err != nil {
		t.Fatal(err)
	}
	full, err := Resume(head, events[2:], untaxed)
	if err != nil {
		t.Fatal(err)
	}
	if len(head.Joined) != 1 || len(full.Joined) != 2 {
		t.Errorf("head %v, full %v", head.Joined, full.Joined)
	}
}
