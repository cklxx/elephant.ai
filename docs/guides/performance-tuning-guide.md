# Alex Cloud Agent - 性能调优指南

## 🚀 性能优化概述

本指南提供Alex Cloud Agent在生产环境中的全面性能调优策略，涵盖系统架构、代码优化、数据库调优、缓存策略和基础设施优化等各个层面。

## 📊 性能基准与目标

### 核心性能指标
| 指标类别 | 指标名称 | 当前基线 | 优化目标 | 世界级水准 |
|----------|----------|----------|----------|------------|
| **响应性能** | API响应时间 (P95) | 200ms | <100ms | <50ms |
| | WebSocket延迟 | 100ms | <50ms | <20ms |
| | Terminal启动时间 | 5s | <2s | <1s |
| **吞吐量** | 并发用户数 | 1,000 | 10,000 | 50,000 |
| | API QPS | 5,000 | 20,000 | 100,000 |
| | 数据库TPS | 10,000 | 50,000 | 200,000 |
| **资源利用** | CPU使用率 | 70% | <60% | <40% |
| | 内存使用率 | 80% | <70% | <50% |
| | 网络带宽利用 | 60% | <50% | <30% |

## 🏗️ 架构层面优化

### 1. 微服务架构优化

#### 服务拆分策略
```yaml
service_decomposition:
  # 计算密集型服务
  compute_intensive:
    services: ["agent-core", "llm-inference"]
    optimization:
      - "独立扩缩容"
      - "GPU节点亲和性"
      - "异步处理队列"
      
  # IO密集型服务  
  io_intensive:
    services: ["session-manager", "storage-service"]
    optimization:
      - "连接池优化"
      - "批量操作"
      - "缓存层增强"
      
  # 实时通信服务
  realtime:
    services: ["terminal-executor", "websocket-gateway"]
    optimization:
      - "低延迟网络"
      - "会话亲和性"
      - "边缘部署"
```

#### 服务网格优化
```yaml
# Istio性能优化配置
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: alex-istio-performance
spec:
  values:
    pilot:
      # 减少配置推送延迟
      env:
        PILOT_PUSH_THROTTLE: 10
        PILOT_DEBOUNCE_AFTER: 100ms
        PILOT_DEBOUNCE_MAX: 10s
        
    proxy:
      # 代理性能优化
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
        limits:
          cpu: 2000m
          memory: 1Gi
          
  components:
    proxy:
      k8s:
        # 优化代理配置
        env:
        - name: PILOT_ENABLE_WORKLOAD_ENTRY_AUTO_REGISTRATION
          value: "false"
        - name: PILOT_ENABLE_CROSS_CLUSTER_WORKLOAD_ENTRY
          value: "false"
        - name: BOOTSTRAP_XDS_AGENT
          value: "true"
```

### 2. 负载均衡策略优化

