# nntplexer

Start your own independant tier 2 usenet business without the need for any storage 

or

simply share your account with friends and family using your home computer

Inspired by https://github.com/ovpn-to/oVPN.to-Advanced-NNTP-Proxy

NNTP protocol multiplexer with auth, stats, multiple backends, etc.

1 binary to deploy which does multicore.

1 gbit throughput per vm core, backends are tried in sequential order by priority in case of missing article.

1 or more hetzner 4 core vm's for internal + 

1 upto 4 hetzner 1 core vm's for external (RRDNS) +

1 or more accounts from https://whatsmyuse.net/ is what you need.

Clients are load balanced using ip hash, hit the same internal always to protect your accounts :-)

Destroy and re-create VM when reaching 20 TB to reset bandwidth :-)

I suggest 1 external and 1 internal to start out with to 'hide' the IP fetching the article. :-)

Only external counts for bandwidth.

Wil you be able to reach https://www.fdcservers.net/configurator?fixedFilter=15&fixedFilterType=bandwidth_option ?

todo

cache articles on mongodb using ttl

skip backends using article postdate and backend retention

skip backend when poster name doesn't match :-)

## Proxying

### nginx

`nginx.conf`

```nginx
upstream nntplexer {
    ip_hash;
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
