# 使用国内镜像加速沙箱构建
> Last updated: 2025-11-18


本文档介绍如何在中国大陆环境下使用镜像源加速 ALEX 沙箱容器的构建。

## 快速开始

### 方法 0：使用预构建的国内镜像（🚀 最快，强烈推荐）

使用火山引擎（Volcengine）托管的预构建沙箱镜像，**无需构建，秒级启动**：

```bash
# 一键配置
./scripts/setup-china-mirrors-all.sh

# 手动配置：编辑 .env 文件，添加
SANDBOX_IMAGE=enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest
SANDBOX_SECURITY_OPT=seccomp=unconfined

# 启动服务
./deploy.sh start
```

**优势**：
- ✅ 无需构建，直接拉取预构建镜像
- ✅ 启动时间从 15-25 分钟缩短到 **30 秒**
- ✅ 使用火山引擎国内 CDN，下载速度极快
- ✅ 无需配置 npm/pip 镜像

**镜像信息**：
- 镜像地址：`enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest`
- 提供方：火山引擎（字节跳动）
- 更新频率：与上游保持同步

### 方法 1：一键配置所有镜像

运行自动化脚本，一次性配置所有镜像源：

```bash
./scripts/setup-china-mirrors-all.sh
```

该脚本会自动：
- **优先配置 `USE_CHINA_SANDBOX=true`（使用预构建镜像）**
- **将 `AUTH_DB_IMAGE` 指向国内 Postgres 镜像（`docker.m.daocloud.io/library/postgres:15`）**
- 在 Linux 上配置 Docker 镜像加速器（需要 sudo）
- 在 macOS/Windows 上提供 Docker Desktop 配置指引
- 备选：配置 NPM 和 PyPI 镜像（用于自行构建）

### 方法 2：手动构建（仅在无法使用预构建镜像时）

如果因为某些原因无法使用预构建镜像，可以配置 npm/pip 镜像自行构建：

编辑项目根目录的 `.env` 文件：

```bash
# 不使用预构建镜像
# USE_CHINA_SANDBOX=false

# 配置构建时的镜像源
NPM_REGISTRY=https://registry.npmmirror.com/
PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple
```

然后正常启动服务：

```bash
./deploy.sh start
```

### 方法 4：直接使用 docker-compose

```bash
# 使用预构建镜像（推荐）
SANDBOX_IMAGE=enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest \
SANDBOX_SECURITY_OPT=seccomp=unconfined \
AUTH_DB_IMAGE=docker.m.daocloud.io/library/postgres:15 \
docker-compose up -d sandbox

# 或使用 npm/pip 镜像构建
NPM_REGISTRY=https://registry.npmmirror.com/ \
PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple \
docker-compose build sandbox
```

## 可用的国内镜像源

### 预构建沙箱镜像

| 提供方 | 镜像地址 | 说明 |
|--------|----------|------|
| 火山引擎 | `enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest` | **强烈推荐**，秒级启动 |

**使用方式**：在 `.env` 中设置 `SANDBOX_IMAGE` 和 `SANDBOX_SECURITY_OPT`

### 认证数据库镜像

| 提供方 | 镜像地址 | 说明 |
|--------|----------|------|
| DaoCloud 镜像站 | `docker.m.daocloud.io/library/postgres:15` | 官方 `postgres:15` 的国内镜像 |

**使用方式**：在 `.env` 中设置 `AUTH_DB_IMAGE=docker.m.daocloud.io/library/postgres:15`

### Docker Hub 镜像加速

| 提供方 | 镜像地址 | 说明 |
|--------|----------|------|
| 中国科技大学 | `https://docker.mirrors.ustc.edu.cn` | 推荐，稳定可靠 |
| 网易 | `https://hub-mirror.c.163.com` | 老牌镜像 |
| 阿里云 | `https://<your-id>.mirror.aliyuncs.com` | 需注册获取专属地址 |
| 腾讯云 | `https://mirror.ccs.tencentyun.com` | 企业级 |

**注意**：由于 Docker Hub 镜像政策变化，部分镜像站可能不可用。建议使用中科大或网易镜像。

### NPM 镜像

| 提供方 | 镜像地址 | 说明 |
|--------|----------|------|
| 淘宝 NPM | `https://registry.npmmirror.com/` | 推荐，更新及时 |
| 腾讯云 | `https://mirrors.cloud.tencent.com/npm/` | 稳定可靠 |
| 华为云 | `https://mirrors.huaweicloud.com/repository/npm/` | 企业级 |

### Python PyPI 镜像

| 提供方 | 镜像地址 | 说明 |
|--------|----------|------|
| 清华大学 | `https://pypi.tuna.tsinghua.edu.cn/simple` | 推荐，更新快 |
| 阿里云 | `https://mirrors.aliyun.com/pypi/simple/` | 稳定可靠 |
| 中国科技大学 | `https://pypi.mirrors.ustc.edu.cn/simple/` | 老牌镜像 |
| 华为云 | `https://mirrors.huaweicloud.com/repository/pypi/simple/` | 企业级 |

## 技术实现