#### 智能负载均衡算法
```go
// 基于负载感知的智能负载均衡器
type IntelligentLoadBalancer struct {
    // 节点健康监控
    healthMonitor   *NodeHealthMonitor
    
    // 负载预测器
    loadPredictor   *LoadPredictor
    
    // 路由策略引擎
    routingEngine   *RoutingStrategyEngine
    
    // 性能度量收集器
    metricsCollector *MetricsCollector
}

type LoadBalancingStrategy struct {
    // 基础策略
    Algorithm       string    `json:"algorithm"`        // weighted_round_robin, least_connections, ip_hash
    
    // 权重计算因子
    WeightFactors   struct {
        CPUUtilization    float64 `json:"cpu_weight"`      // 0.3
        MemoryUtilization float64 `json:"memory_weight"`   // 0.2
        ResponseTime      float64 `json:"response_weight"` // 0.3
        ActiveConnections float64 `json:"connection_weight"` // 0.2
    } `json:"weight_factors"`
    
    // 健康检查配置
    HealthCheck     struct {
        Interval        time.Duration `json:"interval"`         // 5s
        Timeout         time.Duration `json:"timeout"`          // 3s
        HealthyThreshold   int        `json:"healthy_threshold"`   // 2
        UnhealthyThreshold int        `json:"unhealthy_threshold"` // 3
    } `json:"health_check"`
}

// 动态权重计算
func (lb *IntelligentLoadBalancer) CalculateDynamicWeights(
    nodes []*ServiceNode,
) map[string]float64 {
    weights := make(map[string]float64)
    
    for _, node := range nodes {
        // 获取节点实时指标
        metrics := lb.metricsCollector.GetNodeMetrics(node.ID)
        
        // 计算综合负载分数 (越低越好)
        loadScore := 0.0
        loadScore += metrics.CPUUtilization * lb.strategy.WeightFactors.CPUUtilization
        loadScore += metrics.MemoryUtilization * lb.strategy.WeightFactors.MemoryUtilization
        loadScore += metrics.AverageResponseTime * lb.strategy.WeightFactors.ResponseTime
        loadScore += float64(metrics.ActiveConnections) / 1000 * lb.strategy.WeightFactors.ActiveConnections
        
        // 转换为权重 (负载低的节点权重高)
        weight := 1.0 / (loadScore + 0.1) // 避免除零
        weights[node.ID] = weight
        
        log.Printf("Node %s: Load Score=%.3f, Weight=%.3f", 
                   node.ID, loadScore, weight)
    }
    
    return weights
}

// 智能路由决策
func (lb *IntelligentLoadBalancer) RouteRequest(
    request *Request,
) (*ServiceNode, error) {
    // 1. 基于请求特征选择路由策略
    strategy := lb.routingEngine.SelectStrategy(request)
    
    switch strategy {
    case "session_affinity":
        // 会话亲和性路由
        return lb.routeBySessionAffinity(request)
        
    case "geographic":
        // 地理位置路由
        return lb.routeByGeography(request)
        
    case "load_balanced":
        // 负载均衡路由
        return lb.routeByLoad(request)
        
    case "resource_aware":
        // 资源感知路由
        return lb.routeByResourceRequirements(request)
        
    default:
        return lb.routeByLoad(request)
    }
}
```

## 💾 数据层优化

### 1. PostgreSQL性能调优

#### 数据库参数优化
```sql
-- postgresql.conf 优化配置
-- 内存配置
shared_buffers = '4GB'                    -- 系统内存的25%
effective_cache_size = '12GB'             -- 系统总内存的75%
work_mem = '64MB'                         -- 排序和哈希操作内存
maintenance_work_mem = '512MB'            -- 维护操作内存

-- 连接配置
max_connections = 200                     -- 最大连接数
shared_preload_libraries = 'pg_stat_statements'

-- WAL配置
wal_buffers = '64MB'                      -- WAL缓冲区
checkpoint_completion_target = 0.9        -- 检查点完成目标
max_wal_size = '4GB'                      -- 最大WAL大小
min_wal_size = '1GB'                      -- 最小WAL大小

-- 查询优化
random_page_cost = 1.1                    -- SSD存储优化
effective_io_concurrency = 200            -- 并发IO能力
default_statistics_target = 100           -- 统计信息精度

-- 并行查询
max_parallel_workers = 8                  -- 最大并行工作进程
max_parallel_workers_per_gather = 4       -- 每个Gather节点的最大并行工作进程
```

#### 索引优化策略
```sql
-- Alex核心表索引优化

-- 会话表索引
CREATE INDEX CONCURRENTLY idx_sessions_user_id_created 
    ON sessions (user_id, created_at DESC);
CREATE INDEX CONCURRENTLY idx_sessions_updated_at 
    ON sessions (updated_at DESC) WHERE status = 'active';
CREATE INDEX CONCURRENTLY idx_sessions_workspace_id 
    ON sessions (workspace_id) WHERE workspace_id IS NOT NULL;

-- 消息表索引（分区表）
CREATE INDEX CONCURRENTLY idx_messages_session_timestamp 
    ON messages (session_id, timestamp DESC);
CREATE INDEX CONCURRENTLY idx_messages_content_gin 
    ON messages USING gin(to_tsvector('english', content));

-- 用户活动索引
CREATE INDEX CONCURRENTLY idx_user_activities_user_time 
    ON user_activities (user_id, activity_time DESC);
CREATE INDEX CONCURRENTLY idx_user_activities_type_time 
    ON user_activities (activity_type, activity_time DESC);

-- 工具调用历史索引
CREATE INDEX CONCURRENTLY idx_tool_calls_session_tool 
    ON tool_calls (session_id, tool_name, created_at DESC);
    
-- 复合索引优化
CREATE INDEX CONCURRENTLY idx_sessions_composite 
    ON sessions (user_id, status, last_accessed_at DESC) 
    INCLUDE (workspace_id, created_at);
```

