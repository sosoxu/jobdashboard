package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

// callRequest 镜像上游统一入口的请求体。
// 上游统一使用 "params"（复数）作为参数字段名，所有方法一致。
type callRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// callResponse 镜像上游统一响应信封
type callResponse struct {
	Count     int             `json:"count"`
	ErrorCode int             `json:"errorCode"`
	Result    any             `json:"result,omitempty"`
	JobList   any             `json:"jobList,omitempty"`
}

// getJobParams GetJob 请求参数
type getJobParams struct {
	Type            int         `json:"type"`
	Offset          int         `json:"offset"`
	Size            int         `json:"size"`
	FilterType      int         `json:"filterType"`
	JobStatus       string      `json:"jobStatus"`
	UserName        string      `json:"userName"`
	Department      string      `json:"department"`
	DescFilter      string      `json:"descFilter"`
	CommitTimeStart string      `json:"commitTimeStart"`
	CommitTimeEnd   string      `json:"commitTimeEnd"`
	StartTimeStart  string      `json:"startTimeStart"`
	StartTimeEnd    string      `json:"startTimeEnd"`
	EndTimeStart    string      `json:"endTimeStart"`
	EndTimeEnd      string      `json:"endTimeEnd"`
	ProjList        []ProjEntry `json:"projList"`
}

// nameParams Delete/Rerunmulti 请求参数
type nameParams struct {
	Type int      `json:"type"`
	Name []string `json:"name"`
}

func handleJobCall(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusOK, callResponse{ErrorCode: 1})
			return
		}
		defer r.Body.Close()

		var req callRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusOK, callResponse{ErrorCode: 1})
			return
		}

		switch req.Method {
		case "GetCurrentJSFInfo":
			handleGetCurrentJSFInfo(store, w)
		case "GetJob":
			handleGetJob(store, req.Params, w)
		case "Delete":
			handleNames(store, req.Params, w, "Delete")
		case "Rerunmulti":
			handleNames(store, req.Params, w, "Rerunmulti")
		default:
			log.Printf("[mock] unknown method: %s", req.Method)
			writeJSON(w, http.StatusOK, callResponse{ErrorCode: 1})
		}
	}
}

func handleGetCurrentJSFInfo(store *Store, w http.ResponseWriter) {
	st := store.CurrentJSFInfo()
	// 上游 result 为含单个聚合对象的数组
	writeJSON(w, http.StatusOK, callResponse{
		Count:     1,
		ErrorCode: 0,
		Result:    []JSFStats{st},
	})
	log.Printf("[mock] GetCurrentJSFInfo -> active=%d queue=%d finish=%d failed=%d canceled=%d other=%d",
		st.Active, st.Queue, st.Finish, st.Failed, st.Canceled, st.OtherCount)
}

func handleGetJob(store *Store, raw json.RawMessage, w http.ResponseWriter) {
	var p getJobParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			log.Printf("[mock] GetJob params decode error: %v", err)
			writeJSON(w, http.StatusOK, callResponse{ErrorCode: 1})
			return
		}
	}
	if p.Size <= 0 {
		p.Size = 10
	}

	jobs, total := store.GetJob(GetJobQuery{
		Offset:     p.Offset,
		Size:       p.Size,
		JobStatus:  p.JobStatus,
		UserName:   p.UserName,
		DescFilter: p.DescFilter,
		ProjList:   p.ProjList,
	})

	writeJSON(w, http.StatusOK, callResponse{
		Count:     total,
		ErrorCode: 0,
		JobList:   jobs,
	})
	log.Printf("[mock] GetJob offset=%d size=%d -> returned=%d total=%d", p.Offset, p.Size, len(jobs), total)
}

func handleNames(store *Store, raw json.RawMessage, w http.ResponseWriter, method string) {
	var p nameParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			writeJSON(w, http.StatusOK, callResponse{ErrorCode: 1})
			return
		}
	}
	var ok, fail int
	switch method {
	case "Delete":
		ok, fail = store.Delete(p.Name)
	case "Rerunmulti":
		ok, fail = store.Rerunmulti(p.Name)
	}
	writeJSON(w, http.StatusOK, callResponse{
		Count:     ok,
		ErrorCode: 0,
	})
	log.Printf("[mock] %s names=%d -> ok=%d fail=%d", method, len(p.Name), ok, fail)
}

// healthHandler 简单健康检查，便于 BFF 启动时探活
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// statsHandler 返回 mock 内部统计，便于调试
func statsHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		store.mu.RLock()
		total := len(store.jobs)
		store.mu.RUnlock()
		st := store.CurrentJSFInfo()
		writeJSON(w, http.StatusOK, map[string]any{
			"totalJobs": total,
			"jsf":       st,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
