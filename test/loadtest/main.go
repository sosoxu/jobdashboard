// BFF 后端接口压力测试工具
//
// 用法：go run . -base http://127.0.0.1:18088 -user admin -pass admin123
//
// 压测流程：
//  1. 登录获取 token
//  2. 拉取作业列表获取一个有效 jobName
//  3. 对各只读接口在指定并发数下压测指定时长
//  4. 汇总输出 QPS / 延迟分位数 / 错误率
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var (
	baseURL  string
	username string
	password string
	duration time.Duration
	conns    int // 并发数列表
)

func main() {
	flag.StringVar(&baseURL, "base", "http://127.0.0.1:18088", "BFF 基地址")
	flag.StringVar(&username, "user", "admin", "登录用户名")
	flag.StringVar(&password, "pass", "admin123", "登录密码")
	flag.DurationVar(&duration, "dur", 10*time.Second, "每个接口压测时长")
	flag.IntVar(&conns, "c", 50, "并发数")
	flag.Parse()

	fmt.Printf("=== BFF 压测 ===\n基地址: %s\n并发: %d  时长: %s\n\n", baseURL, conns, duration)

	// 1. 登录
	token, err := login()
	if err != nil {
		fmt.Println("登录失败:", err)
		os.Exit(1)
	}
	fmt.Println("登录成功，token 长度:", len(token))

	// 2. 获取有效 jobName
	jobName, err := getJobName(token)
	if err != nil {
		fmt.Println("获取 jobName 失败:", err)
		os.Exit(1)
	}
	fmt.Println("测试 jobName:", jobName)
	fmt.Println()

	// 3. 定义压测目标
	targets := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"healthz", "GET", "/healthz", ""},
		{"dashboard/stats", "GET", "/api/v1/dashboard/stats", ""},
		{"dashboard/trend", "GET", "/api/v1/dashboard/trend?hours=24", ""},
		{"dashboard/top-users", "GET", "/api/v1/dashboard/top-users?limit=10", ""},
		{"jobs(全量)", "GET", "/api/v1/jobs?page=1&pageSize=20", ""},
		{"jobs(仅我的)", "GET", "/api/v1/jobs?page=1&pageSize=20&onlyMine=1", ""},
		{"jobs/filters", "GET", "/api/v1/jobs/filters", ""},
		{"jobs/logs(list)", "GET", fmt.Sprintf("/api/v1/jobs/%s/logs?type=list&page=1&pageSize=200", jobName), ""},
		{"jobs/logs(log)", "GET", fmt.Sprintf("/api/v1/jobs/%s/logs?type=log&page=1&pageSize=200", jobName), ""},
		{"auth/login", "POST", "/api/v1/auth/login", fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password)},
	}

	// 4. 逐个压测
	fmt.Printf("%-22s %8s %8s %8s %8s %8s %8s %6s\n",
		"接口", "QPS", "Avg(ms)", "P50(ms)", "P95(ms)", "P99(ms)", "Max(ms)", "Err%")
	fmt.Println("---")

	for _, t := range targets {
		runBenchmark(t.name, t.method, t.path, t.body, token)
	}

	fmt.Println()
	fmt.Println("=== 压测完成 ===")
}

// runBenchmark 对单个接口压测并输出结果。
func runBenchmark(name, method, path, body, token string) {
	url := baseURL + path
	var reqBody io.Reader
	if body != "" {
		reqBody = bytes.NewBufferString(body)
	}

	// 预热：发一个请求确保缓存就绪
	warmup(url, method, reqBody, token)

	var (
		okCount    int64
		errCount   int64
		totalMS    int64
		latencies  = make([]int64, 0, 4096)
		latMu      sync.Mutex
		wg         sync.WaitGroup
		stop       = make(chan struct{})
	)

	// 并发 worker
	for i := 0; i < conns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				// 每次新建 body reader（POST 场景）
				var rb io.Reader
				if body != "" {
					rb = bytes.NewBufferString(body)
				}
				start := time.Now()
				req, _ := http.NewRequest(method, url, rb)
				if token != "" && path != "/healthz" && path != "/api/v1/auth/login" {
					req.Header.Set("Authorization", "Bearer "+token)
				}
				if method == "POST" {
					req.Header.Set("Content-Type", "application/json")
				}
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					atomic.AddInt64(&errCount, 1)
					continue
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				elapsed := time.Since(start).Milliseconds()
				atomic.AddInt64(&totalMS, elapsed)
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					atomic.AddInt64(&okCount, 1)
				} else {
					atomic.AddInt64(&errCount, 1)
				}
				latMu.Lock()
				latencies = append(latencies, elapsed)
				latMu.Unlock()
			}
		}()
	}

	time.Sleep(duration)
	close(stop)
	wg.Wait()

	total := okCount + errCount
	if total == 0 {
		fmt.Printf("%-22s  无请求\n", name)
		return
	}

	qps := float64(total) / duration.Seconds()
	avg := float64(totalMS) / float64(total)
	errPct := float64(errCount) / float64(total) * 100

	// 排序计算分位数
	latMu.Lock()
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := percentile(latencies, 50)
	p95 := percentile(latencies, 95)
	p99 := percentile(latencies, 99)
	maxLat := latencies[len(latencies)-1]
	latMu.Unlock()

	fmt.Printf("%-22s %8.1f %8.1f %8d %8d %8d %8d %5.1f%%\n",
		name, qps, avg, p50, p95, p99, maxLat, errPct)
}

func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)) * p / 100)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func warmup(url, method string, body io.Reader, token string) {
	req, _ := http.NewRequest(method, url, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func login() (string, error) {
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password)
	resp, err := http.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		// 尝试注册
		regBody := fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password)
		regResp, err := http.Post(baseURL+"/api/v1/auth/register", "application/json", bytes.NewBufferString(regBody))
		if err != nil {
			return "", err
		}
		regResp.Body.Close()
		// 重新登录
		resp, err = http.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(body))
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
	}
	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Data.Token == "" {
		return "", fmt.Errorf("token 为空")
	}
	return result.Data.Token, nil
}

func getJobName(token string) (string, error) {
	req, _ := http.NewRequest("GET", baseURL+"/api/v1/jobs?page=1&pageSize=5", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		Data struct {
			List []struct {
				JobName string `json:"jobName"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Data.List) == 0 {
		return "", fmt.Errorf("作业列表为空")
	}
	return result.Data.List[0].JobName, nil
}
