package digitalocean

import (
	"context"
	"fmt"
	"github.com/digitalocean/godo"
)

func (p *Provider) createTag(ctx context.Context, tagName string) (*godo.Tag, error) {
	req := &godo.TagCreateRequest{
		Name: tagName,
	}

	tag, res, err := p.doClient.Tags.Create(ctx, req)

	if err != nil {
		return nil, err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return tag, nil
}
