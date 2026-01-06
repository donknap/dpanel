FROM alpine

ARG APP_VERSION
ARG TARGETARCH
ARG APP_FAMILY

ENV APP_NAME=dpanel
ENV APP_ENV=production
ENV APP_FAMILY=$APP_FAMILY
ENV APP_VERSION=$APP_VERSION
ENV APP_SERVER_PORT=8080

ENV DOCKER_HOST=unix:///var/run/docker.sock
ENV STORAGE_LOCAL_PATH=/dpanel
ENV DB_DATABASE=${STORAGE_LOCAL_PATH}/dpanel.db
ENV TZ=Asia/Shanghai
ENV DP_ACME_CONFIG_HOME=/dpanel/acme
ENV PATH="/app/server:${PATH}"

COPY ./docker/nginx/nginx.conf /etc/nginx/nginx.conf
COPY ./docker/nginx/default.conf /etc/nginx/http.d/default.conf
COPY ./docker/nginx/include /etc/nginx/conf.d/include
COPY ./docker/script /app/script
COPY ./docker/task /docker

COPY ./runtime/dpanel${APP_FAMILY:+"-${APP_FAMILY}"}-musl-${TARGETARCH} /app/server/dpanel
COPY ./runtime/config.yaml /app/server/config.yaml

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
  apk add --no-cache --update nginx musl docker-cli docker-cli-compose curl openssl tzdata git openssh-client && \
  mkdir -p /tmp/nginx/body /var/lib/nginx/cache/public /var/lib/nginx/cache/private && \
  rm /usr/bin/docker-compose && chmod 755 /docker/* && mkdir -p /root/.ssh && chmod 700 /root/.ssh && mkdir -p /var/www/challenges && \
  cd /docker && ./acme.sh --install --config-home /dpanel/acme

WORKDIR /app/server
VOLUME [ "/dpanel" ]

EXPOSE 443
EXPOSE 80
EXPOSE 8080

ENTRYPOINT [ "/docker/entrypoint.sh" ]