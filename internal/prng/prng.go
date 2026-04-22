// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package prng

import "math/rand/v2"

type PRNG struct {
	r *rand.Rand
}

func NewFromSeed(s1, s2 uint64) *PRNG {
	return &PRNG{
		r: rand.New(rand.NewPCG(s1, s2)),
	}
}

func (p *PRNG) D4(n int) int {
	result := 0
	for ; n > 0; n-- {
		result += p.Roll(1, 4)
	}
	return result
}

func (p *PRNG) D6(n int) int {
	result := 0
	for ; n > 0; n-- {
		result += p.Roll(1, 6)
	}
	return result
}

func (p *PRNG) D8(n int) int {
	result := 0
	for ; n > 0; n-- {
		result += p.Roll(1, 8)
	}
	return result
}

func (p *PRNG) D10(n int) int {
	result := 0
	for ; n > 0; n-- {
		result += p.Roll(1, 10)
	}
	return result
}

func (p *PRNG) D12(n int) int {
	result := 0
	for ; n > 0; n-- {
		result += p.Roll(1, 12)
	}
	return result
}

func (p *PRNG) D20(n int) int {
	result := 0
	for ; n > 0; n-- {
		result += p.Roll(1, 20)
	}
	return result
}

func (p *PRNG) D100(n int) int {
	result := 0
	for ; n > 0; n-- {
		result += p.Roll(1, 100)
	}
	return result
}

func (p *PRNG) Float64() float64 {
	return p.r.Float64()
}

func (p *PRNG) Shuffle(n int, swap func(i, j int)) {
	p.r.Shuffle(n, swap)
}

func (p *PRNG) Split() *PRNG {
	s1, s2 := p.r.Uint64(), p.r.Uint64()
	return &PRNG{
		r: rand.New(rand.NewPCG(s1, s2)),
	}
}

func (p *PRNG) Vary5Percent(x int) float64 {
	return float64(93*x+p.D6(2)) / 100.0
}

func (p *PRNG) Vary10Percent(x int) float64 {
	return float64(86*x+p.D6(4)) / 100.0
}

// Roll returns a uniformly distributed random integer in the range [low, high] (inclusive on both ends).
// intentionally ignores over and under flow.
func (p *PRNG) Roll(low, high int) int {
	if high < low {
		low, high = high, low
	}
	delta := high - low + 1
	return low + p.r.IntN(delta)
}
