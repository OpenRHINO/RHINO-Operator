# RHINO-Operator

  Serverless模式在互联网领域和数字化应用中已经得到了生产级别的使用，但对于并行计算类应用支持目前仍然不足。大量并行计算类应用仍需要以传统方式进行开发、部署和运行，工作效率和硬件利用效率均有较大提升空间。
  为解决该问题，我们提出了RHINO方案（seRverless HIgh performaNce cOmputing），包括：平台、框架和开发工具三个部分，在一些实际应用中得到反馈后，我们将部分代码重构并逐步开源，形成OpenRHINO。

<img width="1300" alt="image" src="https://user-images.githubusercontent.com/20229719/216837309-4af11631-be1d-4cf3-8189-2e07a908af56.png">

## RhinoJob
### API访问示例
```
apiVersion: openrhino.org/v1alpha1
kind: RhinoJob
metadata:
  labels:
    app.kubernetes.io/name: rhinojob
    app.kubernetes.io/instance: rhinojob-sample
    app.kubernetes.io/part-of: rhino-operator
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/created-by: rhino-operator
  name: rhinojob-sample
spec:
  image: "zhuhe0321/libgrape:v1.0"
  ttl: 300
  parallelism: 4
  appExec: "./libgrape"
  appArgs: ["--vfile", "/data/p2p-31.v","--efile", "/data/p2p-31.e",
          "--application", "sssp", "--sssp_source", "6", 
          "--out_prefix", "/data/output_sssp", "--directed"]
  dataServer: "10.0.0.7"
  dataPath: "/kfnmck56"
  
```
### API字段定义
#### Spec
- image：使用RHINO-cli工具build的镜像
- ttl：用户预估的job执行时间(单位s)，默认时间600s，超过预估时间将终止job执行
- parallelism：并行度，MPI任务运行的总进程个数，默认值为1
- appExec：镜像中可执行文件的位置
- appArgs：MPI任务的运行参数，如果并行MPI job需要读写数据，需要使用/data目录
- dataServer: NFS服务器地址
- dataPath：NFS服务器挂载目录，RHINO-Operator会将该目录挂载到所有worker容器的/data路径下
#### Status
- pending：主控容器或任一工作容器未部署成功
- running：所有容器都处于运行状态
- failed：主控容器或任一工作容器运行时出现错误
- completed：主控容器和所有工作容器都执行完毕
