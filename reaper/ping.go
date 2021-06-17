package reaper

import (
	"context"
	"net/http"
)

func (c *client) IsReaperUp(ctx context.Context) (bool, error) {
	if resp, err := c.doHead(ctx, "/ping", nil); err == nil {
		return resp.StatusCode == http.StatusNoContent, nil
	} else {
		return false, err
	}
}