### Dockerfile 构建参数

`Dockerfile.sandbox` 支持以下构建参数：

```dockerfile
ARG NPM_REGISTRY=https://registry.npmjs.org/
ARG PIP_INDEX_URL=https://pypi.org/simple
```

### docker-compose 配置

`docker-compose.yml` 将环境变量传递给构建参数：

```yaml
sandbox:
  build:
    context: .
    dockerfile: Dockerfile.sandbox
    args:
      NPM_REGISTRY: ${NPM_REGISTRY:-https://registry.npmjs.org/}
      PIP_INDEX_URL: ${PIP_INDEX_URL:-https://pypi.org/simple}
```

## 验证镜像配置

启动服务后，查看日志确认镜像配置已生效：

```bash
./deploy.sh logs sandbox
```

在构建过程中会显示使用的镜像源：

```
▸ Using custom mirrors for faster builds:
  NPM: https://registry.npmmirror.com/
  PIP: https://pypi.tuna.tsinghua.edu.cn/simple
```

## Docker 镜像加速配置

### 为什么需要 Docker 镜像加速

构建沙箱容器时，需要从 Docker Hub 拉取基础镜像（如 `ghcr.io/agent-infra/sandbox:latest`）。在中国大陆，直接访问 Docker Hub 速度很慢或无法访问，配置镜像加速器可以显著提升构建速度。

### 配置方法

#### Linux 系统

1. 创建或编辑 `/etc/docker/daemon.json`：

```bash
sudo mkdir -p /etc/docker
sudo tee /etc/docker/daemon.json <<-'EOF'
{
  "registry-mirrors": [
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com"
  ]
}
EOF
```

2. 重启 Docker 服务：

```bash
sudo systemctl daemon-reload
sudo systemctl restart docker
```

3. 验证配置：

```bash
docker info | grep -A 5 "Registry Mirrors"
```

#### macOS 系统

1. 打开 Docker Desktop
2. 点击 **Settings (Preferences)** → **Docker Engine**
3. 在 JSON 配置中添加：

```json
{
  "registry-mirrors": [
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com"
  ]
}
```

4. 点击 **Apply & Restart**

#### Windows 系统

1. 打开 Docker Desktop
2. 点击 **Settings** → **Docker Engine**
3. 在 JSON 配置中添加：

```json
{
  "registry-mirrors": [
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com"
  ]
}
```

4. 点击 **Apply & Restart**

### 使用自动化脚本（推荐）

我们提供了自动化配置脚本：

```bash
# 配置 Docker 镜像加速（仅限 Linux）
./scripts/setup-docker-mirrors.sh

# 测试所有镜像配置
./scripts/test-china-mirrors.sh
```

## 常见问题

### 1. 构建时仍然很慢

**可能原因**：
- Docker 镜像加速器未配置或配置无效
- npm/pip 镜像未配置
- 网络质量问题

**解决方案**：

1. 确认 Docker 镜像加速器已生效：
   ```bash
   docker info | grep -A 5 "Registry Mirrors"
   ```

2. 确认 npm/pip 镜像已配置：
   ```bash
   ./scripts/test-china-mirrors.sh
   ```

3. 尝试使用不同的镜像源

### 2. pip 安装报 SSL 证书错误

**临时解决方案**（不推荐生产环境）：

```bash
PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple \
PIP_TRUSTED_HOST=pypi.tuna.tsinghua.edu.cn \
./deploy.sh start
```

**推荐方案**：使用 HTTPS 且证书有效的镜像源

### 3. 已有镜像如何重新构建

清除旧镜像并重新构建：

```bash
# 停止并删除容器
./deploy.sh down

# 删除镜像
docker rmi alex-sandbox

# 重新构建并启动
NPM_REGISTRY=https://registry.npmmirror.com/ \
PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple \
./deploy.sh start
```

或使用 docker-compose 强制重建：

```bash
NPM_REGISTRY=https://registry.npmmirror.com/ \
PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple \
docker-compose build --no-cache sandbox
```

## 性能对比

在中国大陆环境测试（仅供参考）：

| 配置 | 首次启动时间 | 说明 |
|------|-------------|------|
| 默认源（构建） | ~15-25 分钟 | 依赖网络质量 |
| npm/pip 镜像（构建） | ~5-8 分钟 | 需要配置镜像源 |
| npm/pip + Docker 镜像（构建） | ~3-5 分钟 | 全面优化构建 |
| **预构建镜像（推荐）** | **~30 秒** | **🚀 最快，无需构建** |

**推荐配置**：`USE_CHINA_SANDBOX=true`，启动速度提升 **30-50 倍**！

## 参考资源

- [火山引擎容器镜像服务](https://www.volcengine.com/products/cr) - 预构建沙箱镜像托管
- [淘宝 NPM 镜像](https://npmmirror.com/)
- [清华大学开源软件镜像站](https://mirrors.tuna.tsinghua.edu.cn/)
- [阿里云开源镜像站](https://developer.aliyun.com/mirror/)
- [Docker 镜像加速器配置](https://yeasy.gitbook.io/docker_practice/install/mirror)
