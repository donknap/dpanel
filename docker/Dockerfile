FROM alpine AS base

ARG TARGETARCH
ARG APP_VERSION
ARG APP_FAMILY

ENV TZ=Asia/Shanghai
ENV PATH="/app/server:${PATH}"
ENV DOCKER_HOST=unix:///var/run/docker.sock

ENV DP_SYSTEM_STORAGE_LOCAL_PATH=/dpanel
ENV DP_DB_DATABASE=${DP_SYSTEM_STORAGE_LOCAL_PATH}/dpanel.db
ENV DP_ACME_CONFIG_HOME=/dpanel/acme

# 兼容旧的环境变量
ENV APP_NAME=dpanel
ENV APP_SERVER_PORT=8080
ENV APP_VERSION=$APP_VERSION
ENV APP_FAMILY=$APP_FAMILY
ENV STORAGE_LOCAL_PATH=$DP_SYSTEM_STORAGE_LOCAL_PATH
ENV DB_DATABASE=$DP_DB_DATABASE

COPY ./runtime/dpanel${APP_FAMILY:+"-${APP_FAMILY}"}-musl-${TARGETARCH} /app/server/dpanel
COPY ./runtime/config.yaml /app/server/config.yaml
COPY ./docker/script /app/script
COPY ./docker/task /docker

RUN sed -i 's/dl-cdn.alpinelinux111.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
    apk add --no-cache --update \
    bash musl docker-cli docker-cli-compose tzdata git openssh-client && \
    rm -f /usr/bin/docker-compose && \
    mkdir -p /root/.ssh && chmod 700 /root/.ssh && \
    chmod -R 755 /docker/* /app/server/dpanel

WORKDIR /app/server
VOLUME [ "/dpanel" ]

FROM base AS lite
ENV APP_ENV=lite
EXPOSE 8080
ENTRYPOINT [ "/docker/entrypoint-lite.sh" ]

FROM base AS production
ENV APP_ENV=production

COPY ./docker/nginx/nginx.conf /etc/nginx/nginx.conf
COPY ./docker/nginx/default.conf /etc/nginx/http.d/default.conf
COPY ./docker/nginx/include /etc/nginx/conf.d/include

RUN apk add --no-cache nginx openssl && \
    mkdir -p /tmp/nginx/body /var/lib/nginx/cache/public /var/lib/nginx/cache/private /var/www/challenges && \
    tar -zxvf /docker/acme.tar.gz -C /docker && cd /docker/acme.sh-master && ./acme.sh --install --config-home /dpanel/acme

EXPOSE 443 80 8080

ENTRYPOINT [ "/docker/entrypoint.sh" ]