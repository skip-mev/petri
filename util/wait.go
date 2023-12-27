package util

import (
	"context"
	"fmt"
	"time"
)

func WaitForCondition(ctx context.Context, timeoutAfter, pollingInterval time.Duration, fn func() (bool, error)) error {
	ctx, cancel := context.WithTimeout(ctx, timeoutAfter)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed waiting for condition after %f seconds", timeoutAfter.Seconds())
		case <-time.After(pollingInterval):
			reachedCondition, err := fn()
			if err != nil {
				return fmt.Errorf("error occurred while waiting for condition: %s", err)
			}

			if reachedCondition {
				return nil
			}
		}
	}
}
