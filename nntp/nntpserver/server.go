package nntpserver

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"io"
	"log"
	"net"
	"net/textproto"
	"nntplexer/metrics"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Session struct {
	sync.RWMutex
	id     string
	ip     string
	user   string
	pass   string
	authed bool
	conn   *textproto.Conn
}

func (sess *Session) IsAuthed() bool {
	sess.RLock()
	defer sess.RUnlock()
	return sess.authed
}

func (sess *Session) SetAuthed(authed bool) {
	sess.Lock()
	sess.authed = authed
	sess.Unlock()
}

// Handler is a low-level protocol handler
type Handler func(args []string, sess *Session) error

// The Backend that provides the things and does the stuff.
type Backend interface {
	Greeting() string
	Authenticate(user string, pass string) bool
	CheckConnLimit(user string, conns int) bool
	Article(messageId string) (textproto.MIMEHeader, io.Reader, error)
	Body(messageId string) (textproto.MIMEHeader, io.Reader, error)
	Stats(user string, rx int64, tx int64)
	CheckIpLimit(user string, ip string, ips map[string]int) bool
}

type Server struct {
	sync.RWMutex
	backend  Backend
	handlers map[string]Handler
	sessions map[string][]*Session
}

func NewServer(backend Backend) *Server {
	server := Server{
		backend:  backend,
		handlers: make(map[string]Handler),
		sessions: make(map[string][]*Session),
	}

	server.handlers["capabilities"] = server.handleCapabilities
	server.handlers["authinfo"] = server.handleAuth
	server.handlers["article"] = server.handleArticle
	server.handlers["body"] = server.handleBody
	server.handlers["quit"] = server.handleQuit
	
	server.handlers["head"] = server.handleHead
	server.handlers["group"] = server.handleGroup
	server.handlers["list"] = server.handleList
	server.handlers["mode"] = server.handleMode
	server.handlers["stat"] = server.handleStat

	return &server
}

func (srv *Server) addSession(sess *Session) {
	metrics.ServerSessions.Inc()

	srv.sessions[sess.user] = append(srv.sessions[sess.user], sess)
}

func (srv *Server) removeSession(sess *Session) {
	for index, us := range srv.sessions[sess.user] {
		if us == sess {
			metrics.ServerSessions.Dec()

			srv.sessions[sess.user] = append(srv.sessions[sess.user][:index], srv.sessions[sess.user][index+1:]...)
			break
		}
	}
}

func (srv *Server) getSessionStats(sess *Session) (int, map[string]int) {
	ips := make(map[string]int)
	userSessions, ok := srv.sessions[sess.user]
	if ok {
		for _, cs := range userSessions {
			ips[cs.ip]++
		}

		return len(userSessions), ips
	}
	return 0, ips
}

