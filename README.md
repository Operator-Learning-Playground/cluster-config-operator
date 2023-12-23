## cluster-config-operator 简易型集群配置控制器

### 项目思路与设计
设计背景：

思路：

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
1. 
2. 

### 项目部署与使用
1. 打成镜像或是使用编译二进制。

2. apply crd 资源
  
3. 启动 controller 服务(需要先执行 rbac.yaml，否则服务会报错)
   
4. 查看 operator 服务



### RoadMap
