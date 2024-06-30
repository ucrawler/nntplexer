package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/textproto"
	"nntplexer/metrics"
	"strconv"
	"time"
	"strings"
)

var (
	dateFormats = []string{
		"Mon, 02 Jan 06 15:04:05 MST",           // \w+, \d{2} \w+ \d{2} \d{2}:\d{2}:\d{2} \w+
		"Mon, 02 Jan 2006 15:04:05 MST",         // \w+, \d{2} \w+ \d{4} \d{2}:\d{2}:\d{2} \w+
		"02 Jan 2006 15:04:05 MST",              // \d{2} \w+ \d{4} \d{2}:\d{2}:\d{2} \w+
		"02 Jan 06 15:04 MST",                   // \d{2} \w+ \d{2} \d{2}:\d{2} \w+
		"Mon, 02 Jan 2006 15:04:05 -0700",       // \w+, \d{2} \w+ \d{4} \d{2}:\d{2}:\d{2} [+-]{1}\d{4}
		"Mon, 02 Jan 2006 15:04:05 -0700 (MST)", // \w+, \d{2} \w+ \d{4} \d{2}:\d{2}:\d{2} [+-]{1}\d{4} \(\w+\)
		"Mon, 2 Jan 2006 15:04:05 -0700",		 // \w+, \d{1} \w+ \d{4} \d{2}:\d{2}:\d{2} [+-]{1}\d{4}
	}
)

type NNTPBackend struct {
	ur *UserRepository
	br *BackendRepository
	ar *ArticleRepository
	pp *PoolProvider
}

func (b *NNTPBackend) Authenticate(user string, pass string) bool {
	u := b.ur.Get(user)
	h := sha256.Sum256([]byte(pass))
	return hex.EncodeToString(h[:]) == u.Pass
}

func (b *NNTPBackend) CheckConnLimit(user string, conns int) bool {
	u := b.ur.Get(user)
	return int(u.MaxConns) > conns
}

func (b *NNTPBackend) CheckIpLimit(user string, ip string, ips map[string]int) bool {
	u := b.ur.Get(user)

	// ip sharing is enabled or no sessions from this user yet
	// no need for additional checks
	if u.IpSharing || len(ips) == 0 {
		return true
	}

	// check if we already have a connection from this user with same ip
	if _, ok := ips[ip]; ok {
		// ip found, allowing
		return true
	}

	// otherwise disallow
	return false
}

func (b *NNTPBackend) Greeting() string {
	return "201 Hi!"
}

func (b *NNTPBackend) Article(messageId string) (textproto.MIMEHeader, io.Reader, error) {
	metrics.ArticleRequests.Inc()

	backends := b.br.Get()
	if len(backends) == 0 {
		log.Println("[backend] No backends found")
		return nil, nil, &textproto.Error{Code: 403, Msg: "Something went wrong"}
	}

	for _, be := range backends {
////		if !strings.Contains(messageId, "-newzNZB-") && !strings.Contains(messageId, "astraweb") && !strings.Contains(messageId, "easyusenet") && !strings.Contains(messageId, "camelsystem-powerpost.local") && !strings.Contains(messageId, "@nyuu") && !strings.Contains(messageId, "@PRiVATE") && strings.Contains(be.Name, "ninja") {
//			log.Printf("skipping [backend] [%s] skipping %v\n", be.Name, messageId)
//			pool.Return(po)
////			continue
////		}
		if strings.Contains(messageId, "giganews") && strings.Contains(be.Name, "giga") {
//			log.Printf("skipping [backend] [%s] skipping %v\n", be.Name, messageId)
//			pool.Return(po)
			continue
		}
		if strings.Contains(messageId, "xsnews") && strings.Contains(be.Name, "xsnews") {
//			log.Printf("skipping [backend] [%s] skipping %v\n", be.Name, messageId)
//			pool.Return(po)
			continue
		}
		pool := b.pp.GetPool(be)
		po, err := pool.Get()
		if err != nil {
			log.Printf("[backend] [%s] pool.Get: %v\n", be.Name, err)
			continue
		}

		c := po.object
		
		article, err := c.Article(messageId)
		if err != nil {
			log.Printf("[backend] [%s] c.Article: %v\n", be.Name, err)
			log.Printf("[backend] [%s] c.Article: %v\n", be.Name, messageId)

			// handle common protocol errors
			if tperr, ok := err.(*textproto.Error); ok {
				metrics.BackendRequests.With(prometheus.Labels{"backend": be.Name, "code": strconv.Itoa(tperr.Code)}).Inc()

				switch tperr.Code {
				case 430:
					// article not found
					// try next backend
					pool.Return(po)
					continue
				case 400:
					// service not available or no longer available (the server
					// immediately closes the connection).
					// invalidate (close) connection
					// try next backend
					po.Invalidate()
					pool.Return(po)
					continue
				}
			}

			if _, ok := err.(net.Error); ok {
				// FIXME: retry with another connection from this pool
				// net error, drop conn
				po.Invalidate()
			}

			metrics.BackendRequests.With(prometheus.Labels{"backend": be.Name, "code": "0"}).Inc()

			pool.Return(po)
			continue
		}

		metrics.BackendRequests.With(prometheus.Labels{"backend": be.Name, "code": "220"}).Inc()

//disabled		if err := b.parseDate(messageId, article.Headers); err != nil {
//disabled			log.Printf("[backend] [%s] parseDate: %v\n", be.Name, err)
//disabled		}

		body, err := ioutil.ReadAll(article.Body)
		if err != nil {
			// there were some problems with reading article
			// invalidate (close) connection
			// try next backend
			po.Invalidate()
			pool.Return(po)
			continue
		}

		metrics.BackendBytes.With(prometheus.Labels{"backend": be.Name}).Add(float64(len(body)))

		// article fully read, return conn to pool
		pool.Return(po)

		return article.Headers, bytes.NewReader(body), nil
	}

	return nil, nil, &textproto.Error{Code: 430, Msg: "Article not found"}
}

