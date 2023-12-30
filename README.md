## cluster-config-operator 简易型集群配置控制器

### 项目思路与设计
设计背景：k8s 集群中有原生的存储配置文件 ConfigMap,Secret 等资源，但其中有个问题：ConfigMap Secret 等资源是 Namespace scoped 维度，
这代表 如果集群内所有的 namespace 都要使用某一配置时，需要所有 namespace 都创建同一资源，非常麻烦。本项目在此需求上，基于 k8s 的扩展功能，实现 ClusterConfig 的自定义资源，做出一个自动监听配置文件的 operator 应用。


```yaml
apiVersion: api.practice.com/v1alpha1
kind: ClusterConfig
metadata:
  name: cluster-config-configmaps
  namespace: default
spec:
  configType: configmaps          # 支持 k8s 中 configmaps secrets 资源对象，需要自行设置
  namespaceList: default,test     # 支持填入多个 namespace, ex: default, example1, example2 写法，请用逗号隔开，
                                  # 在填写后请确定此 namespace 确实存在，否则会报错
                                  # 支持 all 字段，默认会在所有 namespace 下都创建该类型资源
  # 按照 k8s 原生的 configmaps secrets 填写即可
  data:
    player_initial_lives: "3"
    ui_properties_file_name: "user-interface.properties"
    game.properties: |
      enemy.types=aliens,monsters
      player.maximum-lives=5
    user-interface.properties: |
      color.good=purple
      color.bad=yellow
      allow.textmode=true    

```

[//]: # (![]&#40;https://github.com/googs1025/dbconfig-operator/blob/main/image/%E6%B5%81%E7%A8%8B%E5%9B%BE.jpg?raw=true&#41;)

### 项目功能
1. 自动在多个 namespace 创建 Secret ConfigMap 资源
2. 支持创建 更新 删除事件

### 项目部署与使用
1. 打成镜像或是使用编译二进制。
```bash
# 项目根目录执行
[root@VM-0-16-centos clusterconfigoperator]# pwd
/root/clusterconfigoperator
# 下列命令会得到一个二进制文件，服务启动时需要使用。
# 可以直接使用 docker 镜像部署
[root@VM-0-16-centos clusterconfigoperator]# docker build -t clusterconfigoperator:v1 .
Sending build context to Docker daemon  194.6kB
Step 1/15 : FROM golang:1.18.7-alpine3.15 as builder
 ---> 33c97f935029
Step 2/15 : WORKDIR /app
...
```
2. apply crd 资源
```bash
[root@VM-0-16-centos clusterconfigoperator]#
[root@VM-0-16-centos clusterconfigoperator]# kubectl apply -f yaml/clusterconfig.yaml
customresourcedefinition.apiextensions.k8s.io/clusterconfigs.api.practice.com unchanged
```
3. 启动 controller 服务(需要先执行 rbac.yaml，否则服务会报错)
```bash
[root@VM-0-16-centos clusterconfigoperator]# kubectl apply -f yaml/rbac.yaml
serviceaccount/myclusterconfig-sa unchanged
clusterrole.rbac.authorization.k8s.io/myclusterconfig-clusterrole unchanged
clusterrolebinding.rbac.authorization.k8s.io/myclusterconfig-ClusterRoleBinding unchanged
[root@VM-0-16-centos clusterconfigoperator]# kubectl apply -f yaml/deploy.yaml
deployment.apps/myclusterconfig-controller unchanged
```
4. 查看 operator 服务
```bash
[root@VM-0-16-centos clusterconfigoperator]# kubectl logs myclusterconfig-controller-6689489dbd-hp4vr
I1224 04:12:00.022968       1 init_k8s_config.go:15] run in the cluster
{"level":"info","ts":"2023-12-24T04:12:00Z","logger":"controller-runtime.metrics","msg":"Metrics server is starting to listen","addr":":8080"}
{"level":"info","ts":"2023-12-24T04:12:00Z","logger":"clusterconfig-operator","msg":"Starting server","path":"/metrics","kind":"metrics","addr":"[::]:8080"}
{"level":"info","ts":"2023-12-24T04:12:00Z","logger":"clusterconfig-operator","msg":"Starting EventSource","controller":"clusterconfig","controllerGroup":"api.practice.com","controllerKind":"ClusterConfig","source":"kind source: *v1alpha1.ClusterConfig"}
{"level":"info","ts":"2023-12-24T04:12:00Z","logger":"clusterconfig-operator","msg":"Starting Controller","controller":"clusterconfig","controllerGroup":"api.practice.com","controllerKind":"ClusterConfig"}
{"level":"info","ts":"2023-12-24T04:12:00Z","logger":"clusterconfig-operator","msg":"Starting workers","controller":"clusterconfig","controllerGroup":"api.practice.com","controllerKind":"ClusterConfig","worker count":1}
I1224 04:12:26.747409       1 controller.go:172] namespace list: [default mycsi test1 test2 test3]
I1224 04:12:26.747447       1 controller.go:179] namespace to create configmaps: default
I1224 04:12:26.851405       1 controller.go:191] [toConfigMap] Created in [default] namespace
I1224 04:12:26.851431       1 controller.go:179] namespace to create configmaps: mycsi
I1224 04:12:26.855867       1 controller.go:191] [toConfigMap] Created in [mycsi] namespace
I1224 04:12:26.855895       1 controller.go:179] namespace to create configmaps: test1
I1224 04:12:26.860142       1 controller.go:191] [toConfigMap] Created in [test1] namespace
```


### RoadMap
