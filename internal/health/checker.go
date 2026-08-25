package health

import (
	"context"
	"time"
)

const (
	StatusReady    = "ready"
	StatusNotReady = "not_ready"
	StatusUp       = "up"
	StatusDown     = "down"
)

type Dependency struct {
	Name  string
	Check func(context.Context) error
}

type Report struct {
	Status       string            `json:"status"`
	Dependencies map[string]string `json:"dependencies"`
}

func (r Report) Ready() bool {
	return r.Status == StatusReady
}

type Checker struct {
	timeout      time.Duration
	dependencies []Dependency
}

func NewChecker(
	timeout time.Duration,
	dependencies ...Dependency,
) *Checker {
	return &Checker{
		timeout:      timeout,
		dependencies: append([]Dependency(nil), dependencies...),
	}
}

func (c *Checker) Check(ctx context.Context) Report {
	report := Report{
		Status:       StatusReady,
		Dependencies: make(map[string]string, len(c.dependencies)),
	}

	if len(c.dependencies) == 0 {
		return report
	}

	checkContext, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	type result struct {
		name string
		err  error
	}

	results := make(chan result, len(c.dependencies))
	pending := make(map[string]struct{}, len(c.dependencies))
	for _, dependency := range c.dependencies {
		dependency := dependency
		pending[dependency.Name] = struct{}{}

		go func() {
			if dependency.Check == nil {
				results <- result{name: dependency.Name, err: context.Canceled}
				return
			}

			results <- result{
				name: dependency.Name,
				err:  dependency.Check(checkContext),
			}
		}()
	}

	for completed := 0; completed < len(c.dependencies); completed++ {
		select {
		case dependencyResult := <-results:
			delete(pending, dependencyResult.name)

			if dependencyResult.err != nil {
				report.Status = StatusNotReady
				report.Dependencies[dependencyResult.name] = StatusDown
				continue
			}

			report.Dependencies[dependencyResult.name] = StatusUp

		case <-checkContext.Done():
			report.Status = StatusNotReady
			for name := range pending {
				report.Dependencies[name] = StatusDown
			}

			return report
		}
	}

	return report
}
