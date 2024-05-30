# DPanel

Docker 可视化面板系统，提供完善的 docker 管理功能。

# 运行

> macos 下需要先将 docker.sock 文件 link 到 /var/run/docker.sock 目录中 \
> ln -s -f /Users/用户/.docker/run/docker.sock  /var/run/docker.sock

##### 新建 DPanel 默认网络

```
docker network create dpanel-local
```

##### 启动容器

> 如果不需要设置容器的域名转发功能，可不暴露 80 端口
> 也可以暴露到其它端口，比如 8808:80。则通过 http://域名:8808 进行转发

```
docker run -it -d --name dpanel -p 80:80 -p 8807:8080 --network dpanel-local -v /var/run/docker.sock:/var/run/docker.sock donknap/dpanel:latest
```

### 默认帐号

admin / admin

# 使用手册

https://donknap.github.io/dpanel-docs

#### 交流群

<img src="https://github.com/donknap/dpanel-docs/blob/master/storage/image/qq.png?raw=true" width="300" />

#### 相关仓库

- 镜像构建基础模板 https://github.com/donknap/dpanel-base-image 
- DPanel镜像 https://github.com/donknap/dpanel-image
- 文档 https://github.com/donknap/dpanel-docs

#### 相关组件

- Rangine 开发框架 https://github.com/we7coreteam/w7-rangine-go-skeleton
- Docker Sdk https://github.com/docker/docker
- React & UmiJs
- Ant Design & Ant Design Pro & Ant Design Charts
