package stock

import (
	"errors"
	"math"
	"testing"
)

func TestValue(t *testing.T) {
	tests := []struct {
		name string
		q    Qty
		unit Unit
		cost int64
		want int64
	}{
		// Milk at 60.00 a litre.
		{"half a litre", 500 * PerUnit, Millilitre, 6000, 3000},
		{"a litre", 1000 * PerUnit, Millilitre, 6000, 6000},
		{"a millilitre is six paise", PerUnit, Millilitre, 6000, 6},
		{"a tenth of a millilitre rounds up from 0.6", 100, Millilitre, 6000, 1},
		{"a twentieth rounds down from 0.3", 50, Millilitre, 6000, 0},
		{"short by half a litre is a loss", -500 * PerUnit, Millilitre, 6000, -3000},
		// Beans at 1,250.00 a kilogram.
		{"an espresso's 18.5 g", 18500, Gram, 125000, 2313}, // 2312.5 rounds away from zero
		{"short by 18.5 g", -18500, Gram, 125000, -2313},
		// Limes at 7.50 each.
		{"three limes", 3 * PerUnit, Each, 750, 2250},
		{"half a lime", 500, Each, 750, 375},
		{"a third of a lime", 333, Each, 750, 250}, // 249.75
		{"nothing", 0, Gram, 125000, 0},
		{"no cost known", 5000 * PerUnit, Gram, 0, 0},
		{"the most of the dearest", MaxQty, Each, MaxCost, 1_000_000_000 * MaxCost},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Value(tt.q, tt.unit, tt.cost)
			if err != nil || got != tt.want {
				t.Errorf("Value(%d, %s, %d) = %d, %v; want %d", tt.q, tt.unit, tt.cost, got, err, tt.want)
			}
		})
	}
}

func TestValueRefuses(t *testing.T) {
	tests := []struct {
		name string
		q    Qty
		unit Unit
		cost int64
	}{
		{"a unit we do not have", PerUnit, "oz", 100},
		{"a cost below zero", PerUnit, Gram, -1},
		{"more than a number can hold", Qty(math.MaxInt64), Each, math.MaxInt64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Value(tt.q, tt.unit, tt.cost); !errors.Is(err, ErrValue) {
				t.Errorf("err = %v, want ErrValue", err)
			}
		})
	}
}

func TestCostMeasureAndClean(t *testing.T) {
	if CostMeasure(Gram) != 1_000_000 || CostMeasure(Millilitre) != 1_000_000 || CostMeasure(Each) != 1000 {
		t.Errorf("measures: %d, %d, %d", CostMeasure(Gram), CostMeasure(Millilitre), CostMeasure(Each))
	}
	for _, ok := range []int64{0, 1, MaxCost} {
		if p := CleanCost(ok); p != nil {
			t.Errorf("CleanCost(%d) = %+v, want none", ok, p)
		}
	}
	for _, bad := range []int64{-1, MaxCost + 1} {
		if p := CleanCost(bad); p == nil || p.Field != "cost" {
			t.Errorf("CleanCost(%d) = %+v, want a problem with the cost", bad, p)
		}
	}
}
