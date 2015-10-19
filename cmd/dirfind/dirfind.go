package main

import (
	"flag"
	"io"
	"log"
	"os"
	"sort"
	"path/filepath"
	"strings"
	"github.com/BurntSushi/toml"
	"io/ioutil"
)

type dirFind struct {
	out       io.Writer
	debugOut io.Writer
	verbose bool
	abs bool
	contains string
	files string

	args []string
}

func (e *dirFind) main() error {
	if e.verbose {
		e.debugOut = os.Stderr
	}
	tlm := tomlLoadedMap{
		log: log.New(e.debugOut, "[dirfind]", 0),
		cache: make(map[string]*ignoreTemplate, 10),
	}
	res, err := tlm.expandPaths(e.args, e.abs)
	if err != nil {
		return err
	}
	res, err = e.filterWithGob(res)
	if err != nil {
		return err
	}
	res, err = e.showFiles(res)
	if err != nil {
		return err
	}
	if len(res) == 0 {
		return nil
	}
	_, err2 := io.WriteString(e.out, strings.Join(res, "\n") + "\n")
	return err2
}

func (e *dirFind) showFiles(dirs []string) ([]string, error) {
	if e.files == "" {
		return dirs, nil
	}
	ret := make([]string, 0, len(dirs))
	for _, d := range dirs {
		matches, err := filesMatching(d, e.files)
		if err != nil {
			return nil, err
		}
		ret = append(ret, matches...)
	}
	return ret, nil
}

func filesMatching(d string, glob string) ([]string, error) {
	files, err := ioutil.ReadDir(d)
	if err != nil {
		return nil, err
	}
	ret := make([]string, 0, len(files))
	for _, f := range files {
		match, err := filepath.Match(glob, f.Name())
		if err != nil {
			return nil, err
		}
		if match {
			ret = append(ret, filepath.Join(d, f.Name()))
		}
	}
	return ret, nil
}

func (e *dirFind) filterWithGob(dirs []string) ([]string, error) {
	if e.contains == "" {
		return dirs, nil
	}
	ret := make([]string, 0, len(dirs))
	for _, d := range dirs {
		matches, err := matches(d, e.contains)
		if err != nil {
			return nil, err
		}
		if matches {
			ret = append(ret, d)
		}
	}
	return ret, nil
}

func matches(d string, glob string) (bool, error) {
	files, err := ioutil.ReadDir(d)
	if err != nil {
		return false, err
	}
	for _, f := range files {
		match, err := filepath.Match(glob, f.Name())
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

type ignoreTemplate struct {
	V Vars `toml:"Vars"`
}

type Vars struct {
	IgnoreDirs []string `toml:"ignoreDirs"`
}

type tomlLoadedMap struct {
	log *log.Logger
	cache map[string]*ignoreTemplate
}

func (t *tomlLoadedMap) loadInDir(dirname string) (*ignoreTemplate, error) {
	if dirname == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		dirname = filepath.Clean(cwd)
	}
	fullFilename := filepath.Join(dirname, "gobuild.toml")
	l, err := os.Stat(fullFilename)
	retInfo := &ignoreTemplate{}
	if err != nil || l.IsDir(){
		return retInfo, nil
	}
	t.log.Printf("Loading filename %s", fullFilename)
	if _, err = toml.DecodeFile(fullFilename, retInfo); err != nil {
		return nil, err
	}
	t.cache[dirname] = retInfo
	return retInfo, nil
}

func (t *tomlLoadedMap) matchDir(storeInto map[string]struct{}, forceAbs bool) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		t.log.Printf("At %s\n", path)
		if err != nil {
			return err
		}
		l, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !l.IsDir() {
			return nil
		}
		finalPath := filepath.Clean(path)
		pathDirName := filepath.Dir(path)
		pathFileName := filepath.Base(finalPath)
		template, err := t.loadInDir(pathDirName)
		if err != nil {
			return err
		}

		t.log.Printf("Ignore for %s is %s parent=%s", path, template.V.IgnoreDirs, pathDirName)
		for _, ignore := range template.V.IgnoreDirs {
			if ignore == pathFileName {
				return filepath.SkipDir
			}
		}
		storeInto[singlePath(path, forceAbs)] = struct{}{}
		return nil
	}
}

func singlePath(path string, forceAbs bool) string {
	path = filepath.Clean(path)
	if !forceAbs && !filepath.IsAbs(path) {
		return filepath.Clean("./" + path)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	symPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return absPath
	}
	return symPath
}

func (t *tomlLoadedMap) expandPaths(paths []string, forceAbs bool) ([]string, error) {
	files := make(map[string]struct{}, len(paths))
	cb := t.matchDir(files, forceAbs)
	for _, path := range paths {
		if strings.HasSuffix(path, "/...") {
			t.log.Printf("At %s\n", path)
			if err := filepath.Walk(filepath.Dir(path), cb); err != nil {
				return nil, err
			}
		} else {
			t.log.Printf("Including path directly: %s", path)
			if l, err := os.Stat(path); err == nil && l.IsDir() {
				files[singlePath(path, forceAbs)] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(files))
	for d := range files {
		out = append(out, d)
	}
	sort.Strings(out)
	return out, nil
}

var mainInstance = dirFind {
	out:    os.Stdout,
	debugOut: ioutil.Discard,
}

func init() {
	flag.BoolVar(&mainInstance.verbose, "verbose", false, "Debug output")
	flag.BoolVar(&mainInstance.abs, "abs", false, "If true, display absolute paths")
	flag.StringVar(&mainInstance.contains, "contains", "", "Only match directories that have a file that matches this gob")
	flag.StringVar(&mainInstance.files, "files", "", "Rather than directories, print files that match this gob inside the directory")
}

func main() {
	flag.Parse()
	mainInstance.args = flag.Args()
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
