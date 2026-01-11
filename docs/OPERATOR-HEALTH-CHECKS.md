# Kubernetes Operator Health Check Best Practices

本文档说明KubeRDE Operator的健康检查实现及Kubernetes Operator健康检查的最佳实践。

## Operator健康检查方案对比

### 1. HTTP Health Server（KubeRDE采用）

**优点**：
- 标准化的HTTP接口
- 易于测试和调试
- 支持复杂的健康检查逻辑
- 与其他服务健康检查方式一致

**实现**：
```go
// 启动HTTP健康检查服务器
func startHealthCheckServer(controller *Controller) {
    http.HandleFunc("/healthz", handleHealthz)
    http.HandleFunc("/livez", handleHealthz)
    http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
        handleReadyz(w, r, controller)
    })

    port := os.Getenv("HEALTH_CHECK_PORT")
    if port == "" {
        port = "8080"
    }

    if err := http.ListenAndServe(":"+port, nil); err != nil {
        log.Printf("Health check server error: %v", err)
    }
}
```

**部署配置**：
```yaml
containers:
- name: operator
  image: kuberde-operator:latest
  # 声明端口（用于健康检查，不需要Service）
  ports:
  - containerPort: 8080
    name: health
    protocol: TCP

  livenessProbe:
    httpGet:
      path: /healthz
      port: 8080  # 直接访问pod内部端口
    initialDelaySeconds: 30
    periodSeconds: 10

  readinessProbe:
    httpGet:
      path: /readyz
      port: 8080
    initialDelaySeconds: 10
    periodSeconds: 5
```

**关键点**：
- ✅ 声明`containerPort`（即使不创建Service）
- ✅ **不创建Service**（operator不需要对外暴露）
- ✅ Probe直接访问pod内部端口
- ✅ 健康检查逻辑可以访问controller状态

### 2. Exec Probe（更简单）

**优点**：
- 不需要HTTP服务器
- 实现简单
- 资源占用更少

**缺点**：
- 难以实现复杂的健康检查
- 调试不便
- 需要额外的命令行工具

**实现**：
```yaml
livenessProbe:
  exec:
    command:
    - /bin/sh
    - -c
    - "pgrep operator || exit 1"
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  exec:
    command:
    - /bin/sh
    - -c
    - "test -f /tmp/operator-ready"
  initialDelaySeconds: 10
  periodSeconds: 5
```

### 3. Leader Election + Health Check

**适用场景**：
- 多副本operator（高可用）
- 使用leader election的operator

**实现**：
```go
// 只有leader执行reconcile
if !elector.IsLeader() {
    // 非leader也返回健康状态（但不执行reconcile）
    return healthz.OK
}
// Leader执行健康检查
return controller.HealthCheck()
```

## KubeRDE Operator健康检查详解

### Liveness Probe (`/healthz`)

**目的**：确认operator进程存活

**检查内容**：
- HTTP服务器响应
- 基本的进程健康

**失败处理**：
- Kubernetes会重启容器
- 用于检测死锁、崩溃等严重问题

**代码**：
```go
func handleHealthz(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status":  "ok",
        "service": "kuberde-operator",
    })
}
```

### Readiness Probe (`/readyz`)

**目的**：确认operator准备好处理请求

**检查内容**：
1. **Informer已同步**：确保operator已从API server同步了所有资源
2. **Kubernetes客户端可用**：能够访问K8s API

**失败处理**：
- Pod从Service endpoints移除（如果有Service）
- 不会重启容器
- 用于优雅处理初始化和临时故障

**代码**：
```go
func handleReadyz(w http.ResponseWriter, r *http.Request, controller *Controller) {
    // 检查informer是否同步
    if controller.informer != nil && !controller.informer.HasSynced() {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{
            "status": "not ready",
            "reason": "informer not synced",
        })
        return
    }

    // 检查Kubernetes客户端（使用operator有权限的资源）
    namespace := os.Getenv("OPERATOR_NAMESPACE")
    if namespace == "" {
        namespace = "kuberde"
    }

    // 使用Pods而不是Namespaces，因为operator的RBAC只有pods权限
    _, err := controller.k8sClient.CoreV1().Pods(namespace).List(
        context.Background(), metav1.ListOptions{Limit: 1})
    if err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{
            "status": "not ready",
            "reason": "kubernetes client not accessible",
        })
        return
    }

    // 所有检查通过
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status":  "ready",
        "service": "kuberde-operator",
    })
}
```

