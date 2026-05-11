# CronScale Operator

`CronScale Operator` 是一个基于 Kubebuilder 的 Kubernetes Operator，用于把业务的定时扩容、定时缩容和镜像预热规则声明成 `CronScale` 自定义资源。

它主要解决两类问题：

| 能力 | 说明 |
| --- | --- |
| 定时扩缩容 | 按 cron 时间调整目标 Deployment 的副本数；如果目标 Deployment 配置了 HPA，会优先同步 HPA 的 `minReplicas` / `maxReplicas` |
| 镜像预热 | 在扩容前把目标 Deployment 使用的镜像提前拉取到缺少该镜像的节点上，减少扩容时 Pod 等待镜像下载的时间 |

详细设计和完整部署说明见 [CronScale-Operator.md](./CronScale-Operator.md)。

## 运行依赖

| 依赖 | 建议版本 | 用途 |
| --- | --- | --- |
| Kubernetes | v1.11.3+ | 运行 CRD、Webhook、Controller、Job、DaemonSet |
| Go | v1.21.0+ | 本地构建和开发 |
| Docker / Podman | v17.03+ | 构建并推送镜像 |
| kubectl | v1.11.3+ | 安装和管理集群资源 |
| containerd | 按集群实际版本 | 镜像预热依赖 containerd socket |
| MySQL | 5.7+ / 8.x | 存储各节点已有镜像清单 |

## 初始化参数

公开仓库中没有提交真实集群凭据、数据库连接串和私有镜像地址。部署前至少需要准备下面这些参数：

| 参数 | 示例 | 说明 |
| --- | --- | --- |
| `IMG` | `registry.example.com/cronscale-operator:v0.1.0` | Operator 镜像地址，供 `make docker-build docker-push deploy` 使用 |
| `AGENT_IMG` | `registry.example.com/cronscale-agent:v0.1.0` | `config/samples/Cronscale-agent.yaml` 中的 agent 镜像 |
| `TASK_IMG` | `registry.example.com/cronscale-task:3.16` | 镜像预热 Job 使用的工具镜像，当前代码中示例值为 `example.com/library/cronscale-task:3.16` |
| `NAMESPACE` | `default` / `liuchong` | CronScale、目标 Deployment、agent 所在命名空间 |
| `MYSQL_DSN` | `user:password@tcp(mysql:3306)/cronscale?parseTime=true` | Operator 和 agent 连接 MySQL 的 DSN；公开版本代码中留空 |
| `KUBECONFIG` | `~/.kube/config` | 本地运行 controller 时使用；集群内运行时建议使用 ServiceAccount |

MySQL 建议表结构：

```sql
CREATE TABLE cronscale_agent (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  node_ip VARCHAR(255) NOT NULL,
  image_name VARCHAR(512) NOT NULL,
  update_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_node_image (node_ip, image_name)
);
```

## 快速开始

1. 构建并推送 Operator 镜像：

```sh
make docker-build docker-push IMG=registry.example.com/cronscale-operator:v0.1.0
```

2. 安装 CRD：

```sh
make install
```

3. 部署 Operator：

```sh
make deploy IMG=registry.example.com/cronscale-operator:v0.1.0
```

4. 如需镜像预热，先构建并推送 agent 镜像，再替换 `config/samples/Cronscale-agent.yaml` 中的镜像、命名空间和 containerd 挂载路径：

```sh
cd cronscale-agent
GOOS=linux GOARCH=amd64 go build -o cronscale-agent .
docker build -t registry.example.com/cronscale-agent:v0.1.0 .
docker push registry.example.com/cronscale-agent:v0.1.0
cd ..
```

部署 agent：

```sh
kubectl apply -f config/samples/Cronscale-agent.yaml
```

5. 创建 CronScale 资源：

```sh
kubectl apply -f config/samples/application_v1_cronscale.yaml
```

示例资源：

```yaml
apiVersion: application.liuchong.cn/v1
kind: CronScale
metadata:
  name: nginx
  namespace: liuchong
spec:
  deploymentName: nginx
  add:
    targetReplicas: 5
    scaleTime: "0 */5 * * * ?"
  minus:
    targetReplicas: 1
    scaleTime: "0 */3 * * * ?"
  imagePullTime: "0 */1 * * * ?"
```

字段说明：

| 字段 | 是否必填 | 说明 |
| --- | --- | --- |
| `metadata.namespace` | 是 | 需要和目标 Deployment 所在命名空间一致 |
| `spec.deploymentName` | 是 | 需要被扩缩容的 Deployment 名称 |
| `spec.add.targetReplicas` | 是 | 扩容后的目标副本数，必须大于缩容副本数 |
| `spec.add.scaleTime` | 是 | 扩容执行时间，使用秒级 cron 表达式 |
| `spec.minus.targetReplicas` | 是 | 缩容后的目标副本数 |
| `spec.minus.scaleTime` | 是 | 缩容执行时间，使用秒级 cron 表达式 |
| `spec.imagePullTime` | 是 | 镜像预热执行时间，使用秒级 cron 表达式 |

## 验证

```sh
kubectl get crd | grep cronscales
kubectl get pod -n cronscale-operator-system
kubectl get crs -n <namespace>
kubectl get deploy <deployment-name> -n <namespace>
kubectl logs -n cronscale-operator-system deploy/cronscale-operator-controller-manager -f
```

## 清理

```sh
kubectl delete -f config/samples/application_v1_cronscale.yaml
kubectl delete -f config/samples/Cronscale-agent.yaml
make undeploy
make uninstall
```

## 注意事项

- 当前公开版本代码中的 `kubeconfig`、MySQL `dsn` 和部分镜像地址是空值或示例值，部署前需要替换或改为从 Secret / 环境变量注入。
- 镜像预热逻辑默认适配 containerd，并依赖 `/run/containerd`、`/data/containerd`、`crictl` 等宿主机路径；不同集群需要按实际运行时调整。
- `CronScale` 资源删除后，Operator 会清理已经注册的定时任务。

## License

Copyright 2024 liuchong.

Licensed under the MIT License.