#### 分区表策略
```sql
-- 按时间分区的消息表
CREATE TABLE messages (
    id BIGSERIAL,
    session_id UUID NOT NULL,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    metadata JSONB,
    
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- 创建月度分区
CREATE TABLE messages_2025_01 PARTITION OF messages
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
    
CREATE TABLE messages_2025_02 PARTITION OF messages
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

-- 自动分区管理
CREATE OR REPLACE FUNCTION create_monthly_partitions()
RETURNS void AS $$
DECLARE
    start_date DATE;
    end_date DATE;
    partition_name TEXT;
BEGIN
    -- 为未来3个月创建分区
    FOR i IN 0..2 LOOP
        start_date := DATE_TRUNC('month', CURRENT_DATE + INTERVAL '1 month' * i);
        end_date := start_date + INTERVAL '1 month';
        partition_name := 'messages_' || TO_CHAR(start_date, 'YYYY_MM');
        
        -- 检查分区是否已存在
        IF NOT EXISTS (
            SELECT 1 FROM pg_tables 
            WHERE tablename = partition_name
        ) THEN
            EXECUTE format('CREATE TABLE %I PARTITION OF messages 
                           FOR VALUES FROM (%L) TO (%L)',
                          partition_name, start_date, end_date);
        END IF;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- 定期执行分区创建
SELECT cron.schedule('create-partitions', '0 0 1 * *', 'SELECT create_monthly_partitions();');
```

### 2. Redis集群优化

#### Redis配置调优
```yaml
# Redis集群性能优化配置
apiVersion: v1
kind: ConfigMap
metadata:
  name: redis-performance-config
data:
  redis.conf: |
    # 内存配置
    maxmemory 2gb
    maxmemory-policy allkeys-lru
    
    # 持久化优化
    save 900 1
    save 300 10
    save 60 10000
    
    # AOF配置
    appendonly yes
    appendfsync everysec
    no-appendfsync-on-rewrite yes
    auto-aof-rewrite-percentage 100
    auto-aof-rewrite-min-size 64mb
    
    # 网络配置
    tcp-keepalive 300
    timeout 0
    tcp-backlog 511
    
    # 性能优化
    hash-max-ziplist-entries 512
    hash-max-ziplist-value 64
    list-max-ziplist-size -2
    list-compress-depth 0
    set-max-intset-entries 512
    zset-max-ziplist-entries 128
    zset-max-ziplist-value 64
    
    # 慢查询日志
    slowlog-log-slower-than 10000
    slowlog-max-len 128
```

