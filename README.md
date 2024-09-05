# DPanel

Docker 可视化面板系统，提供完善的 docker 管理功能。

##### 启动容器

> [!IMPORTANT]  
> macos 下需要先将 docker.sock 文件 link 到 /var/run/docker.sock 目录中 \
> ln -s -f /Users/用户/.docker/run/docker.sock  /var/run/docker.sock

> 国内镜像 registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:latest

```
docker run -it -d --name dpanel --restart=always \
 -p 80:80 -p 443:443 -p 8807:8080 -e APP_NAME=dpanel \
 -v /var/run/docker.sock:/var/run/docker.sock -v dpanel:/dpanel \
 -e INSTALL_USERNAME=admin -e INSTALL_PASSWORD=admin \
 dpanel/dpanel:latest 
```

##### lite 版

lite 版去掉了域名转发相关，需要自行转发域名绑定容器，不需要绑定 80 及 443 端口

```
docker run -it -d --name dpanel --restart=always \
 -p 8807:8080 -e APP_NAME=dpanel \
 -v /var/run/docker.sock:/var/run/docker.sock -v dpanel:/dpanel \
 -e INSTALL_USERNAME=admin -e INSTALL_PASSWORD=admin \
 dpanel/dpanel:lite
```

### 默认帐号

admin / admin

# 使用手册

https://donknap.github.io/dpanel-docs

#### 赞助

DPanel 是一个开源软件。

如果此项目对你所有帮助，并希望我继续下去，请考虑赞助我为爱发电！感谢所有的爱和支持。

https://afdian.com/a/dpanel

#### 交流群

QQ: 837583876

<img src="https://github.com/donknap/dpanel-docs/blob/master/storage/image/qq.png?raw=true" width="300" />

#### 界面预览

###### 概览
![home.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/home.png)
###### 容器管理
![app-list.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/app-list.png)
###### 文件管理
![app-file.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/app-file.png)
###### 镜像管理
![image-list.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/image-list.png)
###### 创建镜像
![image-create.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/image-create.png)
###### 创建Compose
![compose-create.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/compose-create.png)
###### 部署Compose
![compose-deploy.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/compose-deploy.png)
###### 系统管理
![system-basic.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/system-basic.png)

#### 相关仓库

- 镜像构建基础模板 https://github.com/donknap/dpanel-base-image
- 文档 https://github.com/donknap/dpanel-docs

#### 相关组件

- Rangine 开发框架 https://github.com/we7coreteam/w7-rangine-go-skeleton
- Docker Sdk https://github.com/docker/docker
- React & UmiJs
- Ant Design & Ant Design Pro & Ant Design Charts
