# 文字转语音网关技术方案
> Last updated: 2025-11-18


## 1. 模块职责
- 将多家 TTS API 统一封装，提供可缓存、可降级的语音合成功能。  
- 支持文本、SSML 输入，返回音频文件和元数据（时长、音色、供应商信息）。  
- 与任务编排服务协同，将 TTS 结果映射到音频轨道别名。

## 2. 架构设计
| 组件 | 职责 | 接口 |
| --- | --- | --- |
| `internal/tts` | 客户端接口、缓存、失败重试 | `Client.Synthesize`, `Provider` |
| `configs/tts/providers.yaml` | 供应商配置 | API Key、语音 ID、速率限制 |
| `internal/tts/providers/http` | 通用 HTTP provider | `Call(ctx, Request)` |

## 3. 调用流程
1. 任务编排传入 `TTSRequest`（文本、voice、format、别名）。  
2. 客户端计算缓存键（文本 hash + voice + format）。  
3. 若命中缓存 → 返回本地文件路径。  
4. 未命中 → 选择 provider（按权重/地区），构造请求：
   - 组装 SSML（语速、停顿、情感）。
   - 添加鉴权 header、trace-id。  
5. 解析响应并写入本地缓存目录。  
6. 生成 `Result`：包含路径、供应商、缓存命中情况、预估时长。

## 4. 缓存策略
- 采用本地文件缓存（`storage.Manager`），目录结构：`tts/{voice}/{hash}.mp3`。  
- 同时写入 `tts/index.json`，记录文本摘要、生成时间、供应商。  
- 提供清理工具：按时间/缓存大小淘汰。

## 5. 降级策略
- 首选主供应商，失败后根据 `fallback_chain` 依次尝试。  
- 失败后返回结构化错误，包含 `provider`, `status_code`, `request_id`。  
- 可配置开关：允许在降级失败时输出提示音（告警）。

## 6. 安全与合规
- 所有 API Key 由环境变量或本地密钥管理器提供。  
- 文本内容先经过敏感词扫描（同步/异步）。  
- 记录语音授权范围：商业/内部/试用。  
- 支持 GDPR/数据脱敏：可选不缓存原文，仅保留 hash。

## 7. 指标
- `tts_requests_total{provider}`。  
- `tts_cache_hit_ratio`。  
- `tts_request_duration_seconds`.  
- 错误代码分布，供限流与告警。

## 8. 迭代路线
1. MVP：单一 provider + 文件缓存。
2. v1：多提供商、失败重试、敏感词检查。
3. v2：本地推理（VITS/Coqui）、GPU 调度。
4. v3：情感标签自动识别、字幕对齐工具。

## 9. 任务进度
- [x] 文件缓存客户端（`FileCacheClient`）与 Mock Provider。
- [ ] 多供应商策略与敏感词审核。
- [ ] 本地推理入口与 GPU 调度。
