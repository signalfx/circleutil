package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
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

	nodeTotal    int
	nodeIndex    int
	runRes       string
	sourceHost   string
	listenHost   string
	readyTimeout time.Duration
	client       http.Client
	listening    chan struct{}
	out          io.Writer
	logOut       io.Writer
}

type splitServer struct {
	listenHost     string
	listening      chan struct{}
	partsToServe   []string
	log            *log.Logger
	maxClientIndex int

	server        http.Server
	haveToldDone  map[int]struct{}
	doneWaitGroup sync.WaitGroup
	mu            sync.Mutex
	indexIsReady  map[int]struct{}

	processResults   map[string]time.Duration
	processStartTime map[int]startTime
}

type startTime struct {
	sentItem string
	sendTime time.Time
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
	now := time.Now()
	lastItem, exists := s.processStartTime[int(index)]
	if exists {
		s.processResults[lastItem.sentItem] = now.Sub(lastItem.sendTime)
		delete(s.processStartTime, int(index))
	}
	if len(s.partsToServe) != 0 {
		var toRet string
		toRet, s.partsToServe = s.partsToServe[0], s.partsToServe[1:]
		s.log.Printf("%s -> %d", toRet, index)
		_, err := io.WriteString(rw, toRet)
		logIfNotNil(err, "Cannot write response to client")
		s.processStartTime[int(index)] = startTime{
			sentItem: toRet,
			sendTime: now,
		}
		return
	}
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

	j.flags.StringVar(&j.runRes, "run_res", filepath.Join(os.Getenv("CIRCLE_ARTIFACTS"), "circletasker.json"), "Filename to store results into")
	j.flags.StringVar(&j.sourceHost, "source_host", "localhost", "Source host to get information from")
	j.flags.StringVar(&j.listenHost, "listenhost", "0.0.0.0:12012", "Listen addr if a server")
	j.flags.DurationVar(&j.client.Timeout, "timeout", time.Second*30, "Timeout waiting for HTTP responses")
	j.flags.DurationVar(&j.readyTimeout, "ready_timeout", time.Minute*5, "Timeout waiting for ready signal")
	j.flags.IntVar(&j.portNumber, "port", 12012, "Port to use for connections")
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
	for idx, l := range allLines {
		allLines[idx] = strings.TrimSpace(l)
	}
	j.log.Printf("Read %d lines\n", len(allLines))
	ss := splitServer{
		listenHost:       j.listenHost,
		partsToServe:     allLines,
		log:              j.log,
		maxClientIndex:   j.nodeTotal,
		haveToldDone:     make(map[int]struct{}),
		indexIsReady:     make(map[int]struct{}),
		listening:        j.listening,
		processStartTime: make(map[int]startTime, j.nodeTotal),
		processResults:   make(map[string]time.Duration, len(allLines)),
	}
	writeInto, err := os.Create(j.runRes)
	if err != nil {
		return err
	}
	defer func() {
		logIfNotNil(writeInto.Close(), "Cannot close/flush results file")
	}()
	defer func() {
		logIfNotNil(json.NewEncoder(writeInto).Encode(ss.processResults), "Cannot encode JSON process times")
	}()
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
			return err
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
