## 问题背景

最近在研究 Kubernetes Operator，希望通过自定义资源的方式解决业务定时扩缩容的问题。

在实际业务中，经常会遇到这种场景：每天固定时间段流量上涨，需要提前扩容；流量下降后，又需要自动缩容。如果完全依赖人工操作，容易出现扩容不及时、缩容遗漏、资源浪费等问题。

所以这里基于 Kubebuilder 实现了一个 `CronScale-Operator`，主要提供两个能力：

| 能力 | 说明 |
| --- | --- |
| 定时扩缩容 | 按照配置的 cron 时间，自动调整 Deployment 副本数 |
| 镜像预热 | 在扩容前提前把业务镜像拉取到目标节点，减少 Pod 启动耗时 |

项目通过 `CronScale` 自定义资源描述扩容时间、缩容时间、目标副本数和镜像预热时间。Operator 监听到资源后，会自动注册定时任务，到时间后执行对应操作。

## 环境介绍

| 组件名称 | 组件版本 | 组件作用 |
| --- | --- | --- |
| Kubernetes | v1.11.3+ | Operator 运行环境，负责管理 Deployment、Job、CRD 等资源 |
| Go | v1.21.0+ | 编译 Operator 和 cronscale-agent |
| Docker | v17.03+ | 构建和推送 Operator 镜像 |
| kubectl | v1.11.3+ | 安装 CRD、部署资源、查看运行状态 |
| Kubebuilder | controller-runtime v0.17.2 | Operator 开发框架 |
| containerd | 集群节点运行时 | 提供节点镜像管理能力 |
| MySQL | 自建实例 | 存储节点和镜像的对应关系 |
| cronscale-agent | 项目内组件 | 以 DaemonSet 方式采集每个节点已有镜像 |

## 核心架构

| 模块 | 路径 | 作用 |
| --- | --- | --- |
| CRD 定义 | `api/v1/cronscale_types.go` | 定义 `CronScale` 资源字段 |
| Webhook 校验 | `api/v1/cronscale_webhook.go` | 校验扩容副本数、缩容副本数、执行时间等参数 |
| Controller | `internal/controller/cronscale_controller.go` | 监听 `CronScale` 资源，注册定时扩缩容和镜像预热任务 |
| HPA 工具 | `pkg/hpa.go` | 当业务配置 HPA 时，同步调整 HPA 的副本上下限 |
| agent | `cronscale-agent/main.go` | 扫描节点本地镜像，并写入 MySQL |

整体流程：

```text
创建 CronScale 资源
        |
        v
Operator 监听资源变化
        |
        v
注册扩容、缩容、镜像预热 cron 任务
        |
        v
到达指定时间后执行任务
        |
        v
更新 Deployment / HPA 或创建镜像预热 Job
```

## 代码关键节点说明

| 关键节点 | 代码位置 | 说明 |
| --- | --- | --- |
| 初始化 Kubernetes Client | `pkg/client.go` | `kubeconfig` 默认设置为空，public 仓库不提交真实集群凭据；本地调试时再按环境填入 |
| 初始化 MySQL Client | `pkg/client.go`、`cronscale-agent/main.go` | `dsn` 默认设置为空，public 仓库不提交数据库账号、密码、地址等敏感信息 |
| 注册定时任务 | `internal/controller/cronscale_controller.go` | `ReconcileReplicas` 注册扩容和缩容任务，`ReconcileImage` 注册镜像预热任务 |
| 清理定时任务 | `internal/controller/cronscale_controller.go` | 删除 `CronScale` 资源时，通过 `taskMap` 找到已经注册的 cron task id 并移除 |
| HPA 兼容处理 | `internal/controller/cronscale_controller.go`、`pkg/hpa.go` | 如果业务存在 HPA，优先调整 HPA 的 `minReplicas` 和 `maxReplicas`，避免 HPA 覆盖副本数 |
| Deployment 副本调整 | `internal/controller/cronscale_controller.go` | 没有 HPA 或 HPA 处理完成后，通过 Deployment 的 Scale 子资源更新副本数 |
| agent 镜像上报 | `cronscale-agent/main.go` | agent 每 5 秒读取节点 containerd 镜像列表，并写入 MySQL |
| 镜像预热节点计算 | `internal/controller/cronscale_controller.go` | Operator 查询 MySQL 中已有目标镜像的节点，再计算出还需要预热的节点列表 |
| 镜像预热 Job | `internal/controller/cronscale_controller.go` | 对缺少镜像的节点创建临时 Job，在节点上执行 `crictl pull` |
| Job 自动回收 | `internal/controller/cronscale_controller.go` | 轮询 Job 状态，任务完成后删除临时 Job，避免资源残留 |

这里需要注意，示例代码为了适合公开仓库，已经将 kubeconfig、MySQL DSN、私有镜像仓库地址、内网节点 IP 等信息替换为空值或示例值。实际部署时建议通过 Kubernetes Secret、ConfigMap 或环境变量注入。

