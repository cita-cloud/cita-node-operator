# CITA Node Operator

> **注意:** `MAIN` 分支在开发过程中可能处于不稳定状态。

## 简介

`CITA Node Operator`提供了一个简单而可靠的解决方案，以可扩展和高可用的方式将完整的`CITA-Cloud`链部署到目标 [Kubernetes](https://kubernetes.io/) 集群。
`CITA Node Operator`在`Kubernetes`之上定义了多个 [自定义资源](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)，然后可以以声明式的方式使用 `Kubernetes API`来管理`CITA-Cloud`，以确保链的可扩展性和高可用。

## 自定义资源

### CitaNode

不可用

### BlockHeightFallback

消块：块高回退。将一个或多个节点的块高回退到指定的高度。