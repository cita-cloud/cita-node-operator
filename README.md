# CITA Node Operator
English | [简体中文](./README_CN.md)

> **ATTENTIONS:** THE `MAIN` BRANCH MAY BE IN AN UNSTABLE OR EVEN BROKEN STATE DURING DEVELOPMENT.

## Overview

The CITA Node Operator provides an easy and solid solution to deploy and manage a full CITA-Cloud blockchain service stack to the target [Kubernetes](https://kubernetes.io/) clusters in a scalable and high-available way. The CITA Node Operator defines multiple custom resources on top of Kubernetes [Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/). The Kubernetes API can then be used in a declarative way to manage CITA-Cloud deployment stack and ensure its scalability and high-availability operation.

## Getting started
### Deploy CITA Node Operator

- Add CITA Node Operator repository
```shell
helm repo add cita-node-operator https://cita-cloud.github.io/cita-node-operator
```

- Create the namespace to install CITA Node Operator(if needed)
```shell
kubectl create ns cita
```

- Install CITA Node Operator
```shell
helm install cita-node-operator cita-node-operator/cita-node-operator -n=cita
```

- Verify the installation
```shell
kubectl get pod -ncita | grep cita-node-operator
```
The expected output is as follows:
```shell
NAME                                         READY   STATUS    RESTARTS   AGE
cita-node-operator-757fcf4466-gllsn         1/1     Running   0          45h
```

## Custom Resource Definition

### CitaNode

Currently unavailable

### BlockHeightFallback

Block height fallback. Roll back the block height of a node to the specified height.

#### Create a BlockHeightFallback job

> Main step：shutdown node -> fallback -> start node

```yaml
apiVersion: citacloud.rivtower.com/v1
kind: BlockHeightFallback
metadata:
  name: blockheightfallback-sample
  namespace: app-rivspace
spec:
  # The chain name for node（required）
  chain: chain-734116911817822208
  # The node name (required)
  node: chain-734116911817822208-0
  # Block height you want to fallback (required)
  blockHeight: 2000000
  # How nodes are deployed: python/cloud-config/helm (required)
  deployMethod: cloud-config
  # The image of job's pod（optional，default: latest）
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:latest
  # The image's pull policy（optional, IfNotPresent/Always/Never, default: IfNotPresent）
  pullPolicy: Always
  # How long to delete finished job resources: seconds unit(optional, default: 30s)
  ttlSecondsAfterFinished: 30
  # Whether the job's pod has affinity with the blockchain node(optional, default: false)
  podAffinityFlag: true
  # weather or not delete consensus data when block height fallback (optional, default: false)
  deleteConsensusData: true
```

### Backup

Backup: Backup the data of a node to a new PVC.

#### Create a Backup job

> Main step：shutdown node -> backup -> start node

```yaml
apiVersion: citacloud.rivtower.com/v1
kind: Backup
metadata:
  name: backup-sample
  namespace: app-rivspace
spec:
  # The chain name for node（required）
  chain: chain-734116911817822208
  # The node name (required)
  node: chain-734116911817822208-0
  # How nodes are deployed: python/cloud-config/helm (required)
  deployMethod: python
  # The storage class used by the backup PVC (optional，default: the same with node's storage class)
  storageClass: nas-client-provisioner
  # If set this field, operator will set the pvc size to the value you specify (optional)
  pvcSize: 5Gi
  # The actions during backup (optional, default: StopAndStart. Direct: direct data copy; StopAndStart: Shut down the node before backup and start the node after backup)
  action: StopAndStart
  # The image of job's pod（optional，default: latest）
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:latest
  # The image's pull policy (optional, IfNotPresent/Always/Never, default: IfNotPresent)
  pullPolicy: IfNotPresent
  # How long to delete finished job resources: seconds unit (optional, default: 30s)
  ttlSecondsAfterFinished: 30
  # Whether the job's pod has affinity with the blockchain node (optional, default: false)
  podAffinityFlag: true
```

### Snapshot

Snapshot: Take a snapshot of a node and back up the snapshot data to a new PVC.

#### Create a Snapshot job

> Main step：shutdown node -> snapshot -> start node

```yaml
apiVersion: citacloud.rivtower.com/v1
kind: Snapshot
metadata:
  name: snapshot-sample
  namespace: app-rivspace
spec:
  # The chain name for node（required）
  chain: test-chain-zenoh-bft
  # The node name (required)
  node: test-chain-zenoh-bft-node0
  # Block height you want to snapshot (required)
  blockHeight: 30
  # How nodes are deployed: python/cloud-config/helm (required)
  deployMethod: cloud-config
  # The storage class used by the snapshot PVC (optional，default: the same with node's storage class)
  storageClass: nas-client-provisioner
  # The image of job's pod（optional，default: latest）
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:latest
  # The image's pull policy（optional, IfNotPresent/Always/Never, default: IfNotPresent）
  pullPolicy: IfNotPresent
  # How long to delete finished job resources: seconds unit(optional, default: 30s)
  ttlSecondsAfterFinished: 30
  # Whether the job's pod has affinity with the blockchain node(optional, default: false)
  podAffinityFlag: true
```

### Restore

Restore: data recovery of the node based on the specified backup or snapshot.

#### Restore from backup

>  Main step：shutdown node -> data copy -> start node

```yaml
apiVersion: citacloud.rivtower.com/v1
kind: Restore
metadata:
  name: restore-sample
  namespace: app-rivspace
spec:
  # The chain name for node (required)
  chain: chain-734116911817822208
  # The node name (required)
  node: chain-734116911817822208-0
  # How nodes are deployed: python/cloud-config/helm (required)
  deployMethod: python
  # backup cr (required)
  backup: backup-sample
  # The actions during restore (optional, default: StopAndStart. Direct: direct data copy; StopAndStart: Shut down the node before restore and start the node after restore)
  action: StopAndStart
  # The image of job's pod (optional，default: latest)
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:latest
  # The image's pull policy (optional, IfNotPresent/Always/Never, default: IfNotPresent)
  pullPolicy: IfNotPresent
  # How long to delete finished job resources: seconds unit (optional, default: 30s)
  ttlSecondsAfterFinished: 30
  # Whether the job's pod has affinity with the blockchain node (optional, default: false)
  podAffinityFlag: true
  # weather or not delete consensus data when restore (optional, default: false)
  deleteConsensusData: true
```

#### Restore from snapshot

> Main step：shutdown node -> executor snapshot recovery command -> start node

```yaml
 apiVersion: citacloud.rivtower.com/v1
kind: Restore
metadata:
  name: restore-sample-cloud-config
  namespace: app-rivspace
spec:
  # The chain name for node (required)
  chain: test-chain-zenoh-bft
  # The node name (required)
  node: test-chain-zenoh-bft-node0
  # How nodes are deployed: python/cloud-config/helm (required)
  deployMethod: cloud-config
  # snapshot cr (required)
  snapshot: snapshot-sample
  # The actions during restore (optional, default: StopAndStart. Direct: direct data copy; StopAndStart: Shut down the node before restore and start the node after restore)
  action: StopAndStart
  # The image of job's pod (optional，default: latest)
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:latest
  # The image's pull policy (optional, IfNotPresent/Always/Never, default: IfNotPresent)
  pullPolicy: IfNotPresent
  # How long to delete finished job resources: seconds unit (optional, default: 30s)
  ttlSecondsAfterFinished: 30
  # Whether the job's pod has affinity with the blockchain node (optional, default: false)
  podAffinityFlag: true
  # weather or not delete consensus data when restore (optional, default: false)
  deleteConsensusData: true
```

### Duplicate

Duplicate: Backup the data of a node to a exist PVC.

#### Create a Duplicate job

> Main step：shutdown node -> duplicate -> start node

```yaml
apiVersion: citacloud.rivtower.com/v1
kind: Duplicate
metadata:
  name: duplicate-sample
  namespace: rivspace
spec:
  # The chain name for node (required)
  chain: test-chain-zenoh-overlord
  # The node name (required)
  node: test-chain-zenoh-overlord-0
  # How nodes are deployed: python/cloud-config/helm (required)
  deployMethod: python
  # The backend you can set (required)
  backend:
    # backup target pvc name
    pvc: resource-management-nas
    # the path dir in pvc, if not exist, will create one
    path: /data/backup
  # The actions during restore (optional, default: StopAndStart. Direct: direct data copy; StopAndStart: Shut down the node before restore and start the node after restore)
  action: Direct
  # The image's pull policy (optional, IfNotPresent/Always/Never, default: IfNotPresent)
  pullPolicy: Always
  # The image of job's pod (optional，default: latest)
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:latest
  # How long to delete finished job resources: seconds unit (optional, default: 30s)
  ttlSecondsAfterFinished: 30
  # Whether the job's pod has affinity with the blockchain node (optional, default: false)
  podAffinityFlag: true
  # If you want to compress the backup file, you can set this field (optional)
  compress:
    # the compress type
    type: gzip
    # compressed package name
    file: cita.tar.gz
```

### Recover

Recover: Recover the data of a node from a exist PVC.

#### Create a Recover job

> Main step：shutdown node -> recover -> start node

```yaml
apiVersion: citacloud.rivtower.com/v1
kind: Recover
metadata:
  name: recover-sample
  namespace: rivspace
spec:
  # The chain name for node (required)
  chain: test-chain-zenoh-overlord
  # The node name (required)
  node: test-chain-zenoh-overlord-node0
  # How nodes are deployed: python/cloud-config/helm (required)
  deployMethod: cloud-config
  # The backend you can set (required)
  backend:
    # backup target pvc name
    pvc: local-pvc
    # the path dir in pvc, if not exist, will create one
    path: /data/my-chain-0
  # The actions during restore (optional, default: StopAndStart. Direct: direct data copy; StopAndStart: Shut down the node before restore and start the node after restore)
  action: StopAndStart
  # The image's pull policy (optional, IfNotPresent/Always/Never, default: IfNotPresent)
  pullPolicy: Always
  # The image of job's pod (optional，default: latest)
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:latest
  # Whether the job's pod has affinity with the blockchain node (optional, default: false)
  podAffinityFlag: true
  # How long to delete finished job resources: seconds unit (optional, default: 30s)
  ttlSecondsAfterFinished: 30
  # weather or not delete consensus data when restore (optional, default: false)
  deleteConsensusData: true
  # If you compressed when backup, you should set this field (optional)
  decompress:
    # decompress package name
    file: cita.tar.gz
```

### Switchover

Account switch: switch the accounts of two nodes.

#### Create a Switchover job

> Main step：shutdown first node -> shutdown second node -> switch account configmap volume -> start first node -> start second node

```yaml
apiVersion: citacloud.rivtower.com/v1
kind: Switchover
metadata:
  name: switchover-sample
  namespace: app-rivspace
spec:
  # The chain name for node (required)
  chain: test-chain-zenoh-bft
  # first node (required)
  sourceNode: test-chain-zenoh-bft-node0
  # second node (required)
  destNode: test-chain-zenoh-bft-node1
  # The image of job's pod（optional，default: latest）
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:latest
  # The image's pull policy（optional, IfNotPresent/Always/Never, default: IfNotPresent）
  pullPolicy: IfNotPresent
  # How long to delete finished job resources: seconds unit(optional, default: 30s)
  ttlSecondsAfterFinished: 30
```

### ChangeOwner

Permission conversion: used to change node data file permissions during upgrade(uid, gid).

#### Create a ChangeOwner job
```yaml
apiVersion: citacloud.rivtower.com/v1
kind: ChangeOwner
metadata:
  name: changeowner-sample
  namespace: app-rivspace
spec:
  # The chain name for node (required)
  chain: test-chain-zenoh-bft
  # The node name (required)
  node: test-chain-zenoh-bft-node0
  # How nodes are deployed: python/cloud-config/helm (required)
  deployMethod: cloud-config
  # chown uid（optional, default: 1000)
  uid: 1000
  # chown gid (optional, default: 1000)
  gid: 1000
  # The image of job's pod（optional，default: latest）
  image: registry.devops.rivtower.com/cita-cloud/cita-node-job:latest
  # The image's pull policy（optional, IfNotPresent/Always/Never, default: IfNotPresent）
  pullPolicy: IfNotPresent
  # How long to delete finished job resources: seconds unit(optional, default: 30s)
  ttlSecondsAfterFinished: 30
  # Whether the job's pod has affinity with the blockchain node (optional, default: false)
  podAffinityFlag: false
```
