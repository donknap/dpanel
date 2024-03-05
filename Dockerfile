FROM alpine:3.18

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories
RUN apk --no-cache add nginx

RUN mkdir -p /dpanel/nginx/default_host /dpanel/nginx/proxy_host \
     /dpanel/nginx/redirection_host /dpanel/nginx/dead_host \
     /dpanel/nginx/temp

ADD ./docker/nginx/include /dpanel/nginx/include
COPY ./docker/nginx/nginx.conf /etc/nginx/nginx.conf
COPY ./docker/entrypoint.sh /dpanel/entrypoint.sh

EXPOSE 80

ENTRYPOINT ["sh", "/dpanel/entrypoint.sh"]