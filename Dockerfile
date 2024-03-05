FROM nginx

RUN mkdir -p /dpanel/nginx/default_host /dpanel/nginx/proxy_host \
     /dpanel/nginx/redirection_host /dpanel/nginx/dead_host \
     /dpanel/nginx/temp

ADD ./docker/nginx/include /dpanel/nginx/include
COPY ./docker/nginx/nginx.conf /etc/nginx/nginx.conf