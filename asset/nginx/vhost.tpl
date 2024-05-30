map $scheme $hsts_header {
    https   "max-age=63072000; preload";
}

upstream target {
  server {{.ServerAddress}}:{{.Port}};
}

server {
  set $forward_scheme http;
  listen 80;
  listen [::]:80;
  server_name {{.ServerName}};

  {{if .EnableAssetCache}}
  # Asset Caching
  include /dpanel/nginx/include/assets.conf;
  {{end}}
  {{if .EnableBlockCommonExploits}}
  # Block Exploits
  include /dpanel/nginx/include/block-exploits.conf;
  {{end}}

  {{if .EnableWs}}
  proxy_set_header Upgrade $http_upgrade;
  proxy_set_header Connection $http_connection;
  proxy_http_version 1.1;
  {{end}}

  location / {
    {{if .EnableWs}}
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $http_connection;
    proxy_http_version 1.1;
    add_header       X-Served-By $host;
    {{end}}

    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Scheme $scheme;
    proxy_set_header X-Forwarded-Proto  $scheme;
    proxy_set_header X-Forwarded-For    $proxy_add_x_forwarded_for;
    proxy_set_header X-Real-IP          $remote_addr;
    proxy_pass       $forward_scheme://target$request_uri;
  }

  {{.ExtraNginx}}
}