# 存储与元数据治理技术方案
> Last updated: 2025-11-18


## 1. 模块职责
- 统一管理素材、临时文件、成品输出及缓存。  
- 提供路径归一化、权限检查、版本记录与校验和。  
- 为各模块提供一致的文件操作接口（读、写、列举）。

## 2. 组件结构
| 组件 | 职责 | 核心接口 |
| --- | --- | --- |
| `internal/storage` | 路径校验、读写工具 | `Manager.Resolve`, `WriteFile`, `EnsureDir` |
| `configs/storage/policy.yaml` | 白名单根目录、配额、保留策略 | `roots`, `retention_days` |
| `scripts/storage_audit.sh` | 定期审计 | 统计容量、校验和 |

## 3. 目录结构
```
<root>
├── assets/          # 原始素材
├── working/         # 中间文件（可清理）
├── outputs/         # 成品
├── cache/tts/       # TTS 缓存
└── metadata/
    ├── jobs/<job_id>.json
    └── checksums/<file>.sha256
```

## 4. 路径安全
- 所有相对路径使用 `filepath.Clean` 并检测是否逃逸根目录。  
- 写入前调用 `EnsureDir` 创建父目录。  
- 提供 `ReadOnly` 模式：禁止写操作。  
- 配置 `allow_overwrite` 控制是否覆盖已有文件。

## 5. 元数据
- `jobs/*.json`：记录任务输入、输出、校验和、耗时、版本。  
- `checksums/*.sha256`：与文件同名，便于验收。  
- 可选 `sqlite`/`badger` 存储索引，加速查询。

## 6. 生命周期管理
- `working/`：作业结束即删除或 24 小时后自动清理。  
- `cache/tts/`: LRU + 最大容量限制。  
- `outputs/`: 按项目/日期归档，支持自动同步至远端（rsync/对象存储）。

## 7. 备份与恢复
- 每日增量备份 `outputs/` 与 `metadata/` 至 NAS/对象存储。  
- 提供恢复脚本：按任务号恢复素材与日志。  
- 关键参数：保留 30 天以上，恢复成功率 ≥ 99%。

## 8. 安全
- 权限分离：素材目录只读，工作目录读写。  
- 文件操作记录审计日志（时间、用户、动作）。  
- 可选开启文件级加密（基于 fscrypt 或 eCryptfs）。

## 9. 迭代路线
1. MVP：本地文件系统 + 校验和。
2. v1：配额管理、自动清理脚本。
3. v2：多站点同步、版本回溯。
4. v3：对象存储抽象、CDN 分发。

## 10. 任务进度
- [x] 本地根目录守护、覆写策略与读写封装。
- [ ] 校验和与元数据持久化。
- [ ] 配额/清理脚本与对象存储抽象。