## 核心步骤

### 部署CronScale-Operator镜像

```bash
make docker-build docker-push IMG=<registry>/cronscale-operator:<tag>
```

示例：

```bash
make docker-build docker-push IMG=example.com/library/cronscale-operator:1.0.0
```

#### 参数解释

| 参数 | 说明 |
| --- | --- |
| `make docker-build` | 根据项目根目录的 `Dockerfile` 构建 Operator 镜像 |
| `make docker-push` | 将构建好的 Operator 镜像推送到镜像仓库 |
| `IMG` | 指定镜像完整地址，例如 `example.com/library/cronscale-operator:1.0.0` |
| `<registry>` | 镜像仓库地址 |
| `<tag>` | 镜像版本号 |

### 安装CronScale CRD

```bash
make install
```

#### 参数解释

| 参数 | 说明 |
| --- | --- |
| `make install` | 安装项目中的 CRD 资源 |
| `config/crd` | CRD 配置目录 |
| `cronscales.application.liuchong.cn` | 安装后生成的自定义资源类型 |

### 部署CronScale-Operator

```bash
make deploy IMG=<registry>/cronscale-operator:<tag>
```

示例：

```bash
make deploy IMG=example.com/library/cronscale-operator:1.0.0
```

#### 参数解释

| 参数 | 说明 |
| --- | --- |
| `make deploy` | 部署 Controller Manager、RBAC、Webhook 等资源 |
| `IMG` | 指定 Controller Manager 使用的镜像 |
| `config/default` | Operator 默认部署配置 |
| `config/manager` | Controller Manager Deployment 配置 |
| `config/rbac` | Operator 访问 Kubernetes API 所需权限 |

### 部署cronscale-agent

如果只使用定时扩缩容能力，可以跳过这一步。

如果需要镜像预热能力，需要部署 `cronscale-agent`，让每个节点定时上报本机已有镜像。

```bash
kubectl apply -f config/samples/Cronscale-agent.yaml
```

#### 参数解释

| 参数 | 说明 |
| --- | --- |
| `kubectl apply` | 创建或更新 Kubernetes 资源 |
| `config/samples/Cronscale-agent.yaml` | agent 的 DaemonSet 配置文件 |
| `DaemonSet` | 保证每个节点运行一个 agent Pod |
| `/run/containerd` | 挂载宿主机 containerd socket |
| `/data/containerd` | 挂载宿主机 containerd 数据目录 |
| `nodeName` | 通过 Downward API 获取当前节点名称 |

agent 的主要逻辑：

```text
每 5 秒扫描一次节点本地镜像
        |
        v
读取 containerd 镜像列表
        |
        v
将 nodeName 和 imageName 写入 MySQL
```

### 创建CronScale资源

```bash
kubectl apply -f config/samples/application_v1_cronscale.yaml
```

示例：

```yaml
apiVersion: application.liuchong.cn/v1
kind: CronScale
metadata:
  name: nginx
  namespace: liuchong
  labels:
    env: test
spec:
  foo: CronScale-test
  minus:
    targetReplicas: 1
    scaleTime: "0 */3 * * * ?"
  add:
    targetReplicas: 5
    scaleTime: "0 */5 * * * ?"
  deploymentName: "nginx"
  imagePullTime: "0 */1 * * * ?"
```

#### 参数解释

| 参数 | 说明 |
| --- | --- |
| `apiVersion` | CRD API 版本，当前为 `application.liuchong.cn/v1` |
| `kind` | 自定义资源类型，当前为 `CronScale` |
| `metadata.name` | CronScale 资源名称 |
| `metadata.namespace` | CronScale 所在命名空间，需要和目标 Deployment 保持一致 |
| `spec.deploymentName` | 需要被扩缩容的 Deployment 名称 |
| `spec.add.targetReplicas` | 扩容后的目标副本数 |
| `spec.add.scaleTime` | 扩容执行时间，支持秒级 cron 表达式 |
| `spec.minus.targetReplicas` | 缩容后的目标副本数 |
| `spec.minus.scaleTime` | 缩容执行时间，支持秒级 cron 表达式 |
| `spec.imagePullTime` | 镜像预热执行时间，支持秒级 cron 表达式 |

上面的配置含义：

| 配置 | 含义 |
| --- | --- |
| `add.targetReplicas: 5` | 扩容时将 `nginx` 调整到 5 个副本 |
| `add.scaleTime: "0 */5 * * * ?"` | 每 5 分钟执行一次扩容任务 |
| `minus.targetReplicas: 1` | 缩容时将 `nginx` 调整到 1 个副本 |
| `minus.scaleTime: "0 */3 * * * ?"` | 每 3 分钟执行一次缩容任务 |
| `imagePullTime: "0 */1 * * * ?"` | 每 1 分钟执行一次镜像预热任务 |

