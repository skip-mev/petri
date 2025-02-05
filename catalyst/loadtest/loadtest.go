package loadtest

import (
	"context"

	"github.com/skip-mev/catalyst/internal/loadtest"
	"github.com/skip-mev/catalyst/internal/types"
)

// LoadTest represents a load test that can be executed
type LoadTest struct {
	runner *loadtest.Runner
}

// New creates a new load test from a specification
func New(ctx context.Context, spec types.LoadTestSpec) (*LoadTest, error) {
	runner, err := loadtest.NewRunner(ctx, spec)
	if err != nil {
		return nil, err
	}

	return &LoadTest{
		runner: runner,
	}, nil
}

// Run executes the load test and returns the results
func (lt *LoadTest) Run(ctx context.Context) (types.LoadTestResult, error) {
	results, err := lt.runner.Run(ctx)
	if err != nil {
		return results, err
	}

	lt.runner.GetCollector().PrintResults(results)

	return results, nil
}
