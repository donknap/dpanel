<h1 align="center">
<img src="https://cdn.w7.cc/dpanel/logo.jpg" alt="DPanel" width="100" />
  <br> DPanel
</h1>
<h4 align="center"> Docker 可视化面板系统，提供完善的 docker 管理功能。 </h4>

<div align="center">

[![GitHub stars](https://img.shields.io/github/stars/donknap/dpanel.svg)](https://github.com/donknap/dpanel) &nbsp;
[![GitHub latest release](https://img.shields.io/github/v/release/donknap/dpanel)](https://github.com/donknap/dpanel/releases) &nbsp;
[![GitHub latest commit](https://img.shields.io/github/last-commit/donknap/dpanel.svg)](https://github.com/donknap/dpanel/commits/master/) &nbsp;
[![Build Status](https://github.com/donknap/dpanel/actions/workflows/release.yml/badge.svg)](https://github.com/donknap/dpanel/actions) &nbsp;
[![Docker Pulls](https://img.shields.io/docker/pulls/dpanel/dpanel)](https://hub.docker.com/r/dpanel/dpanel/tags) &nbsp;
<a href="https://hellogithub.com/repository/c69089b776704985b989f98626de977a" target="_blank"><img src="https://abroad.hellogithub.com/v1/widgets/recommend.svg?rid=c69089b776704985b989f98626de977a&claim_uid=ekhLfDOxR5U0mVw&theme=small" alt="Featured｜HelloGitHub" /></a>

[**官网**](https://dpanel.cc/) &nbsp; |
&nbsp; [**演示**](https://demo.dpanel.cc/) &nbsp; |
&nbsp; [**文档**](https://doc.dpanel.cc/#/zh-cn/install/docker) &nbsp; |
&nbsp; [**视频教程**](https://space.bilibili.com/346309066) &nbsp; |
&nbsp; [**交流群**](https://qm.qq.com/q/2v4x9x8q4k) &nbsp; |
&nbsp; [**赞助**](https://afdian.com/a/dpanel) &nbsp;

</div>

### 开始使用

> [!IMPORTANT]  
> macos 下需要先将 docker.sock 文件 link 到 /var/run/docker.sock 目录中 \
> ln -s -f /Users/用户/.docker/run/docker.sock  /var/run/docker.sock

> 国内镜像 \
> registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:latest \
> registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:lite

```
docker run -it -d --name dpanel --restart=always \
 -p 80:80 -p 443:443 -p 8807:8080 -e APP_NAME=dpanel \
 -v /var/run/docker.sock:/var/run/docker.sock -v dpanel:/dpanel \
 dpanel/dpanel:latest 
```

##### lite 版

lite 版去掉了域名转发相关，需要自行转发域名绑定容器，不需要绑定 80 及 443 端口

```
docker run -it -d --name dpanel --restart=always \
 -p 8807:8080 -e APP_NAME=dpanel \
 -v /var/run/docker.sock:/var/run/docker.sock -v dpanel:/dpanel \
 dpanel/dpanel:lite
```

#### 为爱发电

DPanel 是一个开源软件。

如果此项目对你所有帮助，并希望我继续下去，请考虑赞助我为爱发电！感谢所有的爱和支持。

https://afdian.com/a/dpanel

#### 赞助感谢

###### 服务器 & CDN

<a href="https://anycast.ai" target="_blank">
<img src="https://doc.dpanel.cc/storage/image/sponsor-server.png" width="200" />
</a>

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

#### Star History
[![Star History Chart](https://api.star-history.com/svg?repos=donknap/dpanel&type=Timeline)](https://star-history.com/#donknap/dpanel&Timeline)
