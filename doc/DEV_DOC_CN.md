# 开发须知

## 说明
当前每一个运维操作定义一个`CRD`，当你通过创建该`CRD`的`CR`并应用到集群时，相应的控制器会生成一个Job资源，该Job会立即生成一个工作Pod，对相应的节点进行操作。
程序分为管理端和执行端。管理端为Operator逻辑，执行端为Job创建出的Pod的逻辑。

## 管理端开发

以下以新建一个运维操作Operator为例

### 环境要求
- golang >= 1.17
- kubebuilder >= 3.3.0
- kubernetes >= 1.18

### 下载项目
```shell
git clone https://github.com/cita-cloud/cita-node-operator.git
```

### 创建新的api资源
```shell
cd cita-node-operator
kubebuilder create api --group citacloud --version v1 --kind XXXX
```

### 添加控制循环逻辑
在controller包下的XXXX_controller.go文件中，编写对应的Reconcile逻辑
主要是创建Job资源(设置command，args，挂载相应的卷)

### 编写cita-node-cli逻辑
具体的逻辑可以参考`blockheightfallback_controller.go`

### 编写测试
使用envtest来编写集成测试
具体的测试可以参考`blockheightfallback_controller_test.go`

### 执行测试
```shell
# 项目根目录下
make test
```

### 本地运行
```shell
go run main.go
```

## 执行端开发

执行端业务主要在pkg目录下，是一个有`cobra`库封装的命令行工具

### 添加运维执行逻辑
可在`pkg/chain`包的`chain.go`文件的`Chain interface`中添加相应的运维操作方法，并在相应的链中实现该功能

### 添加命令封装逻辑
可在`pkg/cmd`包下添加子命令文件，并在`root.go`中添加子命令的调用

### 本地运行
```shell
go run pkg/main/main.go
```