func (b *NNTPBackend) Body(messageId string) (textproto.MIMEHeader, io.Reader, error) {
	metrics.ArticleRequests.Inc()

	backends := b.br.Get()
	if len(backends) == 0 {
		log.Println("[backend] No backends found")
		return nil, nil, &textproto.Error{Code: 403, Msg: "Something went wrong"}
	}

	for _, be := range backends {
		if !strings.Contains(messageId, "-newzNZB-") && !strings.Contains(messageId, "astraweb") && !strings.Contains(messageId, "easyusenet") && !strings.Contains(messageId, "camelsystem-powerpost.local") && !strings.Contains(messageId, "@nyuu") && !strings.Contains(messageId, "@PRiVATE") && strings.Contains(be.Name, "ninja") {
//			log.Printf("skipping [backend] [%s] skipping %v\n", be.Name, messageId)
//			pool.Return(po)
			continue
		}
		if strings.Contains(messageId, "giganews") && strings.Contains(be.Name, "giga") {
//			log.Printf("skipping [backend] [%s] skipping %v\n", be.Name, messageId)
//			pool.Return(po)
			continue
		}
		if strings.Contains(messageId, "xsnews") && strings.Contains(be.Name, "xsnews") {
//			log.Printf("skipping [backend] [%s] skipping %v\n", be.Name, messageId)
//			pool.Return(po)
			continue
		}
		pool := b.pp.GetPool(be)
		po, err := pool.Get()
		if err != nil {
			log.Printf("[backend] [%s] pool.Get: %v\n", be.Name, err)
			continue
		}

		c := po.object

		article, err := c.Body(messageId)
		if err != nil {
//			log.Printf("[backend] [%s] c.Body: %v\n", be.Name, err)
//			log.Printf("[backend] [%s] c.Body: %v\n", be.Name, messageId)

			// handle common protocol errors
			if tperr, ok := err.(*textproto.Error); ok {
				metrics.BackendRequests.With(prometheus.Labels{"backend": be.Name, "code": strconv.Itoa(tperr.Code)}).Inc()

				switch tperr.Code {
				case 430:
					// article not found
					// try next backend
					pool.Return(po)
					continue
				case 400:
					// service not available or no longer available (the server
					// immediately closes the connection).
					// invalidate (close) connection
					// try next backend
					po.Invalidate()
					pool.Return(po)
					continue
				}
			}

			if _, ok := err.(net.Error); ok {
				// FIXME: retry with another connection from this pool
				// net error, drop conn
				po.Invalidate()
			}

			metrics.BackendRequests.With(prometheus.Labels{"backend": be.Name, "code": "0"}).Inc()

			pool.Return(po)
			continue
		}

		metrics.BackendRequests.With(prometheus.Labels{"backend": be.Name, "code": "220"}).Inc()

//disabled		if err := b.parseDate(messageId, article.Headers); err != nil {
//disabled			log.Printf("[backend] [%s] parseDate: %v\n", be.Name, err)
//disabled		}

		body, err := ioutil.ReadAll(article.Body)
		if err != nil {
			// there were some problems with reading article
			// invalidate (close) connection
			// try next backend
			po.Invalidate()
			pool.Return(po)
			continue
		}

		metrics.BackendBytes.With(prometheus.Labels{"backend": be.Name}).Add(float64(len(body)))

		// article fully read, return conn to pool
		pool.Return(po)

		return article.Headers, bytes.NewReader(body), nil
	}

	return nil, nil, &textproto.Error{Code: 430, Msg: "Body not found"}
}

func (b *NNTPBackend) parseDate(id string, headers textproto.MIMEHeader) error {
	date := headers.Get("Date")
	if date == "" {
		return fmt.Errorf("article: %s missing 'Date' header", id)
	}
	for _, dateFormat := range dateFormats {
		ts, err := time.Parse(dateFormat, date)
		if err == nil {
			// date was parsed succesfuly
			// saving
			_ = b.ar.Create(id, ts)
			return nil
		}
	}
	return fmt.Errorf("article: %s unknown date format: %s", id, date)
}

func (b *NNTPBackend) Stats(user string, rx int64, tx int64) {
//disabled	b.ur.Stats(user, rx, tx)
//disabled	b.br.Stats("", tx, rx) // FIXME
}
