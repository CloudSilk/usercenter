## 项目名称
> 请介绍一下你的项目吧  



## 运行条件
> 列出运行该项目所必须的条件和相关依赖  
* 条件一
* 条件二
* 条件三



## 运行说明
> 说明如何运行和使用你的项目，建议给出具体的步骤说明
* 添加配置文件
```yaml
dubbo:
  config-center:
    protocol: nacos
    address: 127.0.0.1:8848
    data-id: "usercenter"
    group: basic
    namespace: nooocode
```

```yaml
dubbo:
  config-center:
    protocol: nacos
    address: 127.0.0.1:8848
    data-id: "usercenter"
    params:
      mysql: "root:123456@(127.0.0.1:3306)/usercenter?charset=utf8mb4&parseTime=True&loc=Local"
      debug: "true"
      token-key: "Lowcode"
      redis-addr: ""
      redis-user-name: ""
      redis-pwd: ""
      token-expired: 120
  application: # 应用配置
    name: usercenter
    module: local
    version: 1.0.0 
    owner: nooocode
    organization: nooocode
    metadata-type: local # 元数据上报方式，默认为本地
  metadata-report: # 元数据上报配置, 不包含此字段则不开启元数据上报，应用级服务发现依赖此字段，参考例子：https://github.com/apache/dubbo-go-samples/tree/master/registry/servicediscovery
    protocol: nacos # 元数据上报方式，支持nacos/zookeeper 
    address: 127.0.0.1:8848 
    username: ""
    password: ""
    timeout: "3s"
  registries:
    nacos:
      protocol: nacos
      timeout: 3s
      address: 127.0.0.1:8848
  protocols:
    triple:
      name: tri
      port: 20003
  provider:
    registry-ids: nacos
    services:
      # you may refer to `Reference()` method defined in `protobuf/triple/helloworld.pb.go`
      UserProvider:
        protocol-ids: triple
        # interface is for registry
        interface: org.nooocode.User
      TenantProvider:
        protocol-ids: triple
        # interface is for registry
        interface: org.nooocode.Tenant
      RoleProvider:
        protocol-ids: triple
        # interface is for registry
        interface: org.nooocode.Role
      MenuProvider:
        protocol-ids: triple
        # interface is for registry
        interface: org.nooocode.Menu
      APIProvider:
        protocol-ids: triple
        # interface is for registry
        interface: org.nooocode.API
      IdentityProvider:
        protocol-ids: triple
        interface: org.nooocode.Identity
```
* 操作二
* 操作三  



## 测试说明
> 如果有测试相关内容需要说明，请填写在这里  



## 技术架构
> 使用的技术框架或系统架构图等相关说明，请填写在这里  


## 协作者
> 高效的协作会激发无尽的创造力，将他们的名字记录在这里吧
