# ALEX 详细架构图谱

## 🎯 终极架构全景图

```mermaid
graph TD
    %% === 用户交互层 ===
    USER[👤 用户]
    CLI[🚀 Cobra CLI]
    TUI[🖥️ Bubble Tea TUI]
    
    %% === 核心控制层 ===
    AGENT[🤖 ReactAgent]
    CORE[⚙️ ReactCore]
    ENGINE[🔄 ReAct引擎]
    
    %% === ReAct循环组件 ===
    THINK[🤔 Think Phase]
    ACT[🎬 Act Phase] 
    OBSERVE[👀 Observe Phase]
    
    %% === LLM抽象层 ===
    LLM_FACTORY[🏭 LLM工厂]
    BASIC_MODEL[🧠 基础模型<br/>DeepSeek Chat]
    REASON_MODEL[🔬 推理模型<br/>DeepSeek R1]
    OPENROUTER[🛣️ OpenRouter API]
    
    %% === 工具生态系统 ===
    TOOL_REGISTRY[📋 工具注册表]
    
    %% 内置工具组
    BUILTIN_TOOLS[⚙️ 内置工具集]
    FILE_TOOLS[📁 文件工具<br/>read/update/replace/list]
    SHELL_TOOLS[🐚 Shell工具<br/>bash/code_execute]
    SEARCH_TOOLS[🔍 搜索工具<br/>grep/ripgrep/find]
    TODO_TOOLS[📝 任务工具<br/>todo_read/update]
    WEB_TOOLS[🌐 Web工具<br/>web_search]
    THINK_TOOLS[🤔 推理工具<br/>think]
    
    %% MCP工具组  
    MCP_SYSTEM[🔌 MCP协议系统]
    MCP_CLIENT[📡 MCP客户端]
    MCP_TRANSPORT[🚛 传输层]
    STDIO_TRANSPORT[📟 STDIO传输]
    SSE_TRANSPORT[📡 SSE传输]
    MCP_PROTOCOL[📋 JSON-RPC 2.0]
    EXTERNAL_TOOLS[🧩 外部工具]
    
    %% === 数据管理层 ===
    SESSION_MGR[💾 会话管理器]
    SESSION_FILES[📄 会话文件<br/>~/.alex-sessions/]
    
    CONTEXT_MGR[📝 上下文管理器] 
    MSG_PROCESSOR[⚡ 消息处理器]
    COMPRESSOR[🗜️ 上下文压缩器]
    
    CONFIG_MGR[⚙️ 配置管理器]
    CONFIG_FILE[📋 配置文件<br/>~/.alex-config.json]
    ENV_VARS[🌍 环境变量]
    
    PROMPT_MGR[📋 提示管理器]
    PROMPT_TEMPLATES[📝 提示模板<br/>initial.md/coder.md/enhanced_coder.md]
    
    %% === 评估与性能层 ===
    SWE_BENCH[📈 SWE-Bench评估]
    PERFORMANCE[⚡ 性能监控]
    BATCH_PROCESSOR[🔄 批处理器]
    
    %% === 流式回调系统 ===
    STREAM_CALLBACK[📡 流式回调]
    MESSAGE_QUEUE[📬 消息队列]
    
    %% ================== 连接关系 ==================
    
    %% 用户交互流
    USER --> CLI
    CLI --> TUI
    CLI --> AGENT
    
    %% 核心控制流
    AGENT --> CORE
    CORE --> ENGINE
    ENGINE --> THINK
    THINK --> ACT
    ACT --> OBSERVE
    OBSERVE --> THINK
    
    %% LLM交互流
    THINK --> LLM_FACTORY
    LLM_FACTORY --> BASIC_MODEL
    LLM_FACTORY --> REASON_MODEL
    BASIC_MODEL --> OPENROUTER
    REASON_MODEL --> OPENROUTER
    
    %% 工具调用流
    ACT --> TOOL_REGISTRY
    TOOL_REGISTRY --> BUILTIN_TOOLS
    TOOL_REGISTRY --> MCP_SYSTEM
    
    %% 内置工具展开
    BUILTIN_TOOLS --> FILE_TOOLS
    BUILTIN_TOOLS --> SHELL_TOOLS
    BUILTIN_TOOLS --> SEARCH_TOOLS
    BUILTIN_TOOLS --> TODO_TOOLS
    BUILTIN_TOOLS --> WEB_TOOLS
    BUILTIN_TOOLS --> THINK_TOOLS
    
    %% MCP系统展开
    MCP_SYSTEM --> MCP_CLIENT
    MCP_CLIENT --> MCP_TRANSPORT
    MCP_TRANSPORT --> STDIO_TRANSPORT
    MCP_TRANSPORT --> SSE_TRANSPORT
    MCP_CLIENT --> MCP_PROTOCOL
    MCP_CLIENT --> EXTERNAL_TOOLS
    
    %% 数据管理流
    AGENT --> SESSION_MGR
    SESSION_MGR --> SESSION_FILES
    
    AGENT --> CONTEXT_MGR
    CONTEXT_MGR --> MSG_PROCESSOR
    MSG_PROCESSOR --> COMPRESSOR
    
    AGENT --> CONFIG_MGR
    CONFIG_MGR --> CONFIG_FILE
    CONFIG_MGR --> ENV_VARS
    
    AGENT --> PROMPT_MGR
    PROMPT_MGR --> PROMPT_TEMPLATES
    
    %% 评估与监控流
    CLI --> SWE_BENCH
    CLI --> PERFORMANCE
    SWE_BENCH --> BATCH_PROCESSOR
    
    %% 流式响应流
    CORE --> STREAM_CALLBACK
    STREAM_CALLBACK --> MESSAGE_QUEUE
    MESSAGE_QUEUE --> TUI
    
    %% 工具结果反馈
    FILE_TOOLS --> OBSERVE
    SHELL_TOOLS --> OBSERVE
    SEARCH_TOOLS --> OBSERVE
    TODO_TOOLS --> OBSERVE
    WEB_TOOLS --> OBSERVE
    THINK_TOOLS --> OBSERVE
    EXTERNAL_TOOLS --> OBSERVE
    
    %% ================== 样式定义 ==================
    
    %% 用户层
    classDef userLayer fill:#e3f2fd,stroke:#1976d2,stroke-width:3px,color:#0d47a1
    
    %% 核心层
    classDef coreLayer fill:#fff8e1,stroke:#f57c00,stroke-width:4px,color:#e65100
    
    %% ReAct循环
    classDef reactLayer fill:#ffebee,stroke:#d32f2f,stroke-width:3px,color:#b71c1c
    
    %% LLM层
    classDef llmLayer fill:#f3e5f5,stroke:#7b1fa2,stroke-width:3px,color:#4a148c
    
    %% 工具层
    classDef toolLayer fill:#e8f5e8,stroke:#388e3c,stroke-width:2px,color:#1b5e20
    
    %% MCP层
    classDef mcpLayer fill:#fce4ec,stroke:#c2185b,stroke-width:2px,color:#880e4f
    
    %% 数据层
    classDef dataLayer fill:#e0f2f1,stroke:#00695c,stroke-width:2px,color:#004d40
    
    %% 评估层
    classDef evalLayer fill:#fff3e0,stroke:#ef6c00,stroke-width:2px,color:#bf360c
    
    %% 流式层
    classDef streamLayer fill:#ede7f6,stroke:#512da8,stroke-width:2px,color:#311b92
    
    %% ================== 样式应用 ==================
    
    class USER,CLI,TUI userLayer
    class AGENT,CORE,ENGINE coreLayer
    class THINK,ACT,OBSERVE reactLayer
    class LLM_FACTORY,BASIC_MODEL,REASON_MODEL,OPENROUTER llmLayer
    class TOOL_REGISTRY,BUILTIN_TOOLS,FILE_TOOLS,SHELL_TOOLS,SEARCH_TOOLS,TODO_TOOLS,WEB_TOOLS,THINK_TOOLS toolLayer
    class MCP_SYSTEM,MCP_CLIENT,MCP_TRANSPORT,STDIO_TRANSPORT,SSE_TRANSPORT,MCP_PROTOCOL,EXTERNAL_TOOLS mcpLayer
    class SESSION_MGR,SESSION_FILES,CONTEXT_MGR,MSG_PROCESSOR,COMPRESSOR,CONFIG_MGR,CONFIG_FILE,ENV_VARS,PROMPT_MGR,PROMPT_TEMPLATES dataLayer
    class SWE_BENCH,PERFORMANCE,BATCH_PROCESSOR evalLayer
    class STREAM_CALLBACK,MESSAGE_QUEUE streamLayer
```