func (srv *Server) handleCapabilities(args []string, sess *Session) error {
	dw := sess.conn.DotWriter()

	if _, err := fmt.Fprintln(dw, "101 Capability list:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(dw, "VERSION 2"); err != nil {
		return err
	}

	if !sess.IsAuthed() {
		if _, err := fmt.Fprintln(dw, "AUTHINFO USER PASS"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(dw, "READER"); err != nil {
			return err
		}
	}

	return dw.Close()
}

func (srv *Server) handleAuth(args []string, sess *Session) error {
	if sess.IsAuthed() {
		return &textproto.Error{Code: 502, Msg: "Command unavailable"}
	}

	if len(args) < 2 {
		return &textproto.Error{Code: 501, Msg: "Syntax error"}
	}

	switch strings.ToLower(args[0]) {
	case "user":
		sess.user = args[1]
		return sess.conn.PrintfLine("381 Password required")
	case "pass":
		if sess.user == "" {
			return &textproto.Error{Code: 482, Msg: "Authentication commands issued out of sequence"}
		}

		sess.pass = args[1]

		if !srv.backend.Authenticate(sess.user, sess.pass) {
			return &textproto.Error{Code: 481, Msg: "Authentication failed"}
		}

		srv.Lock()
		defer srv.Unlock()

		conns, ips := srv.getSessionStats(sess)

		if !srv.backend.CheckConnLimit(sess.user, conns) {
			return &textproto.Error{Code: 502, Msg: "Too many connections"}
		}

		if !srv.backend.CheckIpLimit(sess.user, sess.ip, ips) {
			return &textproto.Error{Code: 502, Msg: "IP sharing not allowed"}
		}

		sess.SetAuthed(true)
		srv.addSession(sess)

		return sess.conn.PrintfLine("281 Authentication accepted")
	default:
		return &textproto.Error{Code: 501, Msg: "Unknown AUTHINFO option " + args[0]}
	}
}

func (srv *Server) handleArticle(args []string, sess *Session) error {
	if !sess.IsAuthed() {
		return &textproto.Error{Code: 480, Msg: "Authentication required"}
	}

	if len(args) < 1 {
		return &textproto.Error{Code: 501, Msg: "Not enough arguments"}
	}

	messageId := args[0]

	headers, reader, err := srv.backend.Article(messageId)
	if err != nil {
		return err
	}
	_ = sess.conn.PrintfLine("222 0 " + messageId)

	for key, values := range headers {
		for _, value := range values {
			_ = sess.conn.PrintfLine("%s: %s", key, value)
		}
	}

	// there were headers
	// print newline delimiter
	if len(headers) > 0 {
		_ = sess.conn.PrintfLine("")
	}

	dw := sess.conn.DotWriter()
	defer dw.Close()

	bytes, err := io.Copy(dw, reader)
	if bytes > 0 {
		go srv.processStats(bytes, sess)
	}

	return err
}

func (srv *Server) handleBody(args []string, sess *Session) error {
	if !sess.IsAuthed() {
		return &textproto.Error{Code: 480, Msg: "Authentication required"}
	}

	if len(args) < 1 {
		return &textproto.Error{Code: 501, Msg: "Not enough arguments"}
	}

	messageId := args[0]

	_, reader, err := srv.backend.Body(messageId)
	if err != nil {
		return err
	}
	_ = sess.conn.PrintfLine("222 0 " + messageId)

	dw := sess.conn.DotWriter()
	defer dw.Close()

	bytes, err := io.Copy(dw, reader)
	if bytes > 0 {
		go srv.processStats(bytes, sess)
	}

	return err
}

func (srv *Server) handleQuit(args []string, sess *Session) error {
	err := sess.conn.PrintfLine("205 Bye!")
	if err != nil {
		return err
	}
	return io.EOF
	
}

func (srv *Server) handleHead(args []string, sess *Session) error {
	err := sess.conn.PrintfLine("205 Bye!")
	if err != nil {
		return err
	}
	return io.EOF
}

func (srv *Server) handleGroup(args []string, sess *Session) error {
	err := sess.conn.PrintfLine("205 Bye!")
	if err != nil {
		return err
	}
	return io.EOF
}

func (srv *Server) handleList(args []string, sess *Session) error {
	err := sess.conn.PrintfLine("205 Bye!")
	if err != nil {
		return err
	}
	return io.EOF
}

func (srv *Server) handleMode(args []string, sess *Session) error {
	err := sess.conn.PrintfLine("205 Bye!")
	if err != nil {
		return err
	}
	return io.EOF
}

func (srv *Server) handleStat(args []string, sess *Session) error {
	err := sess.conn.PrintfLine("205 Bye!")
	if err != nil {
		return err
	}
	return io.EOF
}

func (srv *Server) Serve(listener net.Listener) error {
	var tempDelay time.Duration // how long to sleep on accept failure

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Printf("http: Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		tempDelay = 0
		go srv.Handle(conn)
	}
}

// Handle an NNTP session.
func (srv *Server) Handle(nc net.Conn) {
	ip, _, err := net.SplitHostPort(nc.RemoteAddr().String())
	if err != nil {
		log.Println(err)
		return
	}

	c := textproto.NewConn(nc)
	defer c.Close()

	sess := &Session{
		id:   uuid.NewString(),
		ip:   ip,
		conn: c,
	}

	// deauth session
	defer func() {
		if sess.IsAuthed() {
			srv.Lock()
			srv.removeSession(sess)
			srv.Unlock()
		}
	}()

	log.Printf("[%s] Connection from %s accepted\n", sess.id, sess.ip)

	if err := c.PrintfLine(srv.backend.Greeting()); err != nil {
		log.Println(err)
		return
	}

	for {
		line, err := c.ReadLine()
		if err != nil {
			log.Println(err)
			return
		}
		if line == "" {
			continue
		}

		parts := strings.Split(line, " ")
		if err := srv.dispatch(parts[0], parts[1:], sess); err != nil {
			if txterr, ok := err.(*textproto.Error); ok {
				metrics.ServerResponses.With(prometheus.Labels{"code": strconv.Itoa(txterr.Code)}).Inc()

				log.Printf("[%s] -> Protocol error: %v\n", sess.id, txterr)
				if err := c.PrintfLine("%d %s", txterr.Code, txterr.Msg); err != nil {
					log.Println(err)
					return
				}
				continue
			}

			switch err {
			case io.EOF:
				return
			default:
				log.Print(err)
				return
			}
		}
		metrics.ServerResponses.With(prometheus.Labels{"code": "0"}).Inc()
	}
}

func (srv *Server) dispatch(cmd string, args []string, sess *Session) error {
	if cmd != "BODY" {
		log.Printf("[%s] <- Command: %s, Arguments: %s\n", sess.id, cmd, args)
	}
	handler, ok := srv.handlers[strings.ToLower(cmd)]
	if !ok {
		metrics.ServerRequests.With(prometheus.Labels{"command": "unknown"}).Inc()

		return &textproto.Error{Code: 500, Msg: "Unrecognized command: " + cmd}
	}

	metrics.ServerRequests.With(prometheus.Labels{"command": strings.ToLower(cmd)}).Inc()
	return handler(args, sess)
}

func (srv *Server) processStats(rx int64, sess *Session) {
	srv.backend.Stats(sess.user, rx, 0)
}
