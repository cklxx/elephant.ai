# ALEX 集成测试和验收框架

这是ALEX项目的完整集成测试和验收方案，包含端到端测试、性能测试、自动化测试和验收标准。

## 目录结构

```
tests/
├── README.md                 # 本文档
├── integration/              # 集成测试
│   ├── api/                 # API测试
│   ├── websocket/           # WebSocket测试
│   ├── e2e/                 # 端到端测试
│   └── session/             # 会话测试
├── performance/              # 性能测试
│   ├── load/                # 负载测试
│   ├── stress/              # 压力测试
│   ├── memory/              # 内存测试
│   └── benchmark/           # 基准测试
├── fixtures/                 # 测试数据
│   ├── configs/             # 测试配置
│   ├── sessions/            # 测试会话数据
│   └── scenarios/           # 测试场景
├── utils/                   # 测试工具
│   ├── client/              # 测试客户端
│   ├── mock/                # Mock工具
│   └── helpers/             # 辅助工具
├── reports/                 # 测试报告
│   ├── coverage/            # 覆盖率报告
│   ├── performance/         # 性能报告
│   └── acceptance/          # 验收报告
├── scripts/                 # 测试脚本
│   ├── run-integration.sh   # 集成测试脚本
│   ├── run-performance.sh   # 性能测试脚本
│   ├── run-e2e.sh          # 端到端测试脚本
│   └── generate-report.sh   # 报告生成脚本
└── config/                  # 测试配置
    ├── test.yaml            # 测试配置
    ├── ci.yaml              # CI配置
    └── performance.yaml     # 性能测试配置
```

## 快速开始

### 1. 运行基础集成测试

```bash
# 运行所有集成测试
make test-integration

# 运行特定类型测试
make test-api
make test-websocket
make test-e2e
```

### 2. 运行性能测试

```bash
# 快速性能测试
make test-performance-quick

# 完整性能测试
make test-performance-full

# 负载测试
make test-load
```

### 3. 运行端到端测试

```bash
# 完整E2E测试
make test-e2e-full

# 核心功能E2E测试
make test-e2e-core
```

### 4. 生成测试报告

```bash
# 生成完整测试报告
make test-report

# 生成性能报告
make performance-report

# 生成覆盖率报告
make coverage-report
```

## 测试类型说明

### 集成测试 (Integration Tests)

测试各组件之间的集成，包括：
- API接口测试
- WebSocket通信测试
- 数据库交互测试
- 外部服务集成测试

### 性能测试 (Performance Tests)

评估系统性能，包括：
- 负载测试：正常负载下的性能表现
- 压力测试：极限负载下的性能表现
- 内存测试：内存使用和泄漏检测
- 基准测试：与历史版本的性能对比

### 端到端测试 (E2E Tests)

完整用户流程测试，包括：
- 用户交互流程
- 完整业务场景
- 跨组件协作
- 数据流完整性

## 性能基准

### API性能基准
- 响应时间：< 100ms (95th percentile)
- 吞吐量：> 1000 requests/second
- 错误率：< 0.1%

### WebSocket性能基准
- 连接建立时间：< 50ms
- 消息延迟：< 20ms
- 并发连接数：> 1000

### 内存使用基准
- 基础内存：< 50MB
- 1000条消息内存：< 100MB
- 内存泄漏：0

### 响应时间基准
- 简单查询：< 10ms
- 复杂分析：< 500ms
- 文件处理：< 1000ms

## 验收标准

### 功能完整性
- [ ] 所有核心功能正常运行
- [ ] 所有API接口响应正确
- [ ] 会话管理功能完整
- [ ] 工具调用执行正确

### 性能要求
- [ ] 满足所有性能基准
- [ ] 无内存泄漏
- [ ] 无性能回归
- [ ] 负载下稳定运行

### 用户体验
- [ ] 界面响应流畅
- [ ] 错误处理友好
- [ ] 功能操作直观
- [ ] 文档完整准确

### 安全要求
- [ ] 输入验证完整
- [ ] 权限控制正确
- [ ] 数据传输安全
- [ ] 日志记录完整

## CI/CD 集成

本测试框架已集成到CI/CD流程中：

### Pull Request检查
- 单元测试覆盖率 > 85%
- 集成测试全部通过
- 性能测试无回归
- 代码质量检查通过

### 发布前检查
- 完整E2E测试通过
- 性能基准测试通过
- 安全扫描通过
- 文档更新完整

## 使用说明

### 环境要求
- Go 1.21+
- Node.js 18+
- Docker (可选)
- jq (JSON处理)

### 配置说明
所有测试配置在 `config/` 目录中：
- `test.yaml`: 基础测试配置
- `ci.yaml`: CI环境配置
- `performance.yaml`: 性能测试配置

### 自定义测试
可以通过以下方式自定义测试：
1. 在 `fixtures/scenarios/` 添加测试场景
2. 在 `utils/helpers/` 添加辅助工具
3. 修改配置文件调整测试参数

## 故障排除

### 常见问题
1. **测试超时**: 调整配置中的超时设置
2. **内存不足**: 增加测试环境内存配置
3. **网络问题**: 检查网络连接和代理设置
4. **依赖问题**: 运行 `make deps` 更新依赖

### 日志查看
- 测试日志：`tests/reports/logs/`
- 性能日志：`tests/reports/performance/`
- 错误日志：`tests/reports/errors/`

## 贡献指南

### 添加新测试
1. 在相应目录创建测试文件
2. 遵循现有命名规范
3. 添加必要的文档说明
4. 更新相关配置文件

### 更新基准
1. 运行当前版本测试
2. 分析性能数据
3. 更新基准配置
4. 验证新基准合理性

## 支持和反馈

如有问题或建议，请：
1. 查看本文档和项目Wiki
2. 检查已知问题列表
3. 在GitHub上提交Issue
4. 联系开发团队