# Alex Cloud Agent - 云端部署指南

## 🚀 部署概述

本指南提供Alex Cloud Agent在 Kubernetes 集群上的部署参考。仓库当前仅随附 `k8s/deployment.yaml` 示例清单，需要通过 `kubectl`
手动应用；Istio、Argo CD 等扩展组件仍需读者按需自建。

## 📋 前置要求

### 基础设施要求
```yaml
infrastructure_requirements:
  kubernetes:
    version: ">=1.24"
    nodes: 
      min: 6
      recommended: 12
    node_specs:
      - type: "compute"
        instance: "c5.4xlarge"  
        count: 8
      - type: "storage"
        instance: "i3.2xlarge"
        count: 4
        
  networking:
    vpc_cidr: "10.0.0.0/16"
    subnets:
      - public: "10.0.1.0/24,10.0.2.0/24" 
      - private: "10.0.10.0/24,10.0.11.0/24"
    load_balancer: "Application Load Balancer"
    
  storage:
    persistent_storage: "100GB per node"
    backup_storage: "1TB"
    storage_class: "gp3"
```

### 软件依赖
```bash
# 必需工具
kubectl >= 1.24
helm >= 3.8
istioctl >= 1.16
argocd >= 2.5

# 可选工具  
k9s      # Kubernetes CLI管理
stern    # 多Pod日志查看
kubectx  # 快速切换context
```

## 🏗️ 部署架构

### 1. 命名空间规划
```yaml
# 创建命名空间
apiVersion: v1
kind: Namespace
metadata:
  name: alex-system
  labels:
    name: alex-system
    istio-injection: enabled
---
apiVersion: v1
kind: Namespace
metadata:
  name: alex-execution  
  labels:
    name: alex-execution
    istio-injection: enabled
---
apiVersion: v1
kind: Namespace
metadata:
  name: alex-storage
  labels:
    name: alex-storage
    istio-injection: disabled
---
apiVersion: v1
kind: Namespace
metadata:
  name: alex-monitoring
  labels:
    name: alex-monitoring
    istio-injection: disabled
```

### 2. Istio服务网格安装
```bash
#!/bin/bash
# install-istio.sh

# 1. 下载Istio
curl -L https://istio.io/downloadIstio | sh -
export PATH=$PWD/istio-1.16.0/bin:$PATH

# 2. 安装Istio控制平面
istioctl install --set values.defaultRevision=default -y

# 3. 启用自动注入
kubectl label namespace alex-system istio-injection=enabled
kubectl label namespace alex-execution istio-injection=enabled

# 4. 验证安装
istioctl verify-install
```

### 3. 基础存储配置
```yaml
# Redis集群配置
apiVersion: redis.redis.opstreelabs.in/v1beta1
kind: RedisCluster
metadata:
  name: alex-redis-cluster
  namespace: alex-storage
spec:
  clusterSize: 6
  redisExporter:
    enabled: true
    image: quay.io/opstree/redis-exporter:1.44.0
  storage:
    volumeClaimTemplate:
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 50Gi
        storageClassName: fast-ssd
  resources:
    requests:
      memory: 2Gi
      cpu: 1000m
    limits:
      memory: 4Gi
      cpu: 2000m

---
# PostgreSQL集群配置  
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: alex-postgres
  namespace: alex-storage
spec:
  instances: 3
  postgresql:
    parameters:
      max_connections: "200"
      shared_buffers: "256MB"
      effective_cache_size: "1GB"
  bootstrap:
    initdb:
      database: alexdb
      owner: alex
  storage:
    size: 100Gi
    storageClass: fast-ssd
  monitoring:
    enabled: true
```

## 🔧 服务部署流程

### 第一步：创建配置和密钥
```bash
#!/bin/bash
# setup-configs.sh

# 1. 创建配置映射
kubectl create configmap alex-config \
  --from-file=config.yaml \
  --namespace alex-system

# 2. 创建密钥
kubectl create secret generic alex-secrets \
  --from-literal=llm-api-key="${LLM_API_KEY}" \
  --from-literal=tavily-api-key="${TAVILY_API_KEY}" \
  --from-literal=AUTH_JWT_SECRET="${AUTH_JWT_SECRET}" \
  --from-literal=AUTH_DATABASE_URL="${AUTH_DATABASE_URL}" \
  --namespace alex-system

# 3. 创建TLS证书
kubectl create secret tls alex-tls-cert \
  --cert=alex-cloud.crt \
  --key=alex-cloud.key \
  --namespace istio-system
```

