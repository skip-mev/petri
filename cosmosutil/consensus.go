package cosmosutil

import (
	"context"
	rpctypes "github.com/cometbft/cometbft/rpc/core/types"
)

func (c *ChainClient) Block(ctx context.Context, height *int64) (*rpctypes.ResultBlock, error) {
	cc, err := c.Chain.GetTMClient(ctx)
	defer cc.Stop()

	if err != nil {
		return nil, err
	}

	return cc.Block(ctx, height)
}
