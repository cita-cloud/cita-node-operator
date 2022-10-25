# CITA Node Operator

> **注意:** `MAIN` 分支在开发过程中可能处于不稳定状态。

## 简介

`CITA Node Operator`提供了一个简单而可靠的解决方案，以可扩展和高可用的方式将完整的`CITA-Cloud`链部署到目标 [Kubernetes](https://kubernetes.io/) 集群。
`CITA Node Operator`在`Kubernetes`之上定义了多个 [自定义资源](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)，然后可以以声明式的方式使用 `Kubernetes API`来管理`CITA-Cloud`，以确保链的可扩展性和高可用。

## 安装

### 部署`CITA Node Operator`

- 添加`CITA Node Operator`Chart 仓库
    ```shell
    helm repo add cita-node-operator https://cita-cloud.github.io/cita-node-operator
    ```
- 创建安装`CITA Node Operator`的命名空间（如果需要的话）
    ```shell
    kubectl create ns cita
    ```
- 安装`CITA Node Operator`
    ```shell
    helm install cita-node-operator cita-node-operator/cita-node-operator -n=cita
    ```
- 验证安装
    ```shell
    kubectl get pod -ncita | grep cita-node-operator
    ```
    期望有如下输出:
    ```shell
    NAME                                         READY   STATUS    RESTARTS   AGE
    cita-node-operator-757fcf4466-gllsn         1/1     Running   0          45h
    ```

## 自定义资源

### CitaNode

当前不可用

### BlockHeightFallback

消块：块高回退。将一个或多个节点的块高回退到指定的高度。

#### 创建一个BlockHeightFallback任务

> 对一个节点进行消块：停节点->消块->启节点

```yaml
apiVersion: citacloud.rivtower.com/v1
kind: BlockHeightFallback
metadata:
  name: blockheightfallback-sample
  namespace: app-rivspace
spec:
  # 备份节点对应的链名（必选）
  chain: chain-734116911817822208
  # 备份节点的节点名（必选）
  node: chain-734116911817822208-0
  # 块高
  blockHeight: 2000000
  # 消块节点的部署方式：python/cloud-config/helm（必选）
  deployMethod: cloud-config
  # 执行备份任务的镜像（可选，不指定默认会取latest镜像）
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:v0.0.2
  # 上述镜像的拉取策略（可选，IfNotPresent/Always/Never，不指定默认会取IfNotPresent）
  pullPolicy: Always
  # 多长时间后删除job资源：秒为单位(可选： 默认30s)
  ttlSecondsAfterFinished: 30
  # job产生的pod是否和区块链节点具有亲和性(可选： 默认false)
  # pvc为RWO时，只允许一个work node挂载pv，这是可能会导致job pod hang住，所以这种情况下需要设置该字段为：true
  podAffinityFlag: true
```

### Backup

备份：将一个节点的数据备份至新的PVC中。

#### 创建一个备份

> 对一个节点进行备份：停节点->备份->启节点

```yaml
apiVersion: citacloud.rivtower.com/v1
kind: Backup
metadata:
  # 备份名
  name: backup-sample
  # 备份CR所在的namespace
  namespace: app-rivspace
spec:
  # 备份节点对应的链名（必选）
  chain: chain-734116911817822208
  # 备份节点的节点名（必选）
  node: chain-734116911817822208-0
  # 备份节点的部署方式：python/cloud-config/helm（必选）
  deployMethod: python
  # 备份PVC所属的storage class（可选，不指定会默认和备份节点的storage class保持一致）
  storageClass: nas-client-provisioner
  # 备份时相应的动作（可选，不指定默认为StopAndStart，Direct:直接进行数据拷贝恢复/StopAndStart:恢复前关闭节点，恢复后启动节点）
  action: StopAndStart
  # 执行备份任务的镜像（可选，不指定默认会取latest镜像）
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:v0.0.2
  # 上述镜像的拉取策略（可选，IfNotPresent/Always/Never，不指定默认会取IfNotPresent）
  pullPolicy: IfNotPresent
  # 多长时间后删除job资源：秒为单位(可选： 默认30s)
  ttlSecondsAfterFinished: 30
  # job产生的pod是否和区块链节点具有亲和性(可选： 默认false)
  # pvc为RWO时，只允许一个work node挂载pv，这是可能会导致job pod hang住，所以这种情况下需要设置该字段为：true
  podAffinityFlag: true
```

### Snapshot

快照：对一个节点执行快照，并将快照数据备份至新的PVC中。

#### 创建一个快照

> 对一个节点进行快照：停节点->执行快照命令->启节点

```yaml
apiVersion: citacloud.rivtower.com/v1
kind: Snapshot
metadata:
  # 快照名
  name: snapshot-sample
  # 快照CR所在的namespace
  namespace: app-rivspace
spec:
  # 快照节点对应的链名（必选）
  chain: test-chain-zenoh-bft
  # 快照节点的节点名（必选）
  node: test-chain-zenoh-bft-node0
  # 指定块高执行快照
  blockHeight: 30
  # 快照节点的部署方式：python/cloud-config/helm（必选）
  deployMethod: cloud-config
  # 快照PVC所属的storage class（可选，不指定会默认和快照节点的storage class保持一致）
  storageClass: nas-client-provisioner
  # 执行快照任务的镜像（可选，不指定默认会取v0.0.2镜像）
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:v0.0.2
  # 上述镜像的拉取策略（可选，IfNotPresent/Always/Never，不指定默认会取IfNotPresent）
  pullPolicy: IfNotPresent
  # 多长时间后删除job资源：秒为单位(可选： 默认30s)
  ttlSecondsAfterFinished: 30
  # job产生的pod是否和区块链节点具有亲和性(可选： 默认false)
  # pvc为RWO时，只允许一个work node挂载pv，这是可能会导致job pod hang住，所以这种情况下需要设置该字段为：true
  podAffinityFlag: true
```

### Restore

恢复：根据指定的备份或快照进行节点的数据恢复。

#### 根据备份恢复

>  对一个节点进行数据恢复：停节点->恢复拷贝->启节点

```yaml
apiVersion: citacloud.rivtower.com/v1
kind: Restore
metadata:
  # 恢复名
  name: restore-sample
  # Restore CR所在的namespace
  namespace: app-rivspace
spec:
  # 待恢复节点对应的链名（必选）
  chain: chain-734116911817822208
  # 待恢复节点的节点名（必选）
  node: chain-734116911817822208-0
  # 待恢复节点的部署方式：python/cloud-config/helm（必选）
  deployMethod: python
  # 对应的备份CR (必选)
  backup: backup-sample
  # 恢复时相应的动作（可选，不指定默认为StopAndStart，Direct:直接进行数据拷贝恢复/StopAndStart:恢复前关闭节点，恢复后启动节点）
  action: StopAndStart
  # 执行恢复任务的镜像（可选，不指定默认会取latest镜像）
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:v0.0.2
  # 上述镜像的拉取策略（可选，IfNotPresent/Always/Never，不指定默认会取IfNotPresent）
  pullPolicy: IfNotPresent
  # 多长时间后删除job资源：秒为单位
  ttlSecondsAfterFinished: 30
  # job产生的pod是否和区块链节点具有亲和性(可选： 默认false)
  # pvc为RWO时，只允许一个work node挂载pv，这是可能会导致job pod hang住，所以这种情况下需要设置该字段为：true
  podAffinityFlag: true
```

#### 根据快照恢复

> 对一个节点进行快照恢复：停节点->执行快照恢复命令->启节点

```yaml
 apiVersion: citacloud.rivtower.com/v1
kind: Restore
metadata:
  name: restore-sample-cloud-config
  namespace: app-rivspace
spec:
  # 需要恢复的节点对应的链名（必选）
  chain: test-chain-zenoh-bft
  # 需要恢复的节点名（必选）
  node: test-chain-zenoh-bft-node0
  # 恢复节点的部署方式：python/cloud-config/helm（必选）
  deployMethod: cloud-config
  # 对应的快照CR (必选)
  snapshot: snapshot-sample
  # 快照恢复时相应的动作（可选，不指定默认为StopAndStart，Direct:直接进行快照恢复命令/StopAndStart:快照恢复前关闭节点，快照恢复后启动节点）
  action: StopAndStart
  # 执行快照恢复任务的镜像（可选，不指定默认会取v0.0.2镜像）
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:v0.0.2
  # 上述镜像的拉取策略（可选，IfNotPresent/Always/Never，不指定默认会取IfNotPresent）
  pullPolicy: IfNotPresent
  # 多长时间后删除job资源：秒为单位(可选： 默认30s)
  ttlSecondsAfterFinished: 30
  # job产生的pod是否和区块链节点具有亲和性(可选： 默认false)
  # pvc为RWO时，只允许一个work node挂载pv，这是可能会导致job pod hang住，所以这种情况下需要设置该字段为：true
  podAffinityFlag: true
```

### Switchover

账号切换：将两个节点的账号进行切换。

#### 创建Switchover任务

>   对两个节点进行Switchover：停第一个节点->停第二个节点->交换相应的account cm volume->启节第一个节点-> 启动第二个节点

```yaml
apiVersion: citacloud.rivtower.com/v1
kind: Switchover
metadata:
  name: switchover-sample
  namespace: app-rivspace
spec:
  # 需要切换的对应的链名（必选）
  chain: test-chain-zenoh-bft
  # 节点一（必选）
  sourceNode: test-chain-zenoh-bft-node0
  # 节点二（必选）
  destNode: test-chain-zenoh-bft-node1
  # 执行快照恢复任务的镜像（可选，不指定默认会取v0.0.2镜像）
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:v0.0.2
  # 上述镜像的拉取策略（可选，IfNotPresent/Always/Never，不指定默认会取IfNotPresent）
  pullPolicy: IfNotPresent
  # 多长时间后删除job资源：秒为单位(可选： 默认30s)
  ttlSecondsAfterFinished: 30
```

### ChangeOwner

权限转化：用于升级时改变节点数据文件权限(uid, gid)。

#### 创建一个Chown任务
```yaml
apiVersion: citacloud.rivtower.com/v1
kind: ChangeOwner
metadata:
  name: changeowner-sample
  namespace: app-rivspace
spec:
  # 需要Chown的对应的链名（必选）
  chain: test-chain-zenoh-bft
  # 需要Chown的对应的节点（必选）
  node: test-chain-zenoh-bft-node0
  # 恢复节点的部署方式：python/cloud-config（必选）
  deployMethod: cloud-config
  # chown的uid（可选：默认为1000）
  uid: 1000
  # chown的gid（可选：默认为1000）
  gid: 1000
  # 执行chown任务的镜像（可选，不指定默认会取v0.0.2镜像）
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:v0.0.2
  # 上述镜像的拉取策略（可选，IfNotPresent/Always/Never，不指定默认会取IfNotPresent）
  pullPolicy: IfNotPresent
  # 多长时间后删除job资源：秒为单位(可选： 默认30s)
  ttlSecondsAfterFinished: 30
  # job产生的pod是否和区块链节点具有亲和性(可选： 默认false)
  # pvc为RWO时，只允许一个work node挂载pv，这是可能会导致job pod hang住，所以这种情况下需要设置该字段为：true
  podAffinityFlag: false
```