### 扩缩容执行逻辑

Operator 监听到 `CronScale` 后，会注册两个副本调整任务：

| 任务 | 触发字段 | 执行动作 |
| --- | --- | --- |
| 扩容任务 | `spec.add.scaleTime` | 将目标 Deployment 调整到 `spec.add.targetReplicas` |
| 缩容任务 | `spec.minus.scaleTime` | 将目标 Deployment 调整到 `spec.minus.targetReplicas` |

如果目标 Deployment 没有配置 HPA，Operator 会直接更新 Deployment 的 Scale 子资源。

如果目标 Deployment 配置了 HPA，Operator 会优先更新 HPA 的 `minReplicas` 和 `maxReplicas`，避免 HPA 把副本数重新调整回去。

### 镜像预热执行逻辑

镜像预热主要用于减少扩容时的镜像拉取时间。

执行流程：

```text
获取 Deployment 当前镜像
        |
        v
查询 MySQL 中已有该镜像的节点
        |
        v
计算还缺少镜像的节点
        |
        v
创建 Job 到指定节点执行 crictl pull
        |
        v
Job 完成后自动删除
```

预热命令：

```bash
crictl pull <deployment-image>
```

#### 参数解释

| 参数 | 说明 |
| --- | --- |
| `deployment-image` | 目标 Deployment 当前使用的镜像 |
| `crictl pull` | 使用 containerd 拉取镜像 |
| `NodeAffinity` | 控制预热 Job 只调度到缺少镜像的节点 |
| `PodAntiAffinity` | 控制多个预热 Pod 尽量分散到不同节点 |
| `cronscale-task-xxxxx` | Operator 创建的临时预热 Job |

## 结果验证

### CronScale CRD安装状况

```bash
kubectl get crd | grep cronscales
```

正常结果：

```text
cronscales.application.liuchong.cn
```

### Operator运行状况

```bash
kubectl get pod -n cronscale-system
```

正常结果：

```text
NAME                                                   READY   STATUS    RESTARTS   AGE
cronscale-operator-controller-manager-xxxxxx           2/2     Running   0          1m
```

### cronscale-agent运行状况

```bash
kubectl get ds -n liuchong
```

正常结果：

```text
NAME              DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE
cronscale-agent   3         3         3       3            3
```

### CronScale资源创建状况

```bash
kubectl get crs -n liuchong
```

正常结果：

```text
NAME    AGE
nginx   1m
```

### Deployment扩容状况

```bash
kubectl get deploy nginx -n liuchong
```

到达扩容时间后，副本数会变成 `add.targetReplicas` 中配置的数量。

```text
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   5/5     5            5           10m
```

### Deployment缩容状况

```bash
kubectl get deploy nginx -n liuchong
```

到达缩容时间后，副本数会变成 `minus.targetReplicas` 中配置的数量。

```text
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   1/1     1            1           15m
```

### 镜像预热状况

```bash
kubectl get job -n liuchong
```

镜像预热执行时，会生成临时 Job。

```text
NAME                  COMPLETIONS   DURATION   AGE
cronscale-task-abcde  3/3           20s        20s
```

查看 Job 日志：

```bash
kubectl logs -n liuchong job/cronscale-task-abcde
```

正常结果：

```text
pull done....
```

查看 Operator 日志：

```bash
kubectl logs -n cronscale-system deploy/cronscale-operator-controller-manager -f
```

正常结果：

```text
镜像预热完成
清理Job Task完成
```

## 常见问题

### CronScale资源创建失败

可能原因：

| 原因 | 处理方式 |
| --- | --- |
| `add.targetReplicas` 小于或等于 `minus.targetReplicas` | 调整扩容副本数，保证扩容副本数大于缩容副本数 |
| `deploymentName` 为空 | 填写需要扩缩容的 Deployment 名称 |
| `imagePullTime` 为空 | 填写镜像预热 cron 表达式 |
| Webhook 未正常运行 | 检查 Controller Manager Pod 和 webhook 配置 |

### 镜像预热Job没有创建

可能原因：

| 原因 | 处理方式 |
| --- | --- |
| 没有部署 `cronscale-agent` | 先部署 `config/samples/Cronscale-agent.yaml` |
| MySQL 中没有节点镜像数据 | 检查 agent 日志和数据库连接 |
| 节点不是 containerd 运行时 | 根据实际运行时调整预热逻辑 |
| containerd 路径不一致 | 检查 `/run/containerd`、`/data/containerd` 挂载路径 |

## 总结

`CronScale-Operator` 的核心思路是把定时扩缩容规则抽象成 Kubernetes 自定义资源，由 Operator 负责监听资源并执行定时任务。

对于流量周期比较明显的业务，只需要维护一份 `CronScale` YAML，就可以实现定时扩容、定时缩容和镜像预热。相比人工操作，这种方式更加稳定，也更容易接入 GitOps 流程。
