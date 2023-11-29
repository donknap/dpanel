FROM {{ImageBase}}

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories
RUN apk add --no-cache --update nginx php php-fpm composer \
 php-pdo php-pdo_mysql php-mysqli php-pdo_sqlite \
 php-curl \
 php-gd \
 php-gettext \
 php-bcmath \
 php-iconv \
 php-ctype \
 php-dom \
 php-fileinfo \
 php-json \
 php-mbstring \
 php-openssl \
 php-tokenizer \
 php81-xml php-pecl-imagick \
 php-simplexml \
 php-session \
 php-ftp \
 php-xmlreader \
 php-xmlwriter \
 php-zip

ADD ./nginx/default.conf /etc/nginx/http.d/
ADD ./nginx/php-fpm.conf /etc/php81/php-fpm.d/www.conf.99.conf
ADD ./entrypoint.sh /docker/entrypoint.sh

WORKDIR /home/site
RUN chown -R nginx:nginx /home/site

{{Extra1}}

ENTRYPOINT [ "sh", "/docker/entrypoint.sh" ]