#### 缓存策略优化
```go
// 智能缓存管理器
type IntelligentCacheManager struct {
    // L1: 本地缓存 (FastCache)
    localCache      *fastcache.Cache
    
    // L2: Redis集群
    redisCluster    *redis.ClusterClient
    
    // L3: 分布式缓存
    distributedCache *bigcache.BigCache
    
    // 缓存策略引擎
    strategyEngine  *CacheStrategyEngine
    
    // 性能监控
    performanceMonitor *CachePerformanceMonitor
}

type CacheStrategy struct {
    // TTL策略
    DefaultTTL      time.Duration              `json:"default_ttl"`
    MaxTTL          time.Duration              `json:"max_ttl"`
    TTLByPattern    map[string]time.Duration   `json:"ttl_by_pattern"`
    
    // 淘汰策略
    EvictionPolicy  string                     `json:"eviction_policy"` // LRU, LFU, FIFO
    
    // 预热策略
    WarmupStrategy  struct {
        Enabled         bool     `json:"enabled"`
        WarmupPatterns []string `json:"patterns"`
        WarmupSchedule  string   `json:"schedule"`
    } `json:"warmup_strategy"`
    
    // 压缩策略
    CompressionStrategy struct {
        Enabled         bool   `json:"enabled"`
        Algorithm       string `json:"algorithm"`    // gzip, lz4, snappy
        MinSize         int    `json:"min_size"`     // 1KB
    } `json:"compression_strategy"`
}

// 智能缓存获取
func (c *IntelligentCacheManager) Get(
    ctx context.Context,
    key string,
) (interface{}, error) {
    startTime := time.Now()
    defer c.performanceMonitor.RecordOperation("get", time.Since(startTime))
    
    // 1. L1本地缓存查找
    if value, ok := c.localCache.Get([]byte(key)); ok {
        c.performanceMonitor.RecordCacheHit("L1", key)
        return c.deserialize(value), nil
    }
    
    // 2. L2 Redis集群查找
    value, err := c.redisCluster.Get(ctx, key).Result()
    if err == nil {
        c.performanceMonitor.RecordCacheHit("L2", key)
        
        // 回填L1缓存
        serialized := c.serialize(value)
        c.localCache.Set([]byte(key), serialized)
        
        return value, nil
    }
    
    // 3. L3分布式缓存查找
    if value, err := c.distributedCache.Get(key); err == nil {
        c.performanceMonitor.RecordCacheHit("L3", key)
        
        // 回填上层缓存
        c.redisCluster.Set(ctx, key, value, c.getTTL(key))
        serialized := c.serialize(value)
        c.localCache.Set([]byte(key), serialized)
        
        return value, nil
    }
    
    // 缓存未命中
    c.performanceMonitor.RecordCacheMiss(key)
    return nil, ErrCacheMiss
}

// 批量预热缓存
func (c *IntelligentCacheManager) WarmupCache(
    ctx context.Context,
    patterns []string,
) error {
    log.Printf("Starting cache warmup for patterns: %v", patterns)
    
    for _, pattern := range patterns {
        // 获取需要预热的键列表
        keys, err := c.getKeysForWarmup(pattern)
        if err != nil {
            log.Printf("Failed to get keys for pattern %s: %v", pattern, err)
            continue
        }
        
        // 并发预热
        semaphore := make(chan struct{}, 10) // 限制并发数
        var wg sync.WaitGroup
        
        for _, key := range keys {
            wg.Add(1)
            go func(k string) {
                defer wg.Done()
                semaphore <- struct{}{}
                defer func() { <-semaphore }()
                
                if err := c.warmupSingleKey(ctx, k); err != nil {
                    log.Printf("Failed to warmup key %s: %v", k, err)
                }
            }(key)
        }
        
        wg.Wait()
        log.Printf("Completed warmup for pattern: %s", pattern)
    }
    
    return nil
}
```

## 🖥️ 应用层优化

### 1. Go服务性能优化

#### 内存管理优化
```go
// 内存池管理
type MemoryPoolManager struct {
    // 不同大小的内存池
    smallPool   *sync.Pool // <1KB
    mediumPool  *sync.Pool // 1KB-10KB  
    largePool   *sync.Pool // >10KB
    
    // 内存使用监控
    memoryMonitor *MemoryUsageMonitor
}

func NewMemoryPoolManager() *MemoryPoolManager {
    return &MemoryPoolManager{
        smallPool: &sync.Pool{
            New: func() interface{} {
                return make([]byte, 1024)
            },
        },
        mediumPool: &sync.Pool{
            New: func() interface{} {
                return make([]byte, 10*1024)
            },
        },
        largePool: &sync.Pool{
            New: func() interface{} {
                return make([]byte, 100*1024)
            },
        },
        memoryMonitor: NewMemoryUsageMonitor(),
    }
}

// 智能缓冲区分配
func (m *MemoryPoolManager) GetBuffer(size int) []byte {
    m.memoryMonitor.RecordAllocation(size)
    
    switch {
    case size <= 1024:
        buffer := m.smallPool.Get().([]byte)
        return buffer[:size]
    case size <= 10*1024:
        buffer := m.mediumPool.Get().([]byte)
        return buffer[:size]
    default:
        buffer := m.largePool.Get().([]byte)
        if len(buffer) < size {
            // 需要更大的缓冲区
            return make([]byte, size)
        }
        return buffer[:size]
    }
}

func (m *MemoryPoolManager) PutBuffer(buffer []byte) {
    m.memoryMonitor.RecordDeallocation(len(buffer))
    
    capacity := cap(buffer)
    switch {
    case capacity <= 1024:
        m.smallPool.Put(buffer[:1024])
    case capacity <= 10*1024:
        m.mediumPool.Put(buffer[:10*1024])
    case capacity <= 100*1024:
        m.largePool.Put(buffer[:100*1024])
    // 太大的缓冲区直接丢弃，让GC回收
    }
}
```

