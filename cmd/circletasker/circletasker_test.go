package main

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestServer(t *testing.T) {
	done := sync.WaitGroup{}
	freePort := "15283"
	done.Add(1)
	server := circleTasker{
		flags:     flag.NewFlagSet(os.Args[0], flag.ExitOnError),
		args:      []string{"-listenhost", "localhost:" + freePort, "serve"},
		out:       &bytes.Buffer{},
		readFrom:  strings.NewReader("hello\nworld"),
		logOut:    &bytes.Buffer{},
		listening: make(chan struct{}),
	}
	go func() {
		e1 := server.main()
		if e1 != nil {
			t.Fatalf("Unexpected error %s", e1.Error())
		}
		done.Done()
	}()
	done.Add(1)

	readyLoop := func() {
		client := circleTasker{
			flags:    flag.NewFlagSet(os.Args[0], flag.ExitOnError),
			args:     []string{"-source_host", "localhost", "-port", freePort, "ready"},
			out:      &bytes.Buffer{},
			readFrom: &bytes.Buffer{},
			logOut:   &bytes.Buffer{},
		}
		e1 := client.main()
		if e1 != nil {
			t.Fatalf("Unexpected error %s", e1.Error())
		}
	}

	readFrom := func() string {
		client := circleTasker{
			flags:    flag.NewFlagSet(os.Args[0], flag.ExitOnError),
			args:     []string{"-source_host", "localhost", "-port", freePort, "next"},
			out:      &bytes.Buffer{},
			readFrom: &bytes.Buffer{},
			logOut:   &bytes.Buffer{},
		}
		e1 := client.main()
		if e1 != nil {
			t.Fatalf("Unexpected error %s", e1.Error())
		}
		return client.out.(*bytes.Buffer).String()
	}

	go func() {
		readyLoop()
		<-server.listening
		s1 := readFrom()
		if s1 != "hello" {
			t.Fatal(s1)
		}
		s2 := readFrom()
		if s2 != "world" {
			t.Fatal(s2)
		}
		s3 := readFrom()
		if s3 != "" {
			t.Fatal(s3)
		}
		done.Done()
	}()
	done.Wait()
}
