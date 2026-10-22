package cloudkit_test

import (
	"context"
	"testing"

	"github.com/mez-pos/mez/cloudkit"
)

func TestTheCloudReadsAVenuesReplica(t *testing.T) {
	b := pairedVenueInBucket(t)
	ctx := context.Background()
	r, err := cloudkit.OpenReplica(ctx, cloudkit.ReplicaOptions{Bucket: b, Venue: mirroredVenue, CopyDir: t.TempDir()})
	if err != nil {
		t.Fatalf("OpenReplica: %v", err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()
	if got := r.Venue(); got.PublicID != mirroredVenue {
		t.Errorf("Venue = %+v", got)
	}
	closes, err := r.Closes(ctx, 0, 10)
	if err != nil {
		t.Fatalf("Closes: %v", err)
	}
	if len(closes) != 0 {
		t.Errorf("a venue that never closed has closes: %+v", closes)
	}
	if changed, err := r.Refresh(ctx); err != nil || changed {
		t.Errorf("Refresh = %v, %v", changed, err)
	}
}

func TestOpenReplicaWantsAVenuesPublicID(t *testing.T) {
	if _, err := cloudkit.OpenReplica(context.Background(), cloudkit.ReplicaOptions{Venue: "corner-cafe"}); err == nil {
		t.Fatal("OpenReplica took a slug for a venue id")
	}
}

func TestMoneyText(t *testing.T) {
	tests := []struct {
		minor    int64
		currency string
		want     string
	}{
		{0, "INR", "0.00"},
		{2431000, "INR", "24,310.00"},
		{10000000, "INR", "1,00,000.00"},
		{123456789, "INR", "12,34,567.89"},
		{-12000, "INR", "-120.00"},
		{999, "INR", "9.99"},
		{123456700, "USD", "1,234,567.00"},
		{5000, "JPY", "5,000"},
	}
	for _, tt := range tests {
		if got := cloudkit.MoneyText(tt.minor, tt.currency); got != tt.want {
			t.Errorf("MoneyText(%d, %s) = %q, want %q", tt.minor, tt.currency, got, tt.want)
		}
	}
}
