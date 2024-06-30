# nntplexer

NNTP protocol multiplexer with auth, stats, multiple backends, etc.

## Proxying

### nginx

`nginx.conf`

```nginx
upstream nntplexer {
    least_conn;
    server 127.0.0.1:9999;
}

stream {
    server {
        listen 8888;
        proxy_pass nntplexer;
        proxy_protocol on;
    }
}
```

`nntplexer.ini`

```ini
[server]
proxy_protocol = on
```

![alt text](https://github.com/ucrawler/nntplexer/blob/[branch]/grafana%20dashboard.png)
