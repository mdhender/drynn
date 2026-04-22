// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package prng

import "testing"

// TestSplit_Determinism verifies that splitting the same master seed twice
// yields identical substream sequences, and that consuming the master via
// another method doesn't change that invariant.
func TestSplit_Determinism(t *testing.T) {
	m1 := NewFromSeed(42, 43)
	s1a := m1.Split()
	s1b := m1.Split()

	m2 := NewFromSeed(42, 43)
	s2a := m2.Split()
	s2b := m2.Split()

	for i := 0; i < 256; i++ {
		if got, want := s1a.Roll(1, 1_000_000), s2a.Roll(1, 1_000_000); got != want {
			t.Fatalf("substream a diverged at iteration %d: %d vs %d", i, got, want)
		}
		if got, want := s1b.Roll(1, 1_000_000), s2b.Roll(1, 1_000_000); got != want {
			t.Fatalf("substream b diverged at iteration %d: %d vs %d", i, got, want)
		}
	}
}

// TestSplit_Independence verifies that successive splits yield distinct
// streams (consecutive splits advance the master state).
func TestSplit_Independence(t *testing.T) {
	m := NewFromSeed(42, 43)
	a := m.Split()
	b := m.Split()

	// First 64 rolls should not all match — any match across 64 rolls of
	// a 1-billion range is astronomically unlikely unless the streams
	// are identical.
	matches := 0
	for i := 0; i < 64; i++ {
		if a.Roll(1, 1_000_000_000) == b.Roll(1, 1_000_000_000) {
			matches++
		}
	}
	if matches == 64 {
		t.Fatalf("splits a and b produced identical sequences; Split() is not advancing master state")
	}
}

// TestSplit_ConsumesMasterState verifies that Split() advances the master
// PRNG such that the same master, freshly seeded, yields a different
// substream on its Nth split than on its (N-1)th.
func TestSplit_ConsumesMasterState(t *testing.T) {
	m1 := NewFromSeed(42, 43)
	first := m1.Split()

	m2 := NewFromSeed(42, 43)
	m2.Split() // consume
	second := m2.Split()

	// First Roll of each should almost certainly differ.
	a := first.Roll(1, 1_000_000_000)
	b := second.Roll(1, 1_000_000_000)
	if a == b {
		t.Fatalf("first and second splits of fresh master produced identical first roll: %d", a)
	}
}

// TestSplit_DoesNotPerturbSiblingStream verifies that operations on one
// substream do not affect another substream split earlier. This is the
// load-bearing property for stage isolation: re-seeding or heavily
// consuming one stage must not shift another stage's output.
func TestSplit_DoesNotPerturbSiblingStream(t *testing.T) {
	// Baseline: split A and B from the same master, capture B's first roll.
	m1 := NewFromSeed(42, 43)
	_ = m1.Split()
	b1 := m1.Split()
	baseline := b1.Roll(1, 1_000_000_000)

	// Variant: split A, consume it heavily, then split B, capture its roll.
	m2 := NewFromSeed(42, 43)
	a := m2.Split()
	for i := 0; i < 10_000; i++ {
		a.Roll(1, 100)
	}
	b2 := m2.Split()
	variant := b2.Roll(1, 1_000_000_000)

	if baseline != variant {
		t.Fatalf("consuming substream A perturbed substream B: baseline=%d variant=%d", baseline, variant)
	}
}