#### 并发处理优化
```go
// 智能工作池
type IntelligentWorkerPool struct {
    // 工作队列
    taskQueue       chan Task
    
    // 工作协程
    workers         []*Worker
    
    // 动态扩缩容控制
    minWorkers      int
    maxWorkers      int
    currentWorkers  int
    
    // 负载监控
    loadMonitor     *LoadMonitor
    
    // 性能指标
    metrics         *WorkerPoolMetrics
    
    // 控制通道
    scaleUp         chan struct{}
    scaleDown       chan struct{}
    shutdown        chan struct{}
}

func NewIntelligentWorkerPool(minWorkers, maxWorkers int) *IntelligentWorkerPool {
    pool := &IntelligentWorkerPool{
        taskQueue:      make(chan Task, maxWorkers*10),
        workers:        make([]*Worker, 0, maxWorkers),
        minWorkers:     minWorkers,
        maxWorkers:     maxWorkers,
        currentWorkers: minWorkers,
        loadMonitor:    NewLoadMonitor(),
        metrics:        NewWorkerPoolMetrics(),
        scaleUp:        make(chan struct{}, 1),
        scaleDown:      make(chan struct{}, 1),
        shutdown:       make(chan struct{}),
    }
    
    // 启动初始工作协程
    for i := 0; i < minWorkers; i++ {
        pool.addWorker()
    }
    
    // 启动负载监控和自动扩缩容
    go pool.monitorAndScale()
    
    return pool
}

// 自动扩缩容逻辑
func (p *IntelligentWorkerPool) monitorAndScale() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            p.evaluateScaling()
            
        case <-p.scaleUp:
            if p.currentWorkers < p.maxWorkers {
                p.addWorker()
                log.Printf("Scaled up workers to: %d", p.currentWorkers)
            }
            
        case <-p.scaleDown:
            if p.currentWorkers > p.minWorkers {
                p.removeWorker()
                log.Printf("Scaled down workers to: %d", p.currentWorkers)
            }
            
        case <-p.shutdown:
            return
        }
    }
}

func (p *IntelligentWorkerPool) evaluateScaling() {
    metrics := p.loadMonitor.GetCurrentMetrics()
    
    // 扩容条件：队列长度 > 工作协程数 * 2 且 CPU使用率 < 80%
    if len(p.taskQueue) > p.currentWorkers*2 && metrics.CPUUsage < 0.8 {
        select {
        case p.scaleUp <- struct{}{}:
        default:
        }
        return
    }
    
    // 缩容条件：队列长度 < 工作协程数 / 4 且最近1分钟平均负载 < 30%
    if len(p.taskQueue) < p.currentWorkers/4 && metrics.AverageLoad1Min < 0.3 {
        select {
        case p.scaleDown <- struct{}{}:
        default:
        }
    }
}

// 异步任务处理优化
func (p *IntelligentWorkerPool) SubmitTask(task Task) error {
    select {
    case p.taskQueue <- task:
        p.metrics.RecordTaskSubmitted()
        return nil
    case <-time.After(100 * time.Millisecond):
        p.metrics.RecordTaskRejected()
        return ErrTaskQueueFull
    }
}
```

### 2. HTTP服务优化

