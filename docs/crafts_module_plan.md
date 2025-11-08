# 用户体系与 Crafts 模块落地方案

本文档梳理了在 ALEX 项目中引入用户体系、云端产物存储以及 Crafts 管理模块的整体实施路径。方案基于现有后端 Go 服务与 Next.js 前端，兼顾本地开发与云端部署的可操作性。

## 1. 用户认证与多租户隔离 ✅

### 目标
- 所有会话、任务、产物均绑定具体用户 ID。
- API 层拦截未认证请求，防止跨用户访问。
- 前端具备最小可用的登录体验，并在请求中透传凭证。

### 实施要点
1. **认证中间件**：新增 `internal/server/http/auth` 包，实现基于 Bearer Token 的校验逻辑。中间件在通过后把 `user_id` 注入 `context`，供后续链路读取。
2. **上下文传播**：在 `internal/utils/id` 中扩展 `WithUserID`、`UserIDFromContext` 等辅助方法，确保 `ServerCoordinator`、`AgentCoordinator`、`SessionStore` 均能读取当前用户。
3. **数据模型扩展**：`ports.Session` 增加 `UserID` 字段；`filestore` 层在创建/读取/保存时持久化该字段并校验用户一致性。
4. **API 调整**：所有 Session、Task、Crafts 相关 Handler 在执行业务前校验 `user_id` 是否存在；任务执行、会话读取等均基于用户上下文。
5. **前端登录**：在 `web/app/(auth)/login` 下提供简易登录页，将用户输入的 token 存入 `localStorage`，并由 `apiClient` 在请求头中自动追加 `Authorization`。

## 2. 云端对象存储抽象 ✅

### 目标
- 将用户上传或 Agent 生成的二进制产物写入统一的对象存储。
- 返回可下载的签名地址，避免直接暴露存储凭证。

### 实施要点
1. **接口定义**：新增 `internal/storage/blobstore` 包，定义 `BlobStore` 接口，提供 `PutObject`、`GetSignedURL`、`DeleteObject` 等方法。
2. **本地默认实现**：提供 `filesystem` 版本，使用工作目录下的 `data/blobs` 作为存储，方便本地开发。结构允许日后扩展 S3/GCS 驱动。
3. **上传流程**：在处理用户附件及任务生成的 `Attachment` 时，如检测到 `Data` 字段则上传到 `BlobStore` 并返回对象键，同时清理原始内联数据。
4. **产物记录**：扩展 `ports.Artifact` 数据结构，记录 `StorageKey`、`MediaType`、`Size`、`Checksum` 等信息，并在 Session 中保存。
5. **签名链接**：后端新增 `/api/crafts/:id/download` 等接口，通过 `BlobStore.GetSignedURL` 生成一次性下载地址。

## 3. Crafts 模块 ✅

### 目标
- 在后端提供统一的产物查询、删除、下载 API。
- 在前端新增 `Crafts` 页面，支持列表展示、筛选与下载。

### 实施要点
1. **服务层**：实现 `internal/server/app/craft_service.go`，聚合用户在 Sessions 中的 Artifacts，并提供分页、过滤等能力。
2. **HTTP Handler**：在 `APIHandler` 中新增 `HandleListCrafts`、`HandleDeleteCraft`、`HandleDownloadCraft` 等方法，对应路由 `/api/crafts`、`DELETE /api/crafts/:id`、`GET /api/crafts/:id/download`。
3. **前端页面**：在 `web/app/crafts/page.tsx` 构建界面，调用 `apiClient.crafts.*` 系列函数展示卡片式产物列表、提供下载按钮。
4. **批量操作**：接口设计兼容批量删除/下载，前端先行实现单条操作并为后续扩展预留钩子。
5. **测试覆盖**：为关键组件与服务增加单元测试，确保用户隔离、签名链接与错误场景均得到验证。

---

以上方案已经在当前实现中逐项落地：已扩充上下文与数据结构，接入存储与 Craft 服务，并补齐前端登录与管理界面。

## 4. 实施检查清单

- [x] 后端路由启用认证中间件并对所有会话/任务接口进行用户校验。
- [x] 会话持久层新增 `user_id` 与 `artifacts` 字段，并验证跨用户访问会被拒绝。
- [x] 引入 BlobStore 抽象及文件系统实现，任务产物自动上传并清除内联 Base64。
- [x] CraftService 提供列表、删除、下载签名等 API。
- [x] 前端提供登录页与 crafts 页面，可获取、下载与删除产物。

## 5. Sandbox 工件同步 ✅

### 目标
- 将 Crafts 产物镜像为本地文件，挂载到 Sandbox 容器，确保 Agent 可以直接读取成果。
- 在镜像目录保留 metadata 文件，记录产物来源、存储键与文件名，方便二次处理。

### 实施要点
1. **镜像组件**：新增 `internal/storage/craftsync` 包，写入 `metadata.json` 与原始内容文件，支持自定义目录与权限。
2. **业务集成**：`ArtifactManager`、`WorkbenchService` 在保存/删除产物时调用镜像接口，失败只记录日志不阻塞主流程。
3. **容器挂载**：`docker-compose`、`docker-compose.dev` 为 `alex-server` 与 `sandbox` 服务增加 `alex-crafts` 卷映射，通过环境变量 `ALEX_CRAFT_MIRROR_DIR` 控制写入路径。
4. **回收机制**：`CraftService` 删除 Craft 时同步清理镜像目录，防止文件残留；单元测试覆盖保存、删除的镜像行为。

### 状态
- [x] 实现文件镜像逻辑并默认写入 `~/.alex-crafts`。
- [x] 保存/删除草稿时自动维护镜像文件与 metadata。
- [x] Docker Compose 镜像目录挂载到 sandbox，Agent 可通过 `/workspace/crafts` 直接访问。
