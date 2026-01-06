ARG IMAGE=dpanel/dpanel:lite
FROM ${IMAGE}

ARG APP_FAMILY

ENV APP_ENV=production
ENV APP_SERVER_PORT=8080

COPY ./docker/nginx/nginx.conf /etc/nginx/nginx.conf
COPY ./docker/nginx/default.conf /etc/nginx/http.d/default.conf
COPY ./docker/nginx/include /etc/nginx/conf.d/include
COPY ./docker/task /docker

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
  apk add --no-cache --update nginx curl openssl && \
  mkdir -p /tmp/nginx/body /var/lib/nginx/cache/public /var/lib/nginx/cache/private && \
  sh -s -- /docker/acme.sh --install-online --config-home /dpanel/acme && \
  chmod 755 /docker/* && mkdir -p /var/www/challenges

EXPOSE 443
EXPOSE 80

ENTRYPOINT [ "/docker/entrypoint.sh" ]