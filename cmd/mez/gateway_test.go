package main

import (
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/config"
)

// TestNewGateway: a gateway set with its keys is built; one set without
// them, or one this build cannot use, stops the start, in words that never
// carry the secret.
func TestNewGateway(t *testing.T) {
	const secret = "s3cr3t-that-must-never-be-seen"
	tests := []struct {
		name string
		cfg  config.Config
		ok   bool
		says string
	}{
		{"stripe with its key and the venue's address", config.Config{Gateway: config.GatewayStripe, GatewayKeySecret: secret, PublicURL: "https://cafe.example"}, true, ""},
		{"stripe with no key", config.Config{Gateway: config.GatewayStripe, PublicURL: "https://cafe.example"}, false, "gateway_key_secret"},
		// Stripe makes no page with nowhere to send the guest back to.
		{"stripe with no address to come back to", config.Config{Gateway: config.GatewayStripe, GatewayKeySecret: secret}, false, "public_url"},
		{"razorpay with both keys", config.Config{Gateway: config.GatewayRazorpay, GatewayKeyID: "rzp_test_1", GatewayKeySecret: secret}, true, ""},
		{"razorpay with no secret", config.Config{Gateway: config.GatewayRazorpay, GatewayKeyID: "rzp_test_1"}, false, "gateway_key_secret"},
		{"razorpay with no key id", config.Config{Gateway: config.GatewayRazorpay, GatewayKeySecret: secret}, false, "gateway_key_id"},
		{"a gateway with no adapter", config.Config{Gateway: "paypal", GatewayKeyID: "x", GatewayKeySecret: secret}, false, "no adapter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := newGateway(tt.cfg)
			if (err == nil) != tt.ok {
				t.Fatalf("err = %v", err)
			}
			if err != nil {
				if !strings.Contains(err.Error(), tt.says) {
					t.Errorf("%q does not say %q", err, tt.says)
				}
				if strings.Contains(err.Error(), secret) {
					t.Errorf("the secret is in the error: %v", err)
				}
				return
			}
			if g.Name() != tt.cfg.Gateway {
				t.Errorf("built %q", g.Name())
			}
		})
	}
}
