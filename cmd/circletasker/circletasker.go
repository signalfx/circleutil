package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type circleTasker struct {
	flags      *flag.FlagSet
	args       []string
	portNumber int

	log      *log.Logger
	readFrom io.Reader

	nodeTotal  int
	nodeIndex  int
	sourceHost string
	listenHost string
	readyTimeout time.Duration
	client     http.Client
	listening  chan struct{}
	out        io.Writer
	logOut     io.Writer
}

type splitServer struct {
	listenHost     string
	listening      chan struct{}
	partsToServe   <-chan string
	log            *log.Logger
	maxClientIndex int

	server        http.Server
	haveToldDone  map[int]struct{}
	doneWaitGroup sync.WaitGroup
	mu            sync.Mutex
	indexIsReady map[int]struct{}
}

const sourceIndexHeader = "X-index"

func (s *splitServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	indexStr := req.Header.Get(sourceIndexHeader)
	index, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil {
		s.log.Printf("Invalid X-index %s", indexStr)
		rw.WriteHeader(http.StatusBadRequest)
		_, err := io.WriteString(rw, fmt.Sprintf("Invalid X-index %s", indexStr))
		logIfNotNil(err, "Cannot write response to client")
		return
	}
	if index < 0 || index >= int64(s.maxClientIndex) {
		s.log.Printf("Invalid index %d", index)
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.Method == "HEAD" {
		_, alreadyTold := s.indexIsReady[int(index)]
		if alreadyTold {
			s.log.Printf("Telling index %d twice that I am ready", index)
			rw.WriteHeader(http.StatusBadRequest)
			_, err := io.WriteString(rw, fmt.Sprintf("Index %d was already told to stop", index))
			logIfNotNil(err, "Cannot write response to client")
			return
		}
		s.indexIsReady[int(index)] = struct{}{}
		return
	}
	select {
	case toRet := <-s.partsToServe:
		s.log.Printf("%s -> %d", toRet, index)
		_, err := io.WriteString(rw, toRet)
		logIfNotNil(err, "Cannot write response to client")
	default:
		_, alreadyTold := s.haveToldDone[int(index)]
		if alreadyTold {
			s.log.Printf("Telling index %d twice", index)
			rw.WriteHeader(http.StatusBadRequest)
			_, err := io.WriteString(rw, fmt.Sprintf("Index %d was already told to stop", index))
			logIfNotNil(err, "Cannot write response to client")
			return
		}
		s.haveToldDone[int(index)] = struct{}{}
		s.log.Printf("Done: %d", index)
		rw.WriteHeader(http.StatusNoContent)
		if f, ok := rw.(http.Flusher); ok {
			f.Flush()
		}
		s.doneWaitGroup.Done()
		return
	}
}

func (s *splitServer) start() error {
	errChan := make(chan error, 1)
	l, err := net.Listen("tcp", s.listenHost)
	if err != nil {
		return err
	}
	defer func() {
		logIfNotNil(l.Close(), "Cannot close listen port")
	}()
	s.server.Handler = s
	s.server.ErrorLog = s.log
	go func() {
		close(s.listening)
		errChan <- s.server.Serve(l)
	}()
	s.doneWaitGroup.Wait()
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

var mainInstance = circleTasker{
	flags:     flag.NewFlagSet(os.Args[0], flag.ExitOnError),
	args:      os.Args[1:],
	out:       os.Stdout,
	readFrom:  os.Stdin,
	listening: make(chan struct{}),
	logOut:    os.Stderr,
}

func (j *circleTasker) flagInit() error {
	var err error
	numOfBucketsStr := os.Getenv("CIRCLE_NODE_TOTAL")
	nodeTotal := int64(1)
	if numOfBucketsStr != "" {
		if nodeTotal, err = strconv.ParseInt(numOfBucketsStr, 10, 64); err != nil {
			return err
		}
	}
	j.flags.IntVar(&j.nodeTotal, "node_total", int(nodeTotal), "Number of nodes to split into")

	nodeIndexStr := os.Getenv("CIRCLE_NODE_INDEX")
	nodeIndex := int64(0)
	if nodeIndexStr != "" {
		if nodeIndex, err = strconv.ParseInt(nodeIndexStr, 10, 64); err != nil {
			return err
		}
	}
	j.flags.IntVar(&j.nodeIndex, "node_index", int(nodeIndex), "Index of the node we're building")
	j.flags.StringVar(&j.sourceHost, "source_host", "node0", "Source host to get information from")
	j.flags.StringVar(&j.listenHost, "listenhost", "0.0.0.0:2121", "Listen addr if a server")
	j.flags.DurationVar(&j.client.Timeout, "timeout", time.Second * 30, "Timeout waiting for HTTP responses")
	j.flags.DurationVar(&j.readyTimeout, "ready_timeout", time.Minute * 5, "Timeout waiting for ready signal")
	j.flags.IntVar(&j.portNumber, "port", 2121, "Port to use for connections")
	j.log = log.New(j.logOut, "[circletasker]", log.LstdFlags)
	return j.flags.Parse(j.args)
}

func main() {
	if err := mainInstance.main(); err != nil {
		_, err2 := io.WriteString(os.Stderr, err.Error()+"\n")
		logIfNotNil(err2, "Unable to write err to stderr")
		os.Exit(1)
	}
}

func (j *circleTasker) next() error {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:%d", j.sourceHost, j.portNumber), nil)
	if err != nil {
		return err
	}
	req.Header.Add(sourceIndexHeader, strconv.FormatInt(int64(j.nodeIndex), 10))
	resp, err := j.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		logIfNotNil(resp.Body.Close(), "cannot close client response body")
	}()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode == http.StatusOK {
		_, err := io.Copy(j.out, resp.Body)
		return err
	}
	b, err := ioutil.ReadAll(resp.Body)
	logIfNotNil(err, "Cannot read from response body")
	j.log.Println(string(b))
	return fmt.Errorf("invalid status code %d", resp.StatusCode)
}

func (j *circleTasker) serve() error {
	j.log.Println("Reading lines from stdin")
	allLines := make([]string, 0, 100)
	r := bufio.NewReaderSize(j.readFrom, 20000)
	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		if line != "" {
			allLines = append(allLines, line)
		}
		if err == io.EOF {
			break
		}
	}
	s := make(chan string, len(allLines))
	for _, l := range allLines {
		s <- strings.TrimSpace(l)
	}
	j.log.Printf("Read %d lines\n", len(s))
	ss := splitServer{
		listenHost:     j.listenHost,
		partsToServe:   s,
		log:            j.log,
		maxClientIndex: j.nodeTotal,
		haveToldDone:   make(map[int]struct{}),
		indexIsReady: make(map[int]struct{}),
		listening:      j.listening,
	}
	ss.doneWaitGroup.Add(j.nodeTotal)
	j.log.Println("Starting server")
	return ss.start()
}

