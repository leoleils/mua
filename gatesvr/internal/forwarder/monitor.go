package forwarder

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"mua/gatesvr/config"
)

// 限流状态响应
type RateLimitStatus struct {
	Enabled        bool                   `json:"enabled"`
	GlobalTokens   int64                  `json:"global_tokens"`
	GlobalCapacity int64                  `json:"global_capacity"`
	GlobalRate     int64                  `json:"global_rate"`
	ServiceBuckets int                    `json:"service_buckets"`
	PlayerBuckets  int                    `json:"player_buckets"`
	ServiceStats   map[string]BucketInfo  `json:"service_stats"`
	PlayerStats    map[string]BucketInfo  `json:"player_stats"`
	Timestamp      int64                  `json:"timestamp"`
	Config         config.RateLimitConfig `json:"config"`
}

// 令牌桶信息
type BucketInfo struct {
	Tokens   int64 `json:"tokens"`
	Capacity int64 `json:"capacity"`
	Rate     int64 `json:"rate"`
	LastUsed int64 `json:"last_used"`
}

// StartMonitorService 启动限流监控服务
func StartMonitorService(port int) {
	mux := http.NewServeMux()

	// 限流状态查询
	mux.HandleFunc("/rate-limit/status", handleRateLimitStatus)

	// 限流配置查询
	mux.HandleFunc("/rate-limit/config", handleRateLimitConfig)

	// 重置特定服务的限流
	mux.HandleFunc("/rate-limit/reset/service", handleResetServiceLimit)

	// 重置特定玩家的限流
	mux.HandleFunc("/rate-limit/reset/player", handleResetPlayerLimit)

	// 健康检查
	mux.HandleFunc("/health", handleHealth)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("[限流监控] 启动监控服务，端口: %d", port)
	log.Printf("[限流监控] 访问地址: http://localhost%s/rate-limit/status", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[限流监控] 服务启动失败: %v", err)
		}
	}()
}

// handleRateLimitStatus 处理限流状态查询
func handleRateLimitStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	forwarder := GetForwarder()
	if forwarder == nil {
		http.Error(w, "Forwarder not initialized", http.StatusInternalServerError)
		return
	}

	rateLimiter := forwarder.rateLimiter
	cfg := config.GetRateLimitConfig()

	// 构建响应
	status := RateLimitStatus{
		Enabled:        cfg.Enabled,
		GlobalTokens:   rateLimiter.globalBucket.getTokens(),
		GlobalCapacity: rateLimiter.globalCapacity,
		GlobalRate:     rateLimiter.globalRate,
		ServiceBuckets: len(rateLimiter.serviceBucket),
		PlayerBuckets:  len(rateLimiter.playerBucket),
		ServiceStats:   make(map[string]BucketInfo),
		PlayerStats:    make(map[string]BucketInfo),
		Timestamp:      time.Now().UnixMilli(),
		Config:         cfg,
	}

	// 获取详细统计（如果请求了详细信息）
	if r.URL.Query().Get("detail") == "true" {
		rateLimiter.mu.RLock()

		// 服务统计
		for service, bucket := range rateLimiter.serviceBucket {
			status.ServiceStats[service] = BucketInfo{
				Tokens:   bucket.getTokens(),
				Capacity: rateLimiter.serviceCapacity,
				Rate:     rateLimiter.serviceRate,
				LastUsed: bucket.lastTime.UnixMilli(),
			}
		}

		// 玩家统计（只返回前100个，避免响应过大）
		count := 0
		for player, bucket := range rateLimiter.playerBucket {
			if count >= 100 {
				break
			}
			status.PlayerStats[player] = BucketInfo{
				Tokens:   bucket.getTokens(),
				Capacity: rateLimiter.playerCapacity,
				Rate:     rateLimiter.playerRate,
				LastUsed: bucket.lastTime.UnixMilli(),
			}
			count++
		}

		rateLimiter.mu.RUnlock()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleRateLimitConfig 处理限流配置查询
func handleRateLimitConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg := config.GetRateLimitConfig()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

// handleResetServiceLimit 处理重置服务限流
func handleResetServiceLimit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	serviceName := r.URL.Query().Get("service")
	if serviceName == "" {
		http.Error(w, "Missing service parameter", http.StatusBadRequest)
		return
	}

	forwarder := GetForwarder()
	if forwarder == nil {
		http.Error(w, "Forwarder not initialized", http.StatusInternalServerError)
		return
	}

	// 重置服务限流
	forwarder.rateLimiter.mu.Lock()
	if bucket, exists := forwarder.rateLimiter.serviceBucket[serviceName]; exists {
		bucket.mu.Lock()
		bucket.tokens = bucket.capacity
		bucket.lastTime = time.Now()
		bucket.mu.Unlock()
		log.Printf("[限流监控] 重置服务限流: %s", serviceName)
	}
	forwarder.rateLimiter.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("服务 %s 限流已重置", serviceName),
	})
}

// handleResetPlayerLimit 处理重置玩家限流
func handleResetPlayerLimit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	playerID := r.URL.Query().Get("player")
	if playerID == "" {
		http.Error(w, "Missing player parameter", http.StatusBadRequest)
		return
	}

	forwarder := GetForwarder()
	if forwarder == nil {
		http.Error(w, "Forwarder not initialized", http.StatusInternalServerError)
		return
	}

	// 重置玩家限流
	forwarder.rateLimiter.mu.Lock()
	if bucket, exists := forwarder.rateLimiter.playerBucket[playerID]; exists {
		bucket.mu.Lock()
		bucket.tokens = bucket.capacity
		bucket.lastTime = time.Now()
		bucket.mu.Unlock()
		log.Printf("[限流监控] 重置玩家限流: %s", playerID)
	}
	forwarder.rateLimiter.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("玩家 %s 限流已重置", playerID),
	})
}

// handleHealth 健康检查
func handleHealth(w http.ResponseWriter, r *http.Request) {
	cfg := config.GetRateLimitConfig()
	forwarder := GetForwarder()

	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UnixMilli(),
		"rate_limit": map[string]interface{}{
			"enabled":       cfg.Enabled,
			"global_tokens": 0,
			"service_count": 0,
			"player_count":  0,
		},
	}

	if forwarder != nil && forwarder.rateLimiter != nil {
		health["rate_limit"] = map[string]interface{}{
			"enabled":       cfg.Enabled,
			"global_tokens": forwarder.rateLimiter.globalBucket.getTokens(),
			"service_count": len(forwarder.rateLimiter.serviceBucket),
			"player_count":  len(forwarder.rateLimiter.playerBucket),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// InitMonitor 初始化监控服务
func InitMonitor() {
	// 从配置文件读取监控端口，默认8081
	monitorPort := 8081

	// 这里可以扩展从配置文件读取端口
	// cfg := config.GetConfig()
	// if cfg.Monitor.Port > 0 {
	//     monitorPort = cfg.Monitor.Port
	// }

	StartMonitorService(monitorPort)
}
