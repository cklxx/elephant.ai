# SOLID 与架构坏味道检查清单

## SOLID 原则

### SRP — 单一职责
- 一个文件/struct/class 是否承担了多个不相关的职责？
- 函数是否超过 50 行或包含多个抽象层级？
- 修改某个业务逻辑是否需要同时改动多个不相关的文件？

**Go 信号**:
- 一个 struct 方法超过 10 个
- 一个 package 同时处理 HTTP 路由和业务逻辑
- handler 函数里直接写 SQL 查询

**Rust 信号**:
- 一个 impl 块方法超过 10 个
- 模块同时处理序列化和业务逻辑

### OCP — 开闭原则
- 新增行为是否需要修改已有的 switch/if-else 链？
- 是否可以通过新增类型/接口实现扩展？

**危险模式**:
```go
// Bad: 每加一种类型就要改这里
func process(t string) {
    switch t {
    case "a": ...
    case "b": ...
    // 不断增长
    }
}

// Good: 通过接口扩展
type Processor interface { Process() error }
```

### LSP — 里氏替换
- 子类型/实现是否真的可以替换基类型/接口？
- 是否有 `if v, ok := x.(ConcreteType)` 这种类型断言来做分支？

### ISP — 接口隔离
- 接口是否包含使用者不需要的方法？
- Go 接口是否超过 3-5 个方法？（Go 偏好小接口）

**Go 信号**:
```go
// Bad: 胖接口
type Repository interface {
    Create(...) error
    Update(...) error
    Delete(...) error
    List(...) ([]T, error)
    Export(...) ([]byte, error)  // 并非所有使用者都需要
    Import(...) error            // 同上
}

// Good: 按使用场景拆分
type Reader interface { List(...) ([]T, error) }
type Writer interface { Create(...) error; Update(...) error }
```

### DIP — 依赖倒置
- 业务逻辑是否直接依赖具体实现（数据库驱动、HTTP 客户端）？
- 是否通过接口/trait 注入依赖？

---

## 常见代码坏味道

### 1. 过长函数
- 超过 50 行的函数需警惕。
- 诊断：函数内是否有明显的"段落"可以提取？

### 2. 特性嫉妒 (Feature Envy)
- 一个方法大量访问另一个 struct 的字段而非自己的。
- 建议：将逻辑移到数据所在的 struct。

### 3. 数据泥团 (Data Clumps)
- 多个函数重复传递相同的参数组合。
- 建议：提取为 struct/config 对象。

### 4. 散弹式修改 (Shotgun Surgery)
- 一个小变更需要修改多个文件的相似位置。
- 建议：抽象共同逻辑到一处。

### 5. 死代码
- 未使用的函数、变量、导入、注释掉的代码块。
- Go: `deadcode`、`unused` linter 可以检测。

### 6. 魔法数字/字符串
- 硬编码的数字或字符串，没有命名常量。
- 诊断：这个值的含义是否需要阅读上下文才能理解？

### 7. 原始类型偏执 (Primitive Obsession)
- 用 `string` 表示 email、URL、ID 等有约束的值。
- 建议：使用类型别名或 newtype 模式。

### 8. 投机泛化 (Speculative Generality)
- 为假设的未来需求引入的抽象/参数/配置。
- 原则：YAGNI — 如果当前只有一个实现，不需要接口。

### 9. 发散式变更 (Divergent Change)
- 同一个文件因为不同原因被频繁修改。
- 建议：按变更原因拆分文件/模块。

---

## 诊断问题
- 能否在不修改已有代码的情况下新增功能？
- 能否用一句话描述这个函数/模块的职责？
- 如果删除这个接口/抽象层，代码会变更简单还是更复杂？
- 新人加入团队，能否在 5 分钟内理解这个模块的边界？
