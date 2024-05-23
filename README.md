# DPanel

Docker 可视化面板系统，提供完善的 docker 管理功能。

# 运行

> macos 下需要先将 docker.sock 文件 link 到 /var/run/docker.sock 目录中 \
> ln -s -f /Users/用户/.docker/run/docker.sock  /var/run/docker.sock

```
docker run -it --name dpanel -p 8807:80 -v /var/run/docker.sock:/var/run/docker.sock ccr.ccs.tencentyun.com/donknap/dpanel:latest
```

### 默认帐号

admin / admin

# 使用手册

https://donknap.github.io/dpanel-docs

### 相关仓库

- 镜像构建基础模板 https://github.com/donknap/dpanel-base-image 
- DPanel镜像 https://github.com/donknap/dpanel-image
