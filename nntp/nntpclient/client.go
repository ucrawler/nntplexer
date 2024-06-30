package nntpclient

import (
	"crypto/tls"
	"net"
	"net/textproto"
	"nntplexer/nntp"
	"time"
)

type Client struct {
	text    *textproto.Conn
	config  *Config
	code    int
	message string
}

type Config struct {
	ReadTimeout    int // TODO
	WriteTimeout   int // TODO
	ConnectTimeout int
	Encryption     bool
}

func Dial(addr string, config *Config) (*Client, error) {
	var conn net.Conn
	var err error

	timeout := time.Duration(config.ConnectTimeout)*time.Millisecond

	if config.Encryption {
		dialer := net.Dialer{
			Timeout: timeout,
		}
		conn, err = tls.DialWithDialer(&dialer, "tcp", addr, &tls.Config{})
		if err != nil {
			return nil, err
		}
	} else {
		conn, err = net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return nil, err
		}
	}

	return NewClient(conn, config)
}

func NewClient(conn net.Conn, config *Config) (*Client, error) {
	text := textproto.NewConn(conn)
	code, message, err := text.ReadCodeLine(20)
	if err != nil {
		return nil, err
	}
	c := &Client{
		text:    text,
		config:  config,
		code:    code,
		message: message,
	}
	return c, nil
}

// GetCode returns last operation response code
func (c *Client) GetCode() int {
	return c.code
}

// GetMessage returns last operation response message
func (c *Client) GetMessage() string {
	return c.message
}

// Close closes the connection.
func (c *Client) Close() error {
	return c.text.Close()
}

func (c *Client) Capabilities() ([]string, error) {
	if err := c.Cmd(101, "CAPABILITIES"); err != nil {
		return nil, err
	}

	return c.text.ReadDotLines()
}

func (c *Client) Authenticate(user string, pass string) (bool, error) {
	if err := c.Cmd(381, "AUTHINFO USER "+user); err != nil {
		return false, err
	}

	if err := c.Cmd(281, "AUTHINFO PASS "+pass); err != nil {
		return false, err
	}

	return true, nil
}

func (c *Client) Article(id string) (*nntp.Article, error) {
	if err := c.Cmd(220, "ARTICLE "+id); err != nil {
		return nil, err
	}

	header, err := c.text.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}

	return &nntp.Article{
		Headers: header,
		Body:    c.text.DotReader(),
	}, nil
}

func (c *Client) Body(id string) (*nntp.Article, error) {
	if err := c.Cmd(222, "BODY "+id); err != nil {
		return nil, err
	}

	return &nntp.Article{
		Headers: make(textproto.MIMEHeader),
		Body:    c.text.DotReader(),
	}, nil
}

func (c *Client) Cmd(expectCode int, cmd string) error {
	id, err := c.text.Cmd(cmd)
	if err != nil {
		return err
	}
	c.text.StartResponse(id)
	defer c.text.EndResponse(id)

	code, message, err := c.text.ReadCodeLine(expectCode)
	if err != nil {
		return err
	}

	c.code = code
	c.message = message

	return nil
}
