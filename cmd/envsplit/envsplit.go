package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
)

type envSplit struct {
	indexEnv  string
	totalEnv  string
	readStdin bool
	getEnv    func(string) (string, bool)
	out       io.Writer
}

var errZeroTotal = errors.New("total index value of zero is invalid")

func (e *envSplit) indexes() (uint64, uint64, error) {
	index, exists := e.getEnv(e.indexEnv)
	if !exists {
		return 0, 0, fmt.Errorf("Unable to find env var %s", e.indexEnv)
	}
	total, exists := e.getEnv(e.totalEnv)
	if !exists {
		return 0, 0, fmt.Errorf("Unable to find env var %s", e.totalEnv)
	}
	indexInt, err := strconv.ParseUint(index, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	totalInt, err := strconv.ParseUint(total, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	if totalInt == 0 {
		return 0, 0, errZeroTotal
	}
	if indexInt >= totalInt {
		return 0, 0, fmt.Errorf("invalid index %d >= %d", indexInt, totalInt)
	}
	return indexInt, totalInt, nil
}

func (e *envSplit) main() error {
	indexInt, totalInt, err := e.indexes()
	if err != nil {
		return err
	}
	f := indexesFunc(indexInt, totalInt)
	if e.readStdin {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			if err := f(e.out, s.Text()); err != nil {
				return err
			}
		}
		return s.Err()
	}
	for _, s := range flag.Args() {
		if err := f(e.out, s); err != nil {
			return err
		}
	}
	return nil
}

func indexesFunc(indexInt uint64, totalInt uint64) func(io.Writer, string) error {
	index := uint64(0)
	return func(out io.Writer, toWrite string) error {
		if uint64(index)%totalInt == indexInt {
			if _, err := io.WriteString(out, toWrite); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\n"); err != nil {
				return err
			}
		}
		index++
		return nil
	}
}

var mainInstance = envSplit{
	getEnv: os.LookupEnv,
	out:    os.Stdout,
}

func init() {
	flag.StringVar(&mainInstance.indexEnv, "index_env", "CIRCLE_NODE_INDEX", "Env name of node index")
	flag.StringVar(&mainInstance.totalEnv, "total_env", "CIRCLE_NODE_TOTAL", "Env name of node total")
	flag.BoolVar(&mainInstance.readStdin, "stdin", false, "Read splits from stdin not args")
}

func main() {
	flag.Parse()
	if err := mainInstance.main(); err != nil {
		_, err2 := io.WriteString(os.Stderr, err.Error()+"\n")
		logIfNotNil(err2, "Unable to write err to stderr")
		os.Exit(1)
	}
}

func logIfNotNil(err error, msg string, args ...interface{}) {
	if err != nil {
		log.Printf(msg, args...)
	}
}
