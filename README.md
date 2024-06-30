# nntplexer

Start your own independant tier 2 usenet business without the need for any storage 

or

Simply share your account with friends and family using your home computer

Usenet providers don't allow an account to be used from multiple IP's. Running a plexer allows you to bypass it. Your users are not allowed to share their account from multiple ip's. They can run the same plexer to bypass it.

Compiles to 1 binary to execute with ease for any OS and does multicore.

Inspired by https://github.com/ovpn-to/oVPN.to-Advanced-NNTP-Proxy

NNTP protocol multiplexer with auth, stats, multiple backends, etc.

1 gbit throughput per vm core, backends are tried in sequential order by priority in case of missing article.

You need:

1 or more hetzner 2 core vm's for internal

and

1 upto 4 hetzner 1 core vm's for external (RRDNS)

and

1 or more accounts from https://whatsmyuse.net/ for every internal backend backbone.

Clients are load balanced using ip hash, hit the same internal always to protect your accounts :-)

Destroy and re-create VM when reaching 20 TB to reset bandwidth and avoid paying the 1 EUR per TB surcharge.

I suggest 1 external and 1 internal to start out with to 'hide' the IP fetching the article. :-)

Only external counts for bandwidth.

1 ngninja account allows you to split 50 connections between upto 50 accounts at the same time to access usenet with over 2 gbit/s speed.

Add 3 accounts and set your client to 150 connections for a chuckle.

You need to shorten the distance as much as possible between your proxied account's host and the internal.

A VM in Ashburn, Virginia will give you these results with above account. Expirement from there.

Just imagine the flow in your head, total latency between all hops == speed. 

Usenet -> internal -> client and client -> internal -> Usenet is travelled every message ID. 2 gbit/sec = 285 roundtrips per sec per backend.

Will you be able to reach https://www.fdcservers.net/configurator?fixedFilter=15&fixedFilterType=bandwidth_option ?

Does anyone really know how usenet works? Imagine there is only 1 backbone in reality and you can lease it to start another backbone and you start fresh with no 'missing articles'...
8 backbones storing 300 TB daily feed size == 2400 TB of new storage being added on a daily business. Do you work together or struggle together???

# todo

cache articles on mongodb using ttl 

or

cache articles 'multi continent' conveniently to externals using cf r2

Use jbod for article caching, we don't care about data loss and want to minimize data loss on a drive issue. Spread articles between jbod's using postdate somehow.

skip backends using article postdate and backend retention

skip backend when poster name doesn't match :-)

replace mysql with sqlite

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

![alt text](https://raw.githubusercontent.com/ucrawler/nntplexer/main/grafana.png)
![alt text](https://raw.githubusercontent.com/ucrawler/nntplexer/main/backends%20table.png)
![alt text](https://raw.githubusercontent.com/ucrawler/nntplexer/main/users%20table.png)
![alt text](https://raw.githubusercontent.com/ucrawler/nntplexer/main/vms.png)
![alt text](https://raw.githubusercontent.com/ucrawler/nntplexer/main/console.png)
![alt text](https://raw.githubusercontent.com/ucrawler/nntplexer/main/article%20succes%20rate.png)