#### 连接池与Keep-Alive配置
```go
// 优化的HTTP客户端
func NewOptimizedHTTPClient() *http.Client {
    transport := &http.Transport{
        // 连接池配置
        MaxIdleConns:        100,              // 最大空闲连接数
        MaxIdleConnsPerHost: 20,               // 每个主机最大空闲连接数
        MaxConnsPerHost:     50,               // 每个主机最大连接数
        
        // 超时配置
        IdleConnTimeout:       90 * time.Second,
        TLSHandshakeTimeout:   10 * time.Second,
        ResponseHeaderTimeout: 10 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,
        
        // Keep-Alive配置
        DisableKeepAlives:   false,
        DisableCompression: false,
        
        // TCP配置优化
        DialContext: (&net.Dialer{
            Timeout:   30 * time.Second,
            KeepAlive: 30 * time.Second,
        }).DialContext,
    }
    
    return &http.Client{
        Transport: transport,
        Timeout:   30 * time.Second,
    }
}

// HTTP服务器优化配置
func NewOptimizedHTTPServer(handler http.Handler) *http.Server {
    return &http.Server{
        Handler: handler,
        
        // 超时配置
        ReadTimeout:    10 * time.Second,
        WriteTimeout:   10 * time.Second,
        IdleTimeout:    60 * time.Second,
        MaxHeaderBytes: 1 << 20, // 1MB
        
        // 连接数限制通过中间件实现
    }
}
```

#### 响应压缩与缓存
```go
// 智能响应压缩中间件
func IntelligentCompressionMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 检查客户端支持的压缩格式
        acceptEncoding := r.Header.Get("Accept-Encoding")
        
        var compressor io.WriteCloser
        var encoding string
        
        switch {
        case strings.Contains(acceptEncoding, "br"):
            // Brotli压缩 (最佳压缩率)
            compressor = brotli.NewWriter(w)
            encoding = "br"
        case strings.Contains(acceptEncoding, "gzip"):
            // Gzip压缩 (通用支持)
            compressor = gzip.NewWriter(w)
            encoding = "gzip"
        default:
            // 不压缩
            next.ServeHTTP(w, r)
            return
        }
        
        defer compressor.Close()
        
        w.Header().Set("Content-Encoding", encoding)
        w.Header().Del("Content-Length") // 压缩后长度会变化
        
        // 包装响应写入器
        compressedWriter := &CompressedResponseWriter{
            ResponseWriter: w,
            Writer:        compressor,
        }
        
        next.ServeHTTP(compressedWriter, r)
    })
}

// HTTP缓存优化
func CacheOptimizationMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 静态资源缓存策略
        if isStaticResource(r.URL.Path) {
            w.Header().Set("Cache-Control", "public, max-age=31536000") // 1年
            w.Header().Set("ETag", generateETag(r.URL.Path))
            
            // 检查If-None-Match头
            if clientETag := r.Header.Get("If-None-Match"); clientETag != "" {
                if clientETag == generateETag(r.URL.Path) {
                    w.WriteHeader(http.StatusNotModified)
                    return
                }
            }
        }
        
        // API响应缓存策略
        if isAPIResponse(r.URL.Path) {
            w.Header().Set("Cache-Control", "private, max-age=300") // 5分钟
        }
        
        next.ServeHTTP(w, r)
    })
}
```

## 🌐 网络层优化

### 1. CDN配置优化

#### CloudFlare配置
```yaml
# Terraform CDN配置
resource "cloudflare_zone" "alex_domain" {
  zone = "alex-cloud.com"
}

resource "cloudflare_zone_settings_override" "alex_settings" {
  zone_id = cloudflare_zone.alex_domain.id
  
  settings {
    # 性能优化
    brotli                = "on"
    early_hints          = "on"
    h2_prioritization    = "on"
    http2                = "on"
    http3                = "on"
    
    # 缓存优化
    browser_cache_ttl    = 31536000  # 1年
    cache_level          = "aggressive"
    development_mode     = "off"
    
    # 安全优化
    always_use_https     = "on"
    automatic_https_rewrites = "on"
    ssl                  = "strict"
    min_tls_version      = "1.2"
    
    # 网络优化
    ipv6                 = "on"
    websockets          = "on"
    pseudo_ipv4         = "add_header"
  }
}

# 页面规则优化
resource "cloudflare_page_rule" "api_cache" {
  zone_id = cloudflare_zone.alex_domain.id
  target  = "alex-cloud.com/api/v1/static/*"
  
  actions {
    cache_level       = "cache_everything"
    edge_cache_ttl    = 86400  # 24小时
    browser_cache_ttl = 86400
  }
}
```

