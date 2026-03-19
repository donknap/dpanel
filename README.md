<h1 align="center">
<img src="https://cdn.w7.cc/dpanel/dpanel-logo.png" alt="DPanel" width="500" />
</h1>
<h4 align="center"> 轻量化容器管理面板；优雅的管理 Docker、Podman 容器。 </h4>

<div align="center">

[![GitHub stars](https://img.shields.io/github/stars/donknap/dpanel.svg)](https://github.com/donknap/dpanel) &nbsp;
[![GitHub latest release](https://img.shields.io/github/v/release/donknap/dpanel)](https://github.com/donknap/dpanel/releases) &nbsp;
[![GitHub latest commit](https://img.shields.io/github/last-commit/donknap/dpanel.svg)](https://github.com/donknap/dpanel/commits/master/) &nbsp;
[![Build Status](https://github.com/donknap/dpanel/actions/workflows/release.yml/badge.svg)](https://github.com/donknap/dpanel/actions) &nbsp;
[![Docker Pulls](https://img.shields.io/docker/pulls/dpanel/dpanel)](https://hub.docker.com/r/dpanel/dpanel/tags) &nbsp;
<a href="https://hellogithub.com/repository/c69089b776704985b989f98626de977a" target="_blank"><img src="https://abroad.hellogithub.com/v1/widgets/recommend.svg?rid=c69089b776704985b989f98626de977a&claim_uid=ekhLfDOxR5U0mVw&theme=small" alt="Featured｜HelloGitHub" /></a>


<p align="center">
  <a href="/README.md"><img alt="中文(简体)" src="https://img.shields.io/badge/中文(简体)-1677ff?style=for-the-badge"></a>
  <a href="/docs/README_EN.md"><img alt="English" src="https://img.shields.io/badge/English-1677ff?style=for-the-badge"></a>
</p>

------------------------------

[**官网**](https://dpanel.cc/) &nbsp; |
&nbsp; [**演示**](https://demo.dpanel.cc) &nbsp; |
&nbsp; [**文档**](https://dpanel.cc/#/zh-cn/install/docker) &nbsp; |
&nbsp; [**Pro版**](https://dpanel.cc/#/zh-cn/manual/pro) &nbsp; |
&nbsp; [**交流群**](https://qm.qq.com/q/2v4x9x8q4k) &nbsp; |
&nbsp; [**赞助**](https://afdian.com/a/dpanel) &nbsp;

</div>

### 专业版（PRO）

专业版仅是社区版的一个增强和补充，对于通用的、广泛的功能需求不会收录到专业版中。
针对社区版中的部分功能进行强化、升级或是一些极其个性化的需求功能。

感谢大家的支持与厚爱，希望 DPanel 可以小小的为 Docker 中文圈带来一些惊喜。

🚀🚀🚀 [功能介绍及对比](https://dpanel.cc/pro) 🚀🚀🚀


### 开始使用

> [!IMPORTANT]  
> MacOS 下通过 **/Users/用户/.docker/run/docker.sock:/var/run/docker.sock** 挂载 sock 文件

#### 标准版

```
docker run -d --name dpanel --restart=always \
 -p 80:80 -p 443:443 -p 8807:8080 -e APP_NAME=dpanel \
 -v /var/run/docker.sock:/var/run/docker.sock \
 -v /home/dpanel:/dpanel dpanel/dpanel:latest
```

#### 精简版 Lite

lite 版去掉了域名转发相关，需要自行转发域名绑定容器，不需要绑定 80 及 443 端口

```
docker run -d --name dpanel --restart=always \
 -p 8807:8080 -e APP_NAME=dpanel \
 -v /var/run/docker.sock:/var/run/docker.sock 
 -v /home/dpanel:/dpanel dpanel/dpanel:lite
```

#### 集成脚本

> 支持 Debian Ubuntu Alpine，其它发行版未进行测试，请提交 Issue

```
curl -sSL https://dpanel.cc/quick.sh -o quick.sh && sudo bash quick.sh
```

#### 镜像说明

| 版本  |  构建系统  | 地址                                                                                                    |       说明       |
|:---:|:------:|:------------------------------------------------------------------------------------------------------|:--------------:|
| 标准版 | Alpine | dpanel/dpanel:latest <br/> registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:latest                     |  包含域名转及证书申请功能  |
|  ^  | Alpine | dpanel/dpanel-pe:latest <br/> registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel-pe:latest               |      专业版       |
|  ^  | Debian | dpanel/dpanel:latest-debian <br/> registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:latest-debian       | 基于 Debian 系统构建 |
|  ^  | Debian | dpanel/dpanel-pe:latest-debian <br/> registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel-pe:latest-debian |      专业版       |
| 精简版 | Alpine | dpanel/dpanel:lite <br/> registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:lite                         |   只包含容器管理功能    |
|  ^  | Alpine | dpanel/dpanel-pe:lite <br/> registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel-pe:lite                   |      专业版       |
|  ^  | Debian | dpanel/dpanel:lite-debian <br/> registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:lite-debian           | 基于 Debian 系统构建 |
|  ^  | Debian | dpanel/dpanel-pe:lite-debian <br/> registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel-pe:lite-debian     |      专业版       |


#### 为爱发电

如果此项目对你所有帮助，并希望我继续下去，请考虑赞助我为爱发电！感谢所有的爱和支持。

##### 爱发电平台
[https://afdian.com/a/dpanel](https://afdian.com/a/dpanel)

##### 微信打赏

<img src="https://github.com/donknap/dpanel-docs/blob/vitepress/storage/image/wx-sponsor.jpg?raw=true" width="300" />

#### 交流群

QQ: 837583876

<img src="https://github.com/donknap/dpanel-docs/blob/master/storage/image/qq.png?raw=true" width="300" />

#### 赞助 
- ##### 莱卡云-专业云计算服务器提供商
    <a href="https://www.lcayun.com/actcloud.html?from=dpanel" target="_blank"><img width="200" src="https://www.lcayun.com/upload/banner/2023-10/11/169700642979539.png" /></a>
- ##### Developed using JetBrains IDEs.
    [![JetBrains logo.](https://resources.jetbrains.com/storage/products/company/brand/logos/jetbrains.svg)](https://jb.gg/OpenSource)
  
#### 感谢贡献人员

[![Contributors](https://contrib.rocks/image?repo=donknap/dpanel)](https://github.com/donknap/dpanel/graphs/contributors)

#### 界面预览

###### pro 自定义皮肤

![pro-1](https://cdn.w7.cc/dpanel/pro-1.png)

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
