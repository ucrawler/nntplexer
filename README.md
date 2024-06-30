# nntplexer

Start your own independant tier 2 usenet business without the need for any storage.

Inspired by https://github.com/ovpn-to/oVPN.to-Advanced-NNTP-Proxy

NNTP protocol multiplexer with auth, stats, multiple backends, etc.

1 gbit per vm core, backends are tried in sequential order by priority in case of missing article.

1 or more hetzner 4 core vm's + 1 or more accounts from https://whatsmyuse.net/ is what you need.

Destroy and re-create VM when reaching 20 TB to reset bandwidth :-)

I suggest 1 external and 1 internal to start out with to 'hide' the IP fetching the article.

Only external counts for bandwidth.

Wil you be able to reach https://www.fdcservers.net/configurator?fixedFilter=15&fixedFilterType=bandwidth_option ?

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

![alt text](https://raw.githubusercontent.com/ucrawler/nntplexer/main/grafana%20dashboard.png)
![alt text](https://raw.githubusercontent.com/ucrawler/nntplexer/main/backends%20table.png)
