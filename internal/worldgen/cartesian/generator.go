// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package cartesian

import (
	"github.com/mdhender/drynn/internal/prng"
)

func Generate(options ...Option) (*Galaxy, error) {
	g := &Generator{
		desiredNumSystems: 100,
		gg:                &StandardGalaxyGenerator{},
		pg:                &NaiveDiskPointsGenerator{},
		r:                 prng.NewFromSeed(10, 10),
	}
	for _, opt := range options {
		if err := opt(g); err != nil {
			return nil, err
		}
	}
	return g.gg.Generate(g.desiredNumSystems, g.desiredRadius, g.r, g.pg), nil
}

type Option func(generator *Generator) error

func WithDesiredNumberOfSystems(n int) Option {
	return func(g *Generator) error {
		g.desiredNumSystems = n
		return nil
	}
}

func WithDesiredRadius(n float64) Option {
	return func(g *Generator) error {
		g.desiredRadius = n
		return nil
	}
}

func WithGalaxyGenerator(gg GalaxyGenerator) Option {
	return func(g *Generator) error {
		g.gg = gg
		return nil
	}
}

func WithPointGenerator(pg PointGenerator) Option {
	return func(g *Generator) error {
		g.pg = pg
		return nil
	}
}

func WithPRNG(r *prng.PRNG) Option {
	return func(g *Generator) error {
		g.r = r
		return nil
	}
}

type Generator struct {
	desiredNumSystems int
	desiredRadius     float64
	gg                GalaxyGenerator
	pg                PointGenerator
	r                 *prng.PRNG
}
