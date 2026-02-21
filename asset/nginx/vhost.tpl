# Created by DPanel. DO NOT EDIT OR DELETE!!!

{{if and .EnableSSL (eq (print .ServerPort) "443")}}
server {
    listen 80;
    listen [::]:80;
    server_name {{.ServerName}} {{range $index, $value := .ServerNameAlias}}{{$value}} {{end}};

    return 301 https://$host$request_uri;
}
{{end}}

server {
    set $forward_scheme {{.ServerProtocol}};

    {{if .EnableSSL}}
    listen {{.ServerPort}} ssl;
    listen [::]:{{.ServerPort}} ssl;
    # http2 on;

    error_page 497 =307 https://$host:$server_port$request_uri;

    ssl_certificate {{.SslCrt}};
    ssl_certificate_key {{.SslKey}};
    ssl_session_cache shared:SSL:1m;
    ssl_session_timeout 5m;
    ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE:ECDH:AES:HIGH:!NULL:!aNULL:!MD5:!ADH:!RC4;
    ssl_protocols TLSv1.1 TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;
    add_header Strict-Transport-Security "max-age=63072000; preload" always;
    {{else}}
    listen {{.ServerPort}};
    listen [::]:{{.ServerPort}};
    {{end}}

    server_name {{.ServerName}} {{range $index, $value := .ServerNameAlias}}{{$value}} {{end}};

    include /etc/nginx/conf.d/include/resolver.conf;

    {{if .EnableAssetCache}}
    # Asset Caching
    include /etc/nginx/conf.d/include/assets.conf;
    {{end}}

    {{if .EnableBlockCommonExploits}}
    # Block Exploits
    include /etc/nginx/conf.d/include/block-exploits.conf;
    {{end}}

    {{if .ExtraNginx}}
    # Extra Nginx Configuration
    include /dpanel/nginx/extra_host/{{.ServerName}}.conf;
    {{end}}

    {{if eq .Type "proxy"}}
    location / {
        {{if .EnableWs}}
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_http_version 1.1;
        {{end}}

        add_header X-Served-By $host;

        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Scheme $scheme;
        proxy_set_header X-Forwarded-Proto  $scheme;
        proxy_set_header X-Forwarded-For    $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP          $remote_addr;

        {{if eq .ServerAddress "host.dpanel.local"}}
        proxy_pass $forward_scheme://host.dpanel.local:{{.Port}}$request_uri;
        {{else}}
        set $upstream_endpoint {{.ServerAddress}}:{{.Port}};
        proxy_pass $forward_scheme://$upstream_endpoint$request_uri;
        {{end}}
    }
    {{end}}

    {{if eq .Type "redirect"}}
    location / {
        return 301 $forward_scheme://{{.ServerAddress}}:{{.Port}}$request_uri;
    }
    {{end}}

    {{if eq .Type "fpm"}}
    root {{.WWWRoot}};
    index index.php index.html index.htm;
    location ~ \.php$ {
        try_files $uri =404;
        {{if eq .ServerAddress "host.dpanel.local"}}
        fastcgi_pass host.dpanel.local:{{.Port}};
        {{else}}
        set $upstream_endpoint {{.ServerAddress}}:{{.Port}};
        fastcgi_pass $upstream_endpoint;
        {{end}}
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME {{.FPMRoot}}$fastcgi_script_name;
        include fastcgi_params;

        fastcgi_intercept_errors off;
    }
    {{end}}
}