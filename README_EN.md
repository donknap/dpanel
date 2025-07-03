<h1 align="center">
<img src="https://cdn.w7.cc/dpanel/dpanel-logo.png" alt="DPanel" width="500" />
</h1>
<h4 align="center"> DPanel one of the most lightweight panel for docker. </h4>

<div align="center">

[![GitHub stars](https://img.shields.io/github/stars/donknap/dpanel.svg)](https://github.com/donknap/dpanel) &nbsp;
[![GitHub latest release](https://img.shields.io/github/v/release/donknap/dpanel)](https://github.com/donknap/dpanel/releases) &nbsp;
[![GitHub latest commit](https://img.shields.io/github/last-commit/donknap/dpanel.svg)](https://github.com/donknap/dpanel/commits/master/) &nbsp;
[![Build Status](https://github.com/donknap/dpanel/actions/workflows/release.yml/badge.svg)](https://github.com/donknap/dpanel/actions) &nbsp;
[![Docker Pulls](https://img.shields.io/docker/pulls/dpanel/dpanel)](https://hub.docker.com/r/dpanel/dpanel/tags) &nbsp;
<a href="https://hellogithub.com/repository/c69089b776704985b989f98626de977a" target="_blank"><img src="https://abroad.hellogithub.com/v1/widgets/recommend.svg?rid=c69089b776704985b989f98626de977a&claim_uid=ekhLfDOxR5U0mVw&theme=small" alt="Featured｜HelloGitHub" /></a>

[**Home**](https://dpanel.cc/) &nbsp; |
&nbsp; [**Demo**](https://dpanel.park1991.com/) &nbsp; |
&nbsp; [**Docs**](https://dpanel.cc/#/en-us/README) &nbsp; |
&nbsp; [**Pro Edition**](https://dpanel.cc/#/zh-cn/manual/pro) &nbsp; |
&nbsp; [**Sponsor**](https://afdian.com/a/dpanel) &nbsp;

</div>

### Pro Edition

The Pro Edition is merely an enhancement and supplement to the Community Edition, serving to intensify, upgrade, or provide highly personalized features for certain functionalities in the Community Edition.

Thank you for your support and love. 

🚀🚀🚀 [More Feature](https://dpanel.cc/#/zh-cn/manual/pro?id=%e4%bb%b7%e6%a0%bc%e5%8f%8a%e5%8a%9f%e8%83%bd) 🚀🚀🚀

#### Standard Version

```
docker run -it -d --name dpanel --restart=always \
 -p 80:80 -p 443:443 -p 8807:8080 -e APP_NAME=dpanel \
 -v /var/run/docker.sock:/var/run/docker.sock -v /home/dpanel:/dpanel \
 dpanel/dpanel:latest 
```

#### Lite Version

The lite version removes nginx proxy features, no need to bind ports 80 and 443.

```
docker run -it -d --name dpanel --restart=always \
 -p 8807:8080 -e APP_NAME=dpanel \
 -v /var/run/docker.sock:/var/run/docker.sock -v /home/dpanel:/dpanel \
 dpanel/dpanel:lite
```

#### Install By Script 

> Tested on Debian and Alpine.

```
curl -sSL https://dpanel.cc/quick.sh -o quick.sh && sudo bash quick.sh
```

#### Telegram 

https://t.me/dpanel666

<img src="https://github.com/donknap/dpanel-docs/blob/master/storage/image/telegram.png?raw=true" width="300" />

#### Buy me coffee

If this project has helped you and you want me to keep going, please sponsor me and buy me a coffee! Thanks for all the love and support.

[https://afdian.com/a/dpanel](https://afdian.com/a/dpanel)

#### Thanks Contributors

[![Contributors](https://contrib.rocks/image?repo=donknap/dpanel)](https://github.com/donknap/dpanel/graphs/contributors)

#### Preview

###### overview
![home.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/home-en.png)
###### container
![app-list.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/app-list-en.png)
###### file explorer in container
![app-file.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/app-file-en.png)
###### image
![image-list.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/image-list-en.png)
###### build image
![image-create.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/image-create-en.png)
###### create compose task
![compose-create.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/compose-create-en.png)
###### deploy compose task
![compose-deploy.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/compose-deploy-en.png)
###### system
![system-basic.png](https://raw.githubusercontent.com/donknap/dpanel-docs/master/storage/image/system-basic-en.png)

#### Star History
[![Star History Chart](https://api.star-history.com/svg?repos=donknap/dpanel&type=Timeline)](https://star-history.com/#donknap/dpanel&Timeline)
