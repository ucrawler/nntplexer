package main

import (
	"errors"
	"fmt"
	"net"
	"nntplexer/nntp/nntpclient"
	"strconv"
	"sync"
	"time"
)

type PooledObject struct {
	sync.Mutex
	object *nntpclient.Client
	valid  bool
}

// Invalidate marks object as invalid.
// Such object won't be returned to pool.
func (po *PooledObject) Invalidate() {
	po.Lock()
	po.valid = false
	po.Unlock()
}

type ClientPool struct {
	sync.Mutex
	idle        []*PooledObject
	active      []*PooledObject
	capacity    int
	fails       int
	maxFails    int
	failTimeout int
	checked     time.Time
	factory     func() (*PooledObject, error)
}

func (p *ClientPool) Get() (*PooledObject, error) {
	p.Lock()
	defer p.Unlock()

	now := time.Now()

	if len(p.idle) == 0 {
		if len(p.active) == p.capacity {
			return nil, errors.New("pool is busy")
		}

		// see nginx implementation
		// https://github.com/nginx/nginx/blob/release-1.21.0/src/http/ngx_http_upstream_round_robin.c#L554
		if p.maxFails > 0 &&
			p.fails >= p.maxFails &&
			now.Sub(p.checked) <= time.Duration(p.failTimeout)*time.Second {
			// pool failure
			return nil, errors.New("pool temporarily disabled")
		}

		p.checked = now

		client, err := p.factory()

		// some error occured during new conn creation
		if err != nil {
			// increment fails
			p.fails++
			if p.fails >= p.maxFails {
				return nil, fmt.Errorf("pool temporarily disabled: %v", err)
			}
			return nil, err
		}

		p.fails = 0
		p.active = append(p.active, client)

		return client, nil
	} else {
		client := p.idle[0]
		p.idle = p.idle[1:]
		p.active = append(p.active, client)

		return client, nil
	}
}

func (p *ClientPool) Return(po *PooledObject) {
	p.Lock()
	defer p.Unlock()
	for index, c := range p.active {
		if c == po {
			if po.valid {
				p.idle = append(p.idle, po)
			} else {
				_ = po.object.Close()
			}
			p.active = append(p.active[:index], p.active[index+1:]...)
			break
		}
	}
}

type PoolProvider struct {
	sync.Mutex
	pools map[string]*ClientPool
}

func NewPoolProvider() *PoolProvider {
	return &PoolProvider{
		pools: make(map[string]*ClientPool),
	}
}

func (cp *PoolProvider) GetPool(backend Backend) *ClientPool {
	cp.Lock()
	defer cp.Unlock()

	if _, ok := cp.pools[backend.Name]; !ok {
		cp.pools[backend.Name] = &ClientPool{
			Mutex:       sync.Mutex{},
			idle:        make([]*PooledObject, 0, backend.MaxConns),
			active:      make([]*PooledObject, 0, backend.MaxConns),
			capacity:    int(backend.MaxConns),
			fails:       0,
			maxFails:    int(backend.MaxFails),
			failTimeout: int(backend.FailTimeout),
			factory: func() (*PooledObject, error) {
				addr := net.JoinHostPort(backend.Host, strconv.Itoa(int(backend.Port)))

				client, err := nntpclient.Dial(addr, &nntpclient.Config{
					ReadTimeout:    0,
					WriteTimeout:   0,
					ConnectTimeout: int(backend.ConnectTimeout),
					Encryption:     backend.UseTLS,
				})
				if err != nil {
					return nil, err
				}

				authenticate, err := client.Authenticate(backend.User, backend.Pass)
				if err != nil {
					_ = client.Close()
					return nil, err
				}

				if !authenticate {
					_ = client.Close()
					return nil, errors.New("authentication failed")
				}

				return &PooledObject{object: client, valid: true}, nil
			},
		}
	}

	return cp.pools[backend.Name]
}