**重要提示**：健康检查使用的K8s API调用必须是operator RBAC权限范围内的资源。不要使用`List Namespaces`这种需要cluster-admin权限的操作。

## 为什么不暴露Operator的Service？

### 安全考虑
- Operator不需要接收外部请求
- 只reconcile Kubernetes资源
- 减少攻击面

### 架构清晰
- Operator是控制平面组件
- 不是数据平面服务
- 只与Kubernetes API交互

### 健康检查仍然有效
- Kubernetes probe通过**pod内部网络**访问
- 不需要Service或Ingress
- `containerPort`声明是为了清晰性和文档化

## 测试Operator健康检查

### 方法1：通过Pod直接访问

```bash
# 获取operator pod名称
OPERATOR_POD=$(kubectl get pods -n kuberde -l app=kuberde-operator -o jsonpath='{.items[0].metadata.name}')

# 测试健康检查
kubectl exec -n kuberde $OPERATOR_POD -- curl -s http://localhost:8080/healthz
kubectl exec -n kuberde $OPERATOR_POD -- curl -s http://localhost:8080/readyz
```

### 方法2：检查Pod状态

```bash
# 检查pod是否Ready（readiness probe通过）
kubectl get pod -n kuberde -l app=kuberde-operator

# 检查详细条件
kubectl get pod -n kuberde -l app=kuberde-operator -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")]}'

# 检查重启次数（liveness probe失败会导致重启）
kubectl get pod -n kuberde -l app=kuberde-operator -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}'
```

### 方法3：查看Events

```bash
# 查看probe失败事件
kubectl get events -n kuberde --field-selector involvedObject.kind=Pod | grep -i probe

# 查看特定pod的events
kubectl describe pod -n kuberde $OPERATOR_POD | grep -A 10 "Events:"
```

### 方法4：使用自动化测试脚本

```bash
# 运行健康检查测试脚本
./scripts/test-health-checks.sh
```

## 常见问题排查

### Probe失败但Operator日志正常

**可能原因**：
- HTTP服务器未启动
- 端口配置错误
- 防火墙规则

**排查**：
```bash
# 检查端口是否监听
kubectl exec -n kuberde $OPERATOR_POD -- netstat -tlnp | grep 8080

# 检查HTTP服务器日志
kubectl logs -n kuberde $OPERATOR_POD | grep "health check server"
```

### Readiness Probe持续失败

**可能原因**：
- Informer未同步（初始化时间长）
- Kubernetes API连接问题
- RBAC权限不足

**排查**：
```bash
# 检查operator日志
kubectl logs -n kuberde $OPERATOR_POD

# 检查健康检查endpoint返回
kubectl exec -n kuberde $OPERATOR_POD -- curl -s http://localhost:8080/readyz

# 检查RBAC权限（确保有pods list权限）
kubectl auth can-i list pods --as=system:serviceaccount:kuberde:kuberde-operator -n kuberde

# 增加initialDelaySeconds给informer更多时间
```

**常见错误**：
1. **"kubernetes client not accessible"** - 检查健康检查使用的资源是否在RBAC权限内
   - ❌ 错误：`List Namespaces` - 需要cluster-admin权限
   - ✅ 正确：`List Pods` - operator有此权限

2. **"informer not synced"** - 增加`initialDelaySeconds`或检查CRD是否正确安装

### Liveness Probe导致频繁重启

**可能原因**：
- initialDelaySeconds太短
- timeoutSeconds太短
- operator初始化时间长

**解决**：
```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 60  # 增加初始延迟
  periodSeconds: 15        # 降低检查频率
  timeoutSeconds: 10       # 增加超时时间
  failureThreshold: 5      # 增加失败阈值
```

## 参考资料

- [Kubernetes Liveness and Readiness Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- [Operator SDK Health Checks](https://sdk.operatorframework.io/docs/building-operators/golang/advanced-topics/#health-checks)
- [Controller Runtime Health Checks](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/healthz)
