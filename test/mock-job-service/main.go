package main

import (
	"flag"
	"log"
	"net/http"
	"time"
)

// mock-job-service 模拟已有的作业监控服务，用于本地测试 BFF。
//
// 用法:
//
//	go run .                       # 默认监听 :18080
//	go run . -addr :19000          # 指定端口
//	go run . -evolve 5s            # 状态演进间隔（默认 5s，设 0 关闭）
//
// 启动后将 BFF 配置 upstream.jobServiceURL 指向 http://127.0.0.1:<port>/job/call
// 即可对接。作业 attributes 填充 database/project/survey/line，与 BFF 解析逻辑一致。
//
// 额外调试端点:
//   GET /health  健康检查
//   GET /stats   内部作业统计
func main() {
	addr := flag.String("addr", ":18080", "监听地址")
	evolve := flag.Duration("evolve", 5*time.Second, "作业状态演进间隔，0 表示关闭")
	logRoot := flag.String("logroot", "", "非空时为每个作业在 {logroot}/{project}/{survey}/list|LOG 下生成对应日志文件（用于测试新命名方案）")
	flag.Parse()

	store := NewStore(*logRoot)
	if *logRoot != "" {
		log.Printf("[mock] 已启用日志文件生成，logroot=%s", *logRoot)
	}

	// 后台状态演进，模拟真实调度
	if *evolve > 0 {
		go func() {
			ticker := time.NewTicker(*evolve)
			defer ticker.Stop()
			for range ticker.C {
				store.Evolve()
			}
		}()
		log.Printf("[mock] 状态演进已启动，间隔 %s", *evolve)
	} else {
		log.Printf("[mock] 状态演进已关闭")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/job/call", handleJobCall(store))
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/stats", statsHandler(store))

	log.Printf("[mock] 作业监控服务(mock) 监听 %s", *addr)
	log.Printf("[mock] 已生成 %d 个作业", func() int {
		store.mu.RLock()
		defer store.mu.RUnlock()
		return len(store.jobs)
	}())
	log.Printf("[mock] POST /job/call  统一入口（GetCurrentJSFInfo/GetJob/Delete/Rerunmulti）")
	log.Printf("[mock] GET  /health     健康检查")
	log.Printf("[mock] GET  /stats      内部统计")
	log.Printf("[mock] 请将 BFF 配置 upstream.jobServiceURL 指向 http://127.0.0.1%s/job/call", *addr)

	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
