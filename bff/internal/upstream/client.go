package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dashboard/bff/internal/model"
)

// Client calls the existing job monitoring service.
type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string, timeoutSec int) *Client {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
	}
}

// callResult is the common envelope returned by the upstream service.
type callResult struct {
	Count     int             `json:"count"`
	ErrorCode int             `json:"errorCode"`
	Result    json.RawMessage `json:"result"`
	JobList   []model.JobInfo `json:"jobList"`
}

// rawResponse holds the full upstream response for inspection.
type rawResponse struct {
	Count     int             `json:"count"`
	ErrorCode int             `json:"errorCode"`
	Result    json.RawMessage `json:"result"`
	JobList   json.RawMessage `json:"jobList"`
}

// GetCurrentJSFInfo returns the current aggregate job stats.
// Note: upstream uses "param" (singular) for this method.
func (c *Client) GetCurrentJSFInfo(ctx context.Context) (*model.StatsSnapshot, error) {
	body := map[string]any{
		"method": "GetCurrentJSFInfo",
		"param":  map[string]any{"type": 2},
	}
	var raw rawResponse
	if err := c.do(ctx, body, &raw); err != nil {
		return nil, err
	}
	if raw.ErrorCode != 0 {
		return nil, fmt.Errorf("upstream errorCode=%d", raw.ErrorCode)
	}
	// result is an array of one stats object.
	var stats []struct {
		Active     int `json:"active"`
		Canceled   int `json:"canceled"`
		Failed     int `json:"failed"`
		Finish     int `json:"finish"`
		OtherCount int `json:"othercount"`
		Queue      int `json:"queue"`
	}
	if err := json.Unmarshal(raw.Result, &stats); err != nil {
		return nil, fmt.Errorf("decode GetCurrentJSFInfo result: %w", err)
	}
	s := &model.StatsSnapshot{Ts: time.Now().Unix()}
	if len(stats) > 0 {
		s.Active = stats[0].Active
		s.Canceled = stats[0].Canceled
		s.Failed = stats[0].Failed
		s.Finish = stats[0].Finish
		s.OtherCount = stats[0].OtherCount
		s.Queue = stats[0].Queue
	}
	return s, nil
}

// GetJobParams holds the query parameters for GetJob.
type GetJobParams struct {
	CommitTimeStart string
	CommitTimeEnd   string
	StartTimeStart  string
	StartTimeEnd    string
	EndTimeStart    string
	EndTimeEnd      string
	JobStatus       string
	Department      string
	UserName        string
	DescFilter      string
	FilterType      int
	Offset          int
	Size            int
	Database        string
	Project         string
	Survey          string
	Line            string
}

// GetJobResult is the paginated job list response.
type GetJobResult struct {
	Count int
	Jobs  []model.JobInfo
}

// GetJob queries the job list. Note: upstream uses "params" (plural).
func (c *Client) GetJob(ctx context.Context, p GetJobParams) (*GetJobResult, error) {
	if p.Size <= 0 {
		p.Size = 10
	}
	params := map[string]any{
		"type":            2,
		"offset":          p.Offset,
		"size":            p.Size,
		"filterType":      p.FilterType,
		"jobStatus":       p.JobStatus,
		"userName":        p.UserName,
		"department":      p.Department,
		"commitTimeStart": p.CommitTimeStart,
		"commitTimeEnd":   p.CommitTimeEnd,
		"startTimeStart":  p.StartTimeStart,
		"startTimeEnd":    p.StartTimeEnd,
		"endTimeStart":    p.EndTimeStart,
		"endTimeEnd":      p.EndTimeEnd,
		"descFilter":      p.DescFilter,
		"projList": []map[string]any{
			{
				"database": p.Database,
				"project":  p.Project,
				"survey":   p.Survey,
				"line":     p.Line,
			},
		},
	}
	body := map[string]any{
		"method": "GetJob",
		"params": params,
	}
	var raw rawResponse
	if err := c.do(ctx, body, &raw); err != nil {
		return nil, err
	}
	if raw.ErrorCode != 0 {
		return nil, fmt.Errorf("upstream errorCode=%d", raw.ErrorCode)
	}
	var jobs []model.JobInfo
	if len(raw.JobList) > 0 {
		if err := json.Unmarshal(raw.JobList, &jobs); err != nil {
			return nil, fmt.Errorf("decode jobList: %w", err)
		}
	}
	return &GetJobResult{Count: raw.Count, Jobs: jobs}, nil
}

// Delete terminates jobs by name.
func (c *Client) Delete(ctx context.Context, names []string) error {
	return c.callNames(ctx, "Delete", names)
}

// Rerunmulti retries jobs by name.
func (c *Client) Rerunmulti(ctx context.Context, names []string) error {
	return c.callNames(ctx, "Rerunmulti", names)
}

func (c *Client) callNames(ctx context.Context, method string, names []string) error {
	body := map[string]any{
		"method": method,
		"params": map[string]any{
			"type": 2,
			"name": names,
		},
	}
	var raw rawResponse
	if err := c.do(ctx, body, &raw); err != nil {
		return err
	}
	if raw.ErrorCode != 0 {
		return fmt.Errorf("upstream errorCode=%d", raw.ErrorCode)
	}
	return nil
}

func (c *Client) do(ctx context.Context, body any, out *rawResponse) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("upstream HTTP %d: %s", resp.StatusCode, string(data))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode upstream response: %w", err)
	}
	return nil
}
