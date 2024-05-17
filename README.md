# DPanel

Docker 可视化面板系统。

# 使用

> macos 下需要先将 docker.sock 文件 link 到 /var/run/docker.sock 目录中 \
> ln -s -f /Users/用户/.docker/run/docker.sock  /var/run/docker.sock

```
docker run -it --name dpanel -p 8807:80 -v /var/run/docker.sock:/var/run/docker.sock ccr.ccs.tencentyun.com/donknap/dpanel:v1.0.0
```
