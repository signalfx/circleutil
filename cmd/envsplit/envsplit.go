package main
import (
	"flag"
	"os"
	"fmt"
	"strconv"
	"bufio"
"io"
"errors"
)

type EnvSplit struct {
	indexEnv string
	totalEnv string
	readStdin bool
	getEnv func(string) (string, bool)
	out io.Writer
}

var errZeroTotal = errors.New("total index value of zero is invalid")

func (e *EnvSplit) main() error {
	index, exists := e.getEnv(e.indexEnv)
	if !exists {
		return fmt.Errorf("Unable to find env var %s", e.indexEnv)
	}
	total, exists := e.getEnv(e.totalEnv)
	if !exists {
		return fmt.Errorf("Unable to find env var %s", e.totalEnv)
	}
	indexInt, err := strconv.ParseUint(index, 10, 64)
	if err != nil {
		return err
	}
	totalInt, err := strconv.ParseUint(total, 10, 64)
	if err != nil {
		return err
	}
	if totalInt == 0 {
		return errZeroTotal
	}
	if indexInt >= totalInt {
		return fmt.Errorf("invalid index %d >= %d", indexInt, totalInt)
	}
	if e.readStdin {
		s := bufio.NewScanner(os.Stdin)
		index := uint64(0)
		for s.Scan() {
			txt := s.Text()
			if index % totalInt == indexInt {
				if _, err := io.WriteString(e.out, txt); err != nil {
					return err
				}
				if _, err := io.WriteString(e.out, "\n"); err != nil {
					return err
				}
			}
			index++
		}
		return s.Err()
	}
	for index, s := range flag.Args() {
		if uint64(index) % totalInt == indexInt {
			if _, err := io.WriteString(e.out, s); err != nil {
				return err
			}
			if _, err := io.WriteString(e.out, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

var Main = EnvSplit{
	getEnv: os.LookupEnv,
	out: os.Stdout,
}

func init() {
	flag.StringVar(&Main.indexEnv, "index_env", "CIRCLE_NODE_INDEX", "Env name of node index")
	flag.StringVar(&Main.totalEnv, "total_env", "CIRCLE_NODE_TOTAL", "Env name of node total")
	flag.BoolVar(&Main.readStdin, "stdin", false, "Read splits from stdin not args")
}

func main() {
	flag.Parse()
	if err := Main.main(); err != nil {
		io.WriteString(os.Stderr, err.Error() + "\n")
		os.Exit(1)
	}
}