## 🔄 核心数据流序列图

```mermaid
sequenceDiagram
    participant U as 👤 用户
    participant C as 🚀 CLI
    participant A as 🤖 ReactAgent  
    participant RC as ⚙️ ReactCore
    participant L as 🧠 LLM
    participant T as 🔧 Tools
    participant S as 💾 Session
    
    Note over U,S: ALEX 完整执行流程
    
    U->>C: 输入任务请求
    C->>A: 初始化Agent
    A->>S: 加载/创建会话
    S-->>A: 返回会话状态
    
    A->>RC: 启动ReAct循环
    
    loop ReAct 思考-行动-观察循环
        Note over RC,L: Think Phase - 分析阶段
        RC->>L: 发送任务上下文
        L-->>RC: 返回分析和计划
        
        Note over RC,T: Act Phase - 执行阶段  
        RC->>T: 调用相应工具
        T->>T: 执行具体操作
        T-->>RC: 返回执行结果
        
        Note over RC,RC: Observe Phase - 观察阶段
        RC->>RC: 分析结果，决定下一步
        
        opt 需要更多信息或操作
            RC->>L: 继续推理
        end
    end
    
    RC->>S: 保存会话状态
    RC-->>A: 返回最终结果
    A-->>C: 流式输出响应
    C-->>U: 显示结果
```

