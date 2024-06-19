map $scheme $hsts_header {
    https   "max-age=63072000; preload";
}

upstream {{.TargetName}} {
  server {{.ServerAddress}}:{{.Port}};
}

{{if .EnableSSL}}
server {
    listen 80;
    server_name {{.ServerName}};
    rewrite ^(.*) https://$server_name$1 permanent;
}
{{end}}

server {
  set $forward_scheme http;
  {{if .EnableSSL}}
  listen 443 ssl;
  {{else}}
  listen 80;
  {{end}}

  server_name {{.ServerName}};

  {{if .EnableSSL}}
  ssl_certificate /dpanel/nginx/cert/{{.ServerName}}.crt;
  ssl_certificate_key /dpanel/nginx/cert/{{.ServerName}}.key;
  ssl_session_cache shared:SSL:1m;
  ssl_session_timeout 5m;
  ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE:ECDH:AES:HIGH:!NULL:!aNULL:!MD5:!ADH:!RC4;
  ssl_protocols TLSv1.1 TLSv1.2 TLSv1.3;
  ssl_prefer_server_ciphers on;
  {{end}}

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

 {{.ExtraNginx}}

  location / {
    {{if .EnableWs}}
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $http_connection;
    proxy_http_version 1.1;
    {{end}}

    add_header       X-Served-By $host;

    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Scheme $scheme;
    proxy_set_header X-Forwarded-Proto  $scheme;
    proxy_set_header X-Forwarded-For    $proxy_add_x_forwarded_for;
    proxy_set_header X-Real-IP          $remote_addr;
    proxy_pass       $forward_scheme://{{.TargetName}}$request_uri;
  }

}