func (j *circleTasker) ready() error {
	now := time.Now()
	for {
		req, err := http.NewRequest("HEAD", fmt.Sprintf("http://%s:%d", j.sourceHost, j.portNumber), nil)
		if err != nil {
			return err
		}
		// 5 minute timeout on client requests
		req.Header.Add(sourceIndexHeader, strconv.FormatInt(int64(j.nodeIndex), 10))
		resp, err := j.client.Do(req)
		if err != nil {
			if time.Now().Sub(now).Nanoseconds() <= j.readyTimeout.Nanoseconds() {
				continue
			}
		}
		defer func() {
			logIfNotNil(resp.Body.Close(), "cannot close client response body")
		}()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		b, err := ioutil.ReadAll(resp.Body)
		logIfNotNil(err, "Cannot read from response body")
		j.log.Println(string(b))
		return fmt.Errorf("invalid status code %d", resp.StatusCode)
	}
}

func (j *circleTasker) main() error {
	if err := j.flagInit(); err != nil {
		return err
	}
	if len(j.flags.Args()) != 1 {
		fmt.Println(j.flags.Args())
		return errors.New("Must pass one argument as thing to do")
	}

	cmd := j.flags.Arg(0)

	cmdMap := map[string]func() error{
		"serve": j.serve,
		"next":  j.next,
		"ready": j.ready,
	}

	f, exists := cmdMap[cmd]
	if !exists {
		return fmt.Errorf("Unknown command %s", cmd)
	}
	return f()
}

func logIfNotNil(err error, msg string, args ...interface{}) {
	if err != nil {
		log.Printf(msg, args...)
	}
}
