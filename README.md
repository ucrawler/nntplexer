# nntplexer

Start your own independant tier 2 usenet business without the need for any storage 

or

simply share your account with friends and family using your home computer

Can be compiled for any OS.

Inspired by https://github.com/ovpn-to/oVPN.to-Advanced-NNTP-Proxy

NNTP protocol multiplexer with auth, stats, multiple backends, etc.

1 binary to deploy which does multicore.

1 gbit throughput per vm core, backends are tried in sequential order by priority in case of missing article.

1 or more hetzner 2 core vm's for internal + 

1 upto 4 hetzner 1 core vm's for external (RRDNS) +

1 or more accounts from https://whatsmyuse.net/ is what you need.

Clients are load balanced using ip hash, hit the same internal always to protect your accounts :-)

Destroy and re-create VM when reaching 20 TB to reset bandwidth

I suggest 1 external and 1 internal to start out with to 'hide' the IP fetching the article. :-)

Only external counts for bandwidth.

1 ngninja account allows you to split 50 connections between upto 50 accounts at the same time to access usenet with over 2 gbit/s speed, shorten the distance as much as possible between your proxied account and the internal.

A vm in the south east of USA will give you the best speeds possible..

Just imagine the flow in your head, total latency between all hops == speed. Usenet -> internal -> client and client -> internal -> Usenet is travelled every message ID. 2 gbit/sec = 285 roundtrips per sec.

Will you be able to reach https://www.fdcservers.net/configurator?fixedFilter=15&fixedFilterType=bandwidth_option ?

todo

cache articles on mongodb using ttl

skip backends using article postdate and backend retention

skip backend when poster name doesn't match :-)

## Proxying

### nginx (external)

generate ssl with letsencrypt, switch to positivessl to support the remaining 1%

`nginx.conf`

```nginx
upstream nntplexer {
    hash $remote_addr;
    server 10.0.0.3:9998;
    server 10.0.0.4:9998;
}

stream {
    server {
        listen 9999 ssl;
        ssl_certificate /etc/nginx/bundle.crt;
        ssl_certificate_key /etc/nginx/key.txt;
        proxy_pass nntplexer;
        proxy_protocol off;
    }
}
```

`nntplexer.ini` (internal)

```ini
[server]
addr = "0.0.0.0"
port = 9998
proxy_protocol = off
node = 2 or 10
```

![alt text](https://raw.githubusercontent.com/ucrawler/nntplexer/main/grafana%20dashboard.png)
![alt text](https://raw.githubusercontent.com/ucrawler/nntplexer/main/backends%20table.png)
![alt text](https://raw.githubusercontent.com/ucrawler/nntplexer/main/users%20table.png)
![alt text](https://raw.githubusercontent.com/ucrawler/nntplexer/main/vms.png)
![alt text](https://raw.githubusercontent.com/ucrawler/nntplexer/main/console.png)