## 🧠 ReAct 认知架构详解

```mermaid
mindmap
  root((🤖 ALEX ReAct 架构))
    🤔 THINK 思考层
      问题分析
        自然语言理解
        任务分解
        上下文关联
      计划制定
        工具选择策略
        执行顺序规划  
        风险评估
      推理决策
        逻辑推理
        模式识别
        经验学习
    🎬 ACT 执行层
      工具调用
        内置工具(13个)
        MCP外部工具
        参数验证
      并发控制
        工具并行执行
        资源管理
        错误隔离
      安全机制
        权限控制
        沙箱执行
        风险阻断
    👀 OBSERVE 观察层
      结果分析
        执行状态检查
        输出内容解析
        错误识别
      反馈循环
        成功路径记录
        失败模式学习
        策略调整
      状态更新
        会话历史更新
        上下文压缩
        任务状态同步
```

## 📊 性能与可扩展性指标

| 维度 | 当前能力 | 扩展潜力 |
|------|----------|----------|
| **并发处理** | 单会话流式处理 | 多会话并行支持 |
| **工具生态** | 13个内置工具 + MCP | 无限扩展外部工具 |
| **模型支持** | OpenRouter多模型 | 任意API兼容模型 |
| **会话管理** | 本地文件持久化 | 分布式存储支持 |  
| **评估能力** | SWE-Bench 500实例 | 自定义评估数据集 |
| **部署方式** | 单机CLI工具 | 云原生微服务 |

---

## 🎯 架构优势总结

### 🏗️ **设计优势**
- **简洁性**: 遵循最小复杂度原则，避免过度工程化
- **可读性**: 清晰的接口定义，自文档化代码
- **可维护性**: 模块化设计，单一职责原则
- **可测试性**: 接口驱动，便于单元测试和集成测试

### ⚡ **性能优势**  
- **高效执行**: Go语言原生性能，低内存占用
- **流式处理**: 实时响应，提升用户体验
- **智能缓存**: LLM响应缓存，减少API调用成本
- **资源管控**: 上下文压缩，防止内存爆炸

### 🔗 **集成优势**
- **标准协议**: MCP协议，与主流工具生态兼容
- **多模型支持**: 灵活的LLM抽象，易于切换模型
- **配置层级**: 环境变量、配置文件、默认值三层配置
- **CLI友好**: 终端原生设计，开发者体验优秀

### 🚀 **扩展优势**
- **插件架构**: 工具系统易于扩展新功能
- **协议标准**: MCP协议支持第三方工具集成
- **评估框架**: SWE-Bench标准化评估，持续改进
- **云原生兼容**: 架构设计支持容器化和微服务化

---

这个架构分析展现了ALEX作为一个生产级AI编程助手的完整技术栈和设计哲学，体现了现代软件工程的最佳实践。