### 2. DNS优化

#### 地理位置DNS配置
```yaml
# Route53 地理位置路由
apiVersion: v1
kind: ConfigMap
metadata:
  name: dns-geolocation-config
data:
  route53-config.yaml: |
    dns_records:
      - name: "api.alex-cloud.com"
        type: "A"
        geolocation_routing:
          - location: "US-EAST-1"
            value: "52.1.1.1"
            health_check: true
          - location: "EU-WEST-1" 
            value: "54.2.2.2"
            health_check: true
          - location: "AP-SOUTHEAST-1"
            value: "56.3.3.3"
            health_check: true
        
      - name: "ws.alex-cloud.com"
        type: "A"  
        latency_routing:
          - region: "us-east-1"
            value: "52.1.1.100"
          - region: "eu-west-1"
            value: "54.2.2.100"
          - region: "ap-southeast-1"
            value: "56.3.3.100"
```

## 📊 监控与分析

### 1. 性能监控体系

#### 应用性能监控 (APM)
```yaml
# OpenTelemetry配置
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-performance-config
data:
  otel-config.yaml: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318
            
    processors:
      batch:
        timeout: 1s
        send_batch_size: 1024
      
      memory_limiter:
        check_interval: 1s
        limit_mib: 512
        
      probabilistic_sampler:
        hash_seed: 22
        sampling_percentage: 10
        
    exporters:
      jaeger:
        endpoint: jaeger:14250
        tls:
          insecure: true
          
      prometheus:
        endpoint: "0.0.0.0:8889"
        
    service:
      pipelines:
        traces:
          receivers: [otlp]
          processors: [memory_limiter, batch, probabilistic_sampler]
          exporters: [jaeger]
          
        metrics:
          receivers: [otlp]
          processors: [memory_limiter, batch]
          exporters: [prometheus]
```

#### 自定义性能指标
```go
// 性能指标收集器
type PerformanceMetricsCollector struct {
    // Prometheus指标
    httpRequestDuration    *prometheus.HistogramVec
    httpRequestsTotal      *prometheus.CounterVec
    activeConnections      prometheus.Gauge
    goroutineCount        prometheus.Gauge
    memoryUsage           prometheus.Gauge
    
    // 业务指标
    sessionCreationTime   *prometheus.HistogramVec
    llmResponseTime       *prometheus.HistogramVec
    terminalStartupTime   *prometheus.HistogramVec
    
    // 系统指标
    cpuUsage              prometheus.Gauge
    memoryUtilization     prometheus.Gauge
    diskIO                *prometheus.CounterVec
    networkIO             *prometheus.CounterVec
}

func NewPerformanceMetricsCollector() *PerformanceMetricsCollector {
    return &PerformanceMetricsCollector{
        httpRequestDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "alex_http_request_duration_seconds",
                Help:    "Duration of HTTP requests",
                Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
            },
            []string{"method", "endpoint", "status_code"},
        ),
        
        sessionCreationTime: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "alex_session_creation_duration_seconds",
                Help:    "Time taken to create a new session",
                Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
            },
            []string{"user_type"},
        ),
        
        llmResponseTime: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "alex_llm_response_duration_seconds", 
                Help:    "Time taken for LLM to respond",
                Buckets: []float64{0.5, 1, 2, 5, 10, 20, 60},
            },
            []string{"model", "task_type"},
        ),
        
        // ... 其他指标初始化
    }
}

// 性能数据收集
func (p *PerformanceMetricsCollector) CollectSystemMetrics() {
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        // CPU使用率
        if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
            p.cpuUsage.Set(cpuPercent[0])
        }
        
        // 内存使用率
        if memInfo, err := mem.VirtualMemory(); err == nil {
            p.memoryUtilization.Set(memInfo.UsedPercent)
            p.memoryUsage.Set(float64(memInfo.Used))
        }
        
        // Goroutine数量
        p.goroutineCount.Set(float64(runtime.NumGoroutine()))
        
        // 网络IO统计
        if netStats, err := net.IOCounters(true); err == nil {
            for _, stat := range netStats {
                p.networkIO.WithLabelValues(stat.Name, "bytes_sent").Add(float64(stat.BytesSent))
                p.networkIO.WithLabelValues(stat.Name, "bytes_recv").Add(float64(stat.BytesRecv))
            }
        }
    }
}
```

