package nntp

import (
	"io"
	"net/textproto"
)

type Article struct {
	Headers textproto.MIMEHeader
	Body    io.Reader
}
