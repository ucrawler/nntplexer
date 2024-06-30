package nntpclient

import (
	"fmt"
	"testing"
)

func TestDial(t *testing.T) {
	c, err := Dial("news.eweka.nl:119", &Config{
		ReadTimeout:    0,
		WriteTimeout:   0,
		ConnectTimeout: 3000,
		Encryption:     false,
	})

	if err != nil {
		t.Fatal(err)
	}

	capabilities, err := c.Capabilities()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(capabilities)
}

func TestDialTls(t *testing.T) {
	c, err := Dial("news.eweka.nl:563", &Config{
		ReadTimeout:    0,
		WriteTimeout:   0,
		ConnectTimeout: 3000,
		Encryption:     true,
	})

	if err != nil {
		t.Fatal(err)
	}

	capabilities, err := c.Capabilities()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(capabilities)
}
