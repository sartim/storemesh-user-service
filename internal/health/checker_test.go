package health

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChecker_ReportsReadyWhenDependenciesAreUp(t *testing.T) {
	checker := NewChecker(
		time.Second,
		Dependency{Name: "postgres", Check: func(context.Context) error { return nil }},
		Dependency{Name: "redis", Check: func(context.Context) error { return nil }},
	)

	report := checker.Check(context.Background())

	assert.True(t, report.Ready())
	assert.Equal(t, StatusUp, report.Dependencies["postgres"])
	assert.Equal(t, StatusUp, report.Dependencies["redis"])
}

func TestChecker_ReportsNotReadyWhenDependencyIsDown(t *testing.T) {
	checker := NewChecker(
		time.Second,
		Dependency{Name: "postgres", Check: func(context.Context) error { return nil }},
		Dependency{Name: "redis", Check: func(context.Context) error { return errors.New("unavailable") }},
	)

	report := checker.Check(context.Background())

	assert.False(t, report.Ready())
	assert.Equal(t, StatusUp, report.Dependencies["postgres"])
	assert.Equal(t, StatusDown, report.Dependencies["redis"])
}

func TestChecker_EnforcesTimeout(t *testing.T) {
	checker := NewChecker(
		10*time.Millisecond,
		Dependency{
			Name: "slow",
			Check: func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			},
		},
	)

	report := checker.Check(context.Background())

	assert.False(t, report.Ready())
	assert.Equal(t, StatusDown, report.Dependencies["slow"])
}
