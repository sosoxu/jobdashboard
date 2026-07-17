package upstream

import (
	"context"
	"time"

	"github.com/dashboard/bff/internal/model"
)

// FetchAllJobs paginates GetJob until all jobs the upstream reports have been
// consumed, returning the full slice. Used by the sampler and by the list
// service (on cache miss) to back in-memory multi-value filtering.
//
// pageSleepMs throttles between pages to avoid overwhelming the upstream.
func (c *Client) FetchAllJobs(ctx context.Context, pageSize, pageSleepMs int) ([]model.JobInfo, int, error) {
	if pageSize <= 0 {
		pageSize = 500
	}
	sleep := time.Duration(pageSleepMs) * time.Millisecond
	var all []model.JobInfo
	offset := 0
	for {
		select {
		case <-ctx.Done():
			return all, len(all), ctx.Err()
		default:
		}
		res, err := c.GetJob(ctx, GetJobParams{Offset: offset, Size: pageSize})
		if err != nil {
			return all, len(all), err
		}
		all = append(all, res.Jobs...)
		if res.Count <= 0 || offset+len(res.Jobs) >= res.Count {
			return all, len(all), nil
		}
		if len(res.Jobs) == 0 {
			return all, len(all), nil
		}
		offset += len(res.Jobs)
		if sleep > 0 {
			select {
			case <-ctx.Done():
				return all, len(all), ctx.Err()
			case <-time.After(sleep):
			}
		}
	}
}
