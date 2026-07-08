# Created by DPanel. DO NOT EDIT OR DELETE!!!

{{if and .enableSSL (eq (print .serverPort) "443")}}
server {
    listen 80;
    listen [::]:80;
    server_name {{.serverName}} {{range $index, $value := .serverNameAlias}}{{$value}} {{end}};

    return 301 https://$host$request_uri;
}
{{end}}

server {
    set $forward_scheme {{.serverProtocol}};

    {{if .enableSSL}}
    listen {{.serverPort}} ssl;
    listen [::]:{{.serverPort}} ssl;
    # http2 on;

    error_page 497 =307 https://$host:$server_port$request_uri;

    ssl_certificate {{.sslCrt}};
    ssl_certificate_key {{.sslKey}};
    ssl_session_cache shared:SSL:1m;
    ssl_session_timeout 5m;
    ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE:ECDH:AES:HIGH:!NULL:!aNULL:!MD5:!ADH:!RC4;
    ssl_protocols TLSv1.1 TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;
    add_header Strict-Transport-Security "max-age=63072000; preload" always;
    {{else}}
    listen {{.serverPort}};
    listen [::]:{{.serverPort}};
    {{end}}

    server_name {{.serverName}} {{range $index, $value := .serverNameAlias}}{{$value}} {{end}};

    include /etc/nginx/conf.d/include/resolver.conf;

    {{if .enableAssetCache}}
    # Asset Caching
    include /etc/nginx/conf.d/include/assets.conf;
    {{end}}

    {{if .enableBlockCommonExploits}}
    # Block Exploits
    include /etc/nginx/conf.d/include/block-exploits.conf;
    {{end}}

    {{if .extraNginx}}
    # Extra Nginx Configuration
    include /dpanel/nginx/extra_host/{{.serverName}}.conf;
    {{end}}

    {{if eq .type "proxy"}}
    location / {
        {{if .enableWs}}
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_http_version 1.1;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        {{end}}

        add_header X-Served-By $host;

        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Host   $host;
        proxy_set_header X-Forwarded-Scheme $scheme;
        proxy_set_header X-Forwarded-Proto  $scheme;
        proxy_set_header X-Forwarded-For    $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP          $remote_addr;

        client_max_body_size 0;
        proxy_buffering off;

        {{if eq .serverAddress "host.dpanel.local"}}
        proxy_pass $forward_scheme://host.dpanel.local:{{.port}};
        {{else}}
        set $upstream_endpoint {{.serverAddress}}:{{.port}};
        proxy_pass $forward_scheme://$upstream_endpoint;
        {{end}}
    }
    {{end}}

    {{if eq .type "redirect"}}
    location / {
        return 301 $forward_scheme://{{.serverAddress}}:{{.port}}$request_uri;
    }
    {{end}}

    {{if eq .type "fpm"}}
    root {{.wwwRoot}};
    index index.php index.html index.htm;
    location ~ \.php$ {
        try_files $uri =404;
        {{if eq .serverAddress "host.dpanel.local"}}
        fastcgi_pass host.dpanel.local:{{.port}};
        {{else}}
        set $upstream_endpoint {{.serverAddress}}:{{.port}};
        fastcgi_pass $upstream_endpoint;
        {{end}}
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME {{.fpmRoot}}$fastcgi_script_name;
        include fastcgi_params;

        fastcgi_intercept_errors off;
    }
    {{end}}
}