### 2. 性能分析工具

#### Go pprof性能分析
```go
// 性能分析端点
func setupPprofEndpoints(mux *http.ServeMux) {
    // CPU分析
    mux.HandleFunc("/debug/pprof/", pprof.Index)
    mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
    mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
    mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
    mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
    
    // 自定义分析端点
    mux.HandleFunc("/debug/performance/goroutines", analyzeGoroutines)
    mux.HandleFunc("/debug/performance/memory", analyzeMemoryUsage)
    mux.HandleFunc("/debug/performance/slow-queries", analyzeSlowQueries)
}

// 内存使用分析
func analyzeMemoryUsage(w http.ResponseWriter, r *http.Request) {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    analysis := struct {
        AllocatedMemory    uint64 `json:"allocated_memory_bytes"`
        TotalAllocations   uint64 `json:"total_allocations"`
        GCCycles          uint32 `json:"gc_cycles"`
        NextGCThreshold   uint64 `json:"next_gc_threshold_bytes"`
        HeapSize          uint64 `json:"heap_size_bytes"`
        StackSize         uint64 `json:"stack_size_bytes"`
        
        // 内存使用建议
        Recommendations   []string `json:"recommendations"`
    }{
        AllocatedMemory:   m.Alloc,
        TotalAllocations:  m.TotalAlloc,
        GCCycles:         m.NumGC,
        NextGCThreshold:  m.NextGC,
        HeapSize:         m.HeapAlloc,
        StackSize:        m.StackInuse,
    }
    
    // 生成优化建议
    if analysis.AllocatedMemory > 512*1024*1024 { // >512MB
        analysis.Recommendations = append(analysis.Recommendations,
            "内存使用量过高，考虑增加对象复用和内存池")
    }
    
    if analysis.GCCycles > 1000 {
        analysis.Recommendations = append(analysis.Recommendations,
            "GC频率过高，检查是否有内存泄漏或频繁的小对象分配")
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(analysis)
}
```

## 📋 性能优化检查清单

### 应用层优化
- [ ] 实现连接池和对象池
- [ ] 优化序列化/反序列化
- [ ] 启用HTTP/2和压缩
- [ ] 实现智能缓存策略
- [ ] 优化数据库查询
- [ ] 实现异步处理
- [ ] 配置合适的超时设置
- [ ] 优化内存使用模式

### 系统层优化
- [ ] 调优Kubernetes资源配置
- [ ] 优化容器镜像大小
- [ ] 配置节点亲和性
- [ ] 实现智能负载均衡
- [ ] 优化网络配置
- [ ] 配置适当的存储类型
- [ ] 启用CPU和内存优化特性

### 数据库优化
- [ ] 创建合适的索引
- [ ] 优化查询语句
- [ ] 配置连接池参数
- [ ] 实现读写分离
- [ ] 配置分区表
- [ ] 优化数据库参数
- [ ] 实现查询缓存

### 监控与调优
- [ ] 配置全面的性能监控
- [ ] 建立性能基准测试
- [ ] 实现自动告警
- [ ] 定期性能分析
- [ ] 建立容量规划模型
- [ ] 实施持续优化流程

---

**性能调优指南版本**: v1.0  
**适用环境**: 生产环境  
**更新时间**: 2025-01-27  
**维护团队**: Alex Cloud性能优化团队