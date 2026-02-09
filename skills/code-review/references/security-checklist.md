# 安全与可靠性检查清单

## 输入/输出安全

### 注入防护
- SQL/NoSQL 查询是否使用参数化？是否有字符串拼接 SQL？
- 命令行调用是否对参数做了转义？是否有 `os/exec` 拼接用户输入？
- 日志输出是否包含未过滤的用户输入（log injection）？
- GraphQL/gRPC 输入是否有深度/复杂度限制？

**危险模式**:
```go
// Bad: SQL 注入
db.Query("SELECT * FROM users WHERE name = '" + name + "'")

// Good: 参数化查询
db.Query("SELECT * FROM users WHERE name = $1", name)
```

### 路径遍历
- 文件路径操作是否验证了用户输入不包含 `../`？
- 是否使用 `filepath.Clean` + 前缀检查？

### SSRF
- 对外发起 HTTP 请求时，URL 是否来自用户输入？
- 是否限制了目标域名/IP 范围（禁止内网地址）？

---

## 认证与授权

### IDOR (不安全的直接对象引用)
- API 是否仅通过用户提供的 ID 查询资源，而未验证资源归属？
- 是否有租户隔离检查？

### 权限检查
- 敏感操作（删除、修改配置、导出数据）是否有权限校验？
- 中间件链中权限检查的顺序是否正确？

### JWT/Token
- 是否验证了 token 签名算法（防止 alg=none 攻击）？
- token 是否设置了合理的过期时间？
- refresh token 是否有单独的存储和轮换机制？

---

## 密钥与敏感数据

- 代码/配置中是否硬编码了密钥、token、密码？
- 日志中是否输出了 PII（邮箱、手机号、身份证号）或凭据？
- `.env` 文件是否在 `.gitignore` 中？
- 错误响应是否暴露了内部实现细节（堆栈、SQL、文件路径）？

---

## 竞态条件（重点检查）

### 共享状态
- 多个 goroutine/线程是否访问共享变量？是否有适当的同步？
- map 是否在并发环境中使用？Go 的 `map` 不是并发安全的。

**危险模式**:
```go
// Bad: 并发读写 map
var cache = map[string]string{}
go func() { cache["key"] = "value" }()  // 写
go func() { _ = cache["key"] }()        // 读 → panic

// Good: 使用 sync.Map 或 sync.RWMutex
var mu sync.RWMutex
mu.Lock()
cache["key"] = "value"
mu.Unlock()
```

### TOCTOU (Time-of-Check to Time-of-Use)
- 检查条件和执行操作之间是否有时间窗口被其他操作干扰？
- 文件存在性检查后再操作——文件可能已被删除。

### 数据库并发
- 读取-修改-写回是否在事务中？
- 是否需要乐观锁（version 字段）或悲观锁（SELECT FOR UPDATE）？
- 批量操作是否有死锁风险（锁定顺序不一致）？

### 分布式竞态
- 多实例部署时，是否需要分布式锁？
- 缓存失效是否有惊群效应（thundering herd）？
- 事件处理是否有幂等性保证？

---

## Go 专项

### Goroutine 安全
- goroutine 是否有退出机制（context cancel、done channel）？
- 是否有 goroutine 泄露风险（无限阻塞的 channel）？
- `defer` 是否在正确的位置（尤其是循环中的 defer）？

### Context 传播
- 长时间操作是否接受并检查 `context.Context`？
- 是否有 `context.Background()` 用在了应该传递 request context 的地方？

### Error 处理
- error 是否用 `fmt.Errorf("...: %w", err)` 包装以保留错误链？
- 是否有 `_ = someFunc()` 忽略了重要的错误？

---

## Rust 专项

### Unsafe 审查
- `unsafe` 块是否有充分的注释说明安全性不变量？
- 是否可以用 safe 替代方案？
- raw pointer 操作是否保证了对齐和生命周期？

### Send/Sync 边界
- 跨线程传递的类型是否实现了 `Send`？
- 共享引用的类型是否实现了 `Sync`？

---

## 运行时可靠性

- 是否有无界循环或无界集合增长？
- HTTP 请求/数据库连接是否设置了超时？
- 正则表达式是否有 ReDoS 风险（嵌套量词）？
- 大文件/大数据集处理是否使用流式而非全量加载？

---

## 诊断问题
- 如果这个输入来自恶意用户，会发生什么？
- 两个请求同时到达这段代码，会发生什么？
- 这个操作失败后，系统状态是否一致？
- 如果外部服务不可用，这段代码会怎样？
- 这个 goroutine 什么时候会退出？谁负责关闭它？
