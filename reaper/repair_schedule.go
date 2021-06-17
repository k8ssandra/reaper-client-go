package reaper

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func (c *client) RepairSchedules(ctx context.Context) ([]RepairSchedule, error) {
	return c.fetchRepairSchedules(ctx, "/repair_schedule")
}

func (c *client) RepairSchedulesForCluster(ctx context.Context, clusterName string) ([]RepairSchedule, error) {
	path := "/repair_schedule/cluster/" + url.PathEscape(clusterName)
	return c.fetchRepairSchedules(ctx, path)
}

func (c *client) fetchRepairSchedules(ctx context.Context, path string) ([]RepairSchedule, error) {
	res, err := c.doGet(ctx, path, nil, http.StatusOK)
	if err == nil {
		repairSchedules := make([]RepairSchedule, 0)
		err = c.readBodyAsJson(res, &repairSchedules)
		if err == nil {
			return repairSchedules, nil
		}
	}
	return nil, fmt.Errorf("failed to fetch repair schedules: %w", err)
}
