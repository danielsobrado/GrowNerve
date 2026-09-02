package middleware

import "testing"

func TestLimiterBoundsHighCardinalityClients(t *testing.T) {
	limiter := NewLimiter(RateLimit{Rate: 1, Burst: 1})
	limiter.maximumBuckets = 2
	if !limiter.Allow("first") || !limiter.Allow("second") {
		t.Fatal("regular client buckets were not admitted")
	}
	if !limiter.Allow("overflow-first") {
		t.Fatal("first overflow client should receive the shared overflow burst")
	}
	if limiter.Allow("overflow-second") {
		t.Fatal("high-cardinality clients received independent buckets past the cap")
	}
	if len(limiter.buckets) != 2 {
		t.Fatalf("bucket count = %d, want 2", len(limiter.buckets))
	}
}