> 🔐 **生产登录提示**：`alex-server` 在检测到 `AUTH_JWT_SECRET` 与 `AUTH_DATABASE_URL` 后会自动启用 Postgres 仓储和 JWT 鉴权中间件。部署前请确保：
>
> 1. 在生产数据库中创建认证库并运行 `psql "$AUTH_DATABASE_URL" -f migrations/auth/001_init.sql`，初始化 `auth_users`、`auth_user_identities`、`auth_sessions`、`auth_states` 等表。
> 2. `AUTH_DATABASE_URL` 指向可从集群访问的地址（推荐通过 Kubernetes Service 或私网负载均衡暴露）。
> 3. 生成高熵的 `AUTH_JWT_SECRET`，并将 `AUTH_REDIRECT_BASE_URL` 设置为实际域名，保证前端回调正确。
> 4. 为数据库与 Secret 配置备份和轮换策略，刷新令牌及 OAuth state 才能跨 Pod 持久化。

### 第二步：部署核心服务
```yaml
# alex-agent-core-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: alex-agent-core
  namespace: alex-system
spec:
  replicas: 3
  selector:
    matchLabels:
      app: alex-agent-core
  template:
    metadata:
      labels:
        app: alex-agent-core
        version: v1
    spec:
      serviceAccountName: alex-agent-core
      containers:
      - name: agent-core
        image: alex/agent-core:v1.0
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: grpc
        env:
        - name: REDIS_URL
          value: "redis://alex-redis-cluster:6379"
        - name: POSTGRES_URL
          valueFrom:
            secretKeyRef:
              name: alex-postgres-app
              key: uri
        - name: LLM_API_KEY
          valueFrom:
            secretKeyRef:
              name: alex-secrets
              key: llm-api-key
        resources:
          requests:
            memory: 512Mi
            cpu: 500m
          limits:
            memory: 1Gi
            cpu: 1000m
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5

---
apiVersion: v1
kind: Service
metadata:
  name: alex-agent-core
  namespace: alex-system
spec:
  selector:
    app: alex-agent-core
  ports:
  - name: http
    port: 8080
    targetPort: 8080
  - name: grpc
    port: 9090
    targetPort: 9090
```

### 第三步：配置Istio网关和路由
```yaml
# alex-gateway.yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: alex-gateway
  namespace: alex-system
spec:
  selector:
    istio: ingressgateway
  servers:
  - port:
      number: 443
      name: https
      protocol: HTTPS
    tls:
      mode: SIMPLE
      credentialName: alex-tls-cert
    hosts:
    - alex-api.example.com

---
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: alex-routes
  namespace: alex-system
spec:
  hosts:
  - alex-api.example.com
  gateways:
  - alex-gateway
  http:
  - match:
    - uri:
        prefix: /api/v1/agent
    route:
    - destination:
        host: alex-agent-core
        port:
          number: 8080
    timeout: 30s
  - match:
    - uri:
        prefix: /api/v1/terminal
    route:
    - destination:
        host: alex-execution-engine
        port:
          number: 9090
    timeout: 300s
```

## 📊 监控和日志配置

### Prometheus监控
```yaml
# prometheus-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: alex-monitoring
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
      evaluation_interval: 15s
    
    rule_files:
    - "/etc/prometheus/rules/*.yml"
    
    scrape_configs:
    - job_name: 'alex-agent-core'
      kubernetes_sd_configs:
      - role: pod
        namespaces:
          names: ['alex-system']
      relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: alex-agent-core
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: pod
      - source_labels: [__meta_kubernetes_namespace]
        target_label: namespace
    
    - job_name: 'redis-cluster'
      static_configs:
      - targets: ['alex-redis-cluster:9121']
    
    - job_name: 'postgresql'
      static_configs:
      - targets: ['alex-postgres:9187']
```

### Grafana仪表板
> 提示：ALEX 不再提供预置的 Prometheus/Grafana 资产，请复用你现有的监控基础设施，并参考
> `docs/operations/monitoring_and_metrics.md` 中的指标说明编写自己的配置文件。

```bash
#!/bin/bash
# setup-monitoring.sh

# 1. 安装 Prometheus Operator（使用你的自定义 values 文件）
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace alex-monitoring \
  --create-namespace \
  --values /path/to/your-prometheus-values.yaml

# 2. 导入你的 Grafana 仪表板
kubectl apply -f /path/to/your-grafana-dashboards.yaml

# 3. 配置自定义告警规则
kubectl apply -f /path/to/your-alert-rules.yaml
```

## ✅ 部署验证

