FROM --platform=${BUILDPLATFORM} dpanel/builder:alpine AS builder

ARG TARGETARCH
ARG APP_VERSION
ARG APP_FAMILY

WORKDIR /src
COPY ./ /src

RUN --mount=type=cache,target=/root/.cache/go-build,id=dpanel_go_build_${TARGETARCH} \
    --mount=type=cache,target=/go/pkg/mod,id=dpanel_mod_cache_${TARGETARCH} \
    export HTTP_PROXY=${HTTP_PROXY} && \
    export HTTPS_PROXY=${HTTP_PROXY} && \
    COMMON_ARGS="VERSION=${APP_VERSION} FAMILY=${APP_FAMILY-ce} OS=linux" && \
    if [ "$TARGETARCH" = "amd64" ]; then \
        make build ${COMMON_ARGS} AMD64=1 AMD64_CC=gcc; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        make build ${COMMON_ARGS} ARM64=1 ARM64_CC=$TC_MUSL_ARM64; \
    elif [ "$TARGETARCH" = "arm" ]; then \
        make build ${COMMON_ARGS} ARM7=1 ARM7_CC=$TC_MUSL_ARMV7; \
    else \
        echo "Unsupported architecture: $TARGETARCH" && exit 1; \
    fi

FROM scratch AS binary-export

ARG TARGETARCH
ARG APP_FAMILY

COPY --from=builder /src/runtime /

FROM alpine AS base

ARG HTTP_PROXY

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories && \
    export HTTP_PROXY=${HTTP_PROXY} HTTPS_PROXY=${HTTP_PROXY} && apk add --no-cache --update \
    bash musl docker-cli docker-cli-compose docker-cli-buildx tzdata git openssh-client curl && \
    rm -f /usr/bin/docker-compose && \
    mkdir -p /root/.ssh && chmod 700 /root/.ssh

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

COPY --from=builder /src/runtime/dpanel${APP_FAMILY:+"-${APP_FAMILY}"}-musl-${TARGETARCH} /app/server/dpanel
COPY ./config.yaml /app/server/config.yaml
COPY ./docker/script /app/script
COPY ./docker/task /docker

RUN chmod -R 755 /docker/* /app/server/dpanel

WORKDIR /app/server
VOLUME [ "/dpanel" ]

FROM base AS lite
ENV APP_ENV=lite
EXPOSE 8080
ENTRYPOINT [ "/docker/entrypoint-lite.sh" ]

FROM base AS production
ENV APP_ENV=production

RUN export HTTP_PROXY=${HTTP_PROXY} HTTPS_PROXY=${HTTP_PROXY} && apk add --no-cache nginx openssl && \
    mkdir -p /tmp/nginx/body /var/lib/nginx/cache/public /var/lib/nginx/cache/private /var/www/challenges

COPY ./docker/nginx/nginx.conf /etc/nginx/nginx.conf
COPY ./docker/nginx/default.conf /etc/nginx/http.d/default.conf
COPY ./docker/nginx/include /etc/nginx/conf.d/include

RUN tar -zxvf /docker/acme.tar.gz -C /docker && cd /docker/acme.sh-master && ./acme.sh --install --config-home /dpanel/acme

EXPOSE 443 80 8080

ENTRYPOINT [ "/docker/entrypoint.sh" ]