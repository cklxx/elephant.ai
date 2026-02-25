# 代码质量检查清单

## 错误处理

### 吞掉的错误
- 是否有 `catch` / `recover` 不做任何处理就返回？
- 是否有 `if err != nil { return nil }` 丢失了错误信息？

**危险模式**:
```go
// Bad: 吞掉错误
result, err := doSomething()
if err != nil {
    return nil  // 调用者不知道失败了
}

// Good: 传播错误
result, err := doSomething()
if err != nil {
    return fmt.Errorf("doSomething failed: %w", err)
}
```

### 过宽的错误捕获
- `recover()` 是否捕获了不应该恢复的 panic？
- 是否有 `catch (Exception e)` 这种过宽的捕获？

### 错误上下文
- 错误信息是否包含足够的上下文（操作、参数、状态）？
- 错误链是否完整（用 `%w` 包装而非 `%v`）？

---

## 性能

### N+1 查询
- 循环中是否有数据库查询？是否可以用批量查询替代？
- ORM 关联加载是否导致了 N+1？

**危险模式**:
```go
// Bad: N+1
for _, user := range users {
    orders, _ := db.GetOrdersByUserID(user.ID)  // N 次查询
}

// Good: 批量查询
userIDs := extractIDs(users)
orders, _ := db.GetOrdersByUserIDs(userIDs)  // 1 次查询
```

### 热路径优化
- 高频调用路径上是否有不必要的内存分配？
- 是否在循环中做了可以提到循环外的计算？
- Go: 是否在热路径上使用了反射（`reflect` 包）？

### 缓存
- 重复计算/查询是否有缓存？
- 缓存是否设置了 TTL 和大小上限？
- 缓存失效策略是否正确？
- 缓存 key 是否有碰撞风险？

### 内存
- 是否有无界集合（map/slice 无限增长）？
- 大对象是否及时释放？
- Go: 是否有 slice 引用导致底层大数组无法 GC？
- 字符串拼接是否在循环中使用了 `strings.Builder`？

---

## 边界条件

### Nil / Zero-value
- 是否检查了可能为 nil 的指针/引用？
- Go struct 的 zero-value 是否是合理的默认值？
- 链式调用中间步骤返回 nil 会怎样？

**危险模式**:
```go
// Bad: 未检查 nil
func getName(u *User) string {
    return u.Profile.Name  // u 或 Profile 为 nil → panic
}

// Good: 安全访问
func getName(u *User) string {
    if u == nil || u.Profile == nil {
        return ""
    }
    return u.Profile.Name
}
```

### 空集合
- 对 slice/array 的首元素访问是否先检查长度？
- `range` 空 slice 是安全的，但 `slice[0]` 不是。
- map 查找是否检查了 key 不存在的情况？

### 数值边界
- 是否有除零风险？
- 整数运算是否有溢出可能？
- 浮点数比较是否使用了 epsilon 而非 `==`？
- 循环索引是否有 off-by-one？

### 字符串边界
- 是否处理了空字符串和纯空白字符串？
- 是否考虑了超长字符串？
- Unicode/多字节字符是否正确处理（Go `len` vs `utf8.RuneCountInString`）？

---

## 可观测性

### 日志
- 关键操作（创建、删除、权限变更）是否有结构化日志？
- 错误日志是否包含请求 ID / trace ID？
- 日志级别是否合理（不要把 info 级别的日志放在热路径上）？

### Metrics
- 关键路径是否有延迟/吞吐量 metric？
- 错误率是否有 metric 监控？
- 是否有资源使用量 metric（连接池、队列深度）？

### Tracing
- 跨服务调用是否传播了 trace context？
- span 名称是否有意义？

---

## 诊断问题
- 这个操作失败时，调用者能得到足够的信息来排查吗？
- 数据量增长 10x/100x 时，这段代码的表现如何？
- 这个 nil/零值会在什么场景下出现？
- 这个 metric/log 能帮助我在凌晨 3 点排查线上问题吗？