### 1. 健康检查脚本
```bash
#!/bin/bash
# health-check.sh

echo "=== Alex Cloud Agent 部署验证 ==="

# 检查命名空间
echo "检查命名空间..."
kubectl get namespaces | grep alex-

# 检查Pod状态
echo "检查Pod状态..."
kubectl get pods -n alex-system
kubectl get pods -n alex-execution  
kubectl get pods -n alex-storage

# 检查服务状态
echo "检查服务状态..."
kubectl get services -n alex-system

# 检查Istio配置
echo "检查Istio配置..."
istioctl proxy-status

# 检查外部连接
echo "检查外部API连接..."
curl -k https://alex-api.example.com/health

# 检查数据库连接
echo "检查数据库连接..."
kubectl exec -n alex-storage alex-postgres-1 -- psql -U alex -d alexdb -c "SELECT 1"

# 检查Redis连接
echo "检查Redis连接..."
kubectl exec -n alex-storage alex-redis-cluster-leader-0 -- redis-cli ping

echo "=== 验证完成 ==="
```

### 2. 功能测试
```bash
#!/bin/bash
# functional-test.sh

# API功能测试
echo "测试Agent API..."
curl -X POST https://alex-api.example.com/api/v1/agent/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -d '{
    "message": "Hello Alex",
    "session_id": "test-session"
  }'

# WebSocket终端测试
echo "测试Terminal WebSocket..."
wscat -c wss://alex-api.example.com/ws/v1/terminal \
  -H "Authorization: Bearer $AUTH_TOKEN"

# 会话管理测试
echo "测试Session管理..."
curl https://alex-api.example.com/api/v1/sessions \
  -H "Authorization: Bearer $AUTH_TOKEN"
```

## 🔄 滚动更新策略

### 蓝绿部署
```yaml
# rollout-strategy.yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: alex-agent-core
  namespace: alex-system
spec:
  replicas: 6
  strategy:
    blueGreen:
      activeService: alex-agent-core-active
      previewService: alex-agent-core-preview
      autoPromotionEnabled: false
      scaleDownDelaySeconds: 30
      prePromotionAnalysis:
        templates:
        - templateName: success-rate
        args:
        - name: service-name
          value: alex-agent-core-preview
  selector:
    matchLabels:
      app: alex-agent-core
  template:
    metadata:
      labels:
        app: alex-agent-core
    spec:
      containers:
      - name: agent-core
        image: alex/agent-core:v1.1  # 新版本
```

### 回滚程序
```bash
#!/bin/bash
# rollback.sh

# 获取当前rollout状态
kubectl argo rollouts get rollout alex-agent-core -n alex-system

# 执行回滚
kubectl argo rollouts undo alex-agent-core -n alex-system

# 验证回滚状态
kubectl argo rollouts status alex-agent-core -n alex-system
```

## 🛠️ 故障排查

### 常见问题诊断
```bash
#!/bin/bash
# troubleshoot.sh

echo "=== Alex Cloud Agent 故障排查 ==="

# 1. 检查Pod状态和事件
kubectl describe pods -n alex-system -l app=alex-agent-core

# 2. 查看Pod日志
kubectl logs -n alex-system -l app=alex-agent-core --tail=100

# 3. 检查网络连接
kubectl exec -n alex-system deployment/alex-agent-core -- netstat -tlnp

# 4. 检查存储状态
kubectl get pvc -n alex-storage

# 5. 检查Istio配置
istioctl analyze -n alex-system

# 6. 检查资源使用
kubectl top pods -n alex-system
kubectl top nodes
```

### 性能分析
```bash
#!/bin/bash
# performance-analysis.sh

# CPU和内存使用分析
kubectl top pods -n alex-system --sort-by=cpu
kubectl top pods -n alex-system --sort-by=memory

# 网络延迟测试
kubectl exec -n alex-system deployment/alex-agent-core -- \
  wget -qO- https://alex-api.example.com/health

# 数据库性能检查
kubectl exec -n alex-storage alex-postgres-1 -- \
  psql -U alex -d alexdb -c "SELECT * FROM pg_stat_activity;"
```

## 📋 部署检查清单

### 部署前检查
- [ ] Kubernetes集群版本 >= 1.24
- [ ] 节点资源充足 (CPU: 16核+, 内存: 32GB+)
- [ ] 存储类已配置 (fast-ssd)
- [ ] 网络策略已规划
- [ ] SSL证书已准备
- [ ] 必要的secrets已创建

### 部署后验证
- [ ] 所有Pod状态为Running
- [ ] 服务健康检查通过
- [ ] Istio网关配置正确
- [ ] 外部API可访问
- [ ] WebSocket连接正常
- [ ] 数据库连接正常
- [ ] 监控指标正常
- [ ] 日志输出正常

### 生产就绪检查
- [ ] 负载测试通过
- [ ] 故障转移测试通过  
- [ ] 备份恢复测试通过
- [ ] 安全扫描通过
- [ ] 性能基准达标
- [ ] 监控告警配置完成
- [ ] 运维手册完善

---

**部署指南版本**: v1.0  
**适用版本**: Alex Cloud Agent v1.0+  
**更新时间**: 2025-01-27