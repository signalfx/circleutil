package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
	"net/http"
	"net/url"
	"bytes"
	"io/ioutil"
)

type circleTestResult struct {
	Classname  string  `json:"classname"`
	File       *string `json:"file"`
	Name       string  `json:"name"`
	Result     string  `json:"result"`
	RunTime    float64 `json:"run_time"`
	Message    *string `json:"message"`
	Source     string  `json:"source"`
	SourceType string  `json:"source_type"`
}

type circleTestGetResp struct {
	Tests []circleTestResult `json:"tests"`
}

func (c *circleTestGetResp) timeByClass(source string) map[string]float64 {
	ret := make(map[string]float64, len(c.Tests))
	for _, t := range c.Tests {
		if t.Source == source {
			ret[t.Name] = t.RunTime
		}
	}
	return ret
}

type junitAppend struct {
	fileName          string
	suitName          string
	className         string
	testName          string
	testDuration      time.Duration
	failureMsg        string
	failureType       string
	circlePrevResults string
	circleToken string
	circlePrevBuildNum int
	circleUsername string
	circleProject string

	nodeTotal int
	nodeIndex int
	flags     *flag.FlagSet

	client http.Client
}

type testSuites struct {
	XMLName xml.Name     `xml:"testsuites"`
	Tests   []*testSuite `xml:"testsuite"`
}

type testSuite struct {
	Tests    int     `xml:"tests,attr"`
	Failures int     `xml:"failures,attr"`
	Time     float64 `xml:"time,attr"`
	Name     string  `xml:"name,attr"`

	Cases []*testCase `xml:"testcase"`
}

func (t *testSuite) addTest(classname string, testName string, duration time.Duration, failureMsg string, failureType string, failureData string) {
	tc := &testCase{
		ClassName: classname,
		Name:      testName,
		Time:      duration.Seconds(),
	}
	t.Tests++
	if failureMsg != "" {
		tc.Failure = &testFailure{
			Type:    failureType,
			Message: failureMsg,
			Data:    failureData,
		}
		t.Failures++
	}
	t.Time += duration.Seconds()
	t.Cases = append(t.Cases, tc)
}

func (t *testSuites) createOrGetSuit(name string) *testSuite {
	for _, c := range t.Tests {
		if c.Name == name {
			return c
		}
	}
	ret := &testSuite{
		Name: name,
	}
	t.Tests = append(t.Tests, ret)
	return ret
}

type testCase struct {
	ClassName string       `xml:"classname,attr"`
	Name      string       `xml:"name,attr"`
	Time      float64      `xml:"time,attr"`
	Failure   *testFailure `xml:"failure,omitempty"`
}

type testFailure struct {
	Type    string `xml:"type,attr"`
	Message string `xml:"message,attr"`
	Data    string `xml:",chardata"`
}

var mainInstance = junitAppend{
	flags: flag.NewFlagSet(os.Args[0], flag.ExitOnError),
}

func (j *junitAppend) flagInit() error {
	tout := os.Getenv("CIRCLE_TEST_REPORTS")
	defaultFile := ""
	var err error
	if tout != "" {
		if err = os.MkdirAll(filepath.Join(tout, "speedsplit"), 0777); err != nil {
			return err
		}
		defaultFile = filepath.Join(tout, "speedsplit", "speed-junit.xml")
	}

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

	prevBuildStr := os.Getenv("CIRCLE_PREVIOUS_BUILD_NUM")
	prevBuild := int64(0)
	if nodeIndexStr != "" {
		if prevBuild, err = strconv.ParseInt(prevBuildStr, 10, 64); err != nil {
			return err
		}
	}
	j.flags.IntVar(&j.circlePrevBuildNum, "circleprev", int(prevBuild), "Previous build number to load test results from")

	j.flags.StringVar(&j.fileName, "file", defaultFile, "Name of the file to append results to")
	j.flags.StringVar(&j.suitName, "suitname", "speedsplit", "Test suit to operate on")
	j.flags.StringVar(&j.circleToken, "circletoken", os.Getenv("CIRCLE_TOKEN"), "Circle token to use to fetch previous test result ")
	j.flags.StringVar(&j.circleUsername, "circleusername", os.Getenv("CIRCLE_PROJECT_USERNAME"), "Circle project to fetch prev results from")
	j.flags.StringVar(&j.circleProject, "circleproject", os.Getenv("CIRCLE_PROJECT_REPONAME"), "Circle repo to fetch prev results from")
	j.flags.StringVar(&j.className, "classname", "default", "Name of the class the test was on")
	j.flags.StringVar(&j.testName, "testname", "", "Name of the ran test")
	j.flags.DurationVar(&j.testDuration, "testduration", 0, "Length of the test")
	j.flags.StringVar(&j.failureMsg, "failuremsg", "", "A test failure msg")
	j.flags.StringVar(&j.failureType, "failuretype", "", "A test failure type")
	j.flags.StringVar(&j.circlePrevResults, "lastcircle", filepath.Join(os.Getenv("CIRCLE_ARTIFACTS"), "last_circle_tests.json"), "Location of tests result for last circle build")
	return j.flags.Parse(os.Args[1:])
}

func main() {
	if err := mainInstance.main(); err != nil {
		_, err2 := io.WriteString(os.Stderr, err.Error()+"\n")
		logIfNotNil(err2, "Unable to write err to stderr")
		os.Exit(1)
	}
}

func (j *junitAppend) addMsg() error {
	f, err := j.loadFile()
	if err != nil {
		return err
	}
	suit := f.createOrGetSuit(j.suitName)
	suit.addTest(j.className, j.testName, j.testDuration, j.failureMsg, j.failureType, "")
	return j.writeFile(f)
}

func (j *junitAppend) writeFile(toWrite *testSuites) error {
	f, err := os.Create(j.fileName)
	if err != nil {
		return err
	}
	defer func() {
		logIfNotNil(err, "Cannot open file for creation %s", j.fileName)
	}()
	_, err = io.WriteString(f, xml.Header)
	if err != nil {
		return err
	}
	log.Printf("Writing file")
	e := xml.NewEncoder(f)
	e.Indent("", "\t")
	return e.Encode(toWrite)
}

func (j *junitAppend) loadFile() (*testSuites, error) {
	_, err := os.Stat(j.fileName)
	if err != nil && os.IsNotExist(err) {
		return &testSuites{}, nil
	}
	if err != nil {
		return nil, err
	}
	o, err := os.Open(j.fileName)
	if err != nil {
		return nil, err
	}
	defer func() {
		logIfNotNil(o.Close(), "Unable to close %s", j.fileName)
	}()
	var ret testSuites
	if err := xml.NewDecoder(o).Decode(&ret); err != nil {
		return nil, err
	}
	return &ret, nil
}

func (j *junitAppend) loadFromCirclePrevRun() (*circleTestGetResp, error) {
	if j.circleUsername == "" || j.circleProject == "" || j.circlePrevBuildNum == 0 || j.circleToken == "" {
		log.Println("No previous results to fetch")
		return &circleTestGetResp{}, nil
	}
	baseURL := fmt.Sprintf("https://circleci.com/api/v1/project/%s/%s/%d/tests", j.circleUsername, j.circleProject, j.circlePrevBuildNum)
	log.Printf("Fetching base URL %s\n", baseURL)
	baseURL = baseURL + "?" + url.QueryEscape(j.circleToken)
	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/json")
	resp, err := j.client.Do(req)
	if err != nil {
		return nil, err
	}
	fullBody := bytes.Buffer{}
	if _, err := io.Copy(&fullBody, resp.Body); err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(j.circlePrevResults, fullBody.Bytes(), 0777); err != nil {
		return nil, err
	}
	defer func() {
		logIfNotNil(resp.Body.Close(), "Unable to close HTTP response body")
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code %d", resp.StatusCode)
	}
	r := circleTestGetResp{}
	if err := json.NewDecoder(&fullBody).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (j *junitAppend) loadPrevRun() (*circleTestGetResp, error) {
	_, err := os.Stat(j.circlePrevResults)
	if err != nil && os.IsNotExist(err) {
		return j.loadFromCirclePrevRun()
	}
	if err != nil {
		return nil, err
	}

	f, err := os.Open(j.circlePrevResults)
	if err != nil {
		return nil, err
	}
	defer func() {
		logIfNotNil(f.Close(), "Unable to close file %s", j.circlePrevResults)
	}()
	ret := &circleTestGetResp{}
	if err := json.NewDecoder(f).Decode(ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func getAvgTime(times map[string]float64) float64 {
	sum := 0.0
	count := 0
	for _, t := range times {
		count++
		sum += t
	}
	if count == 0 {
		return 1.0
	}
	return sum / float64(count)
}

// split stdin according to your execution index and previous run values
func (j *junitAppend) split() error {
	prevRun, err := j.loadPrevRun()
	if err != nil {
		return err
	}
	times := prevRun.timeByClass(j.suitName)
	avgTime := getAvgTime(times)
	s := bufio.NewScanner(os.Stdin)
	parts := make(map[string]struct{}, 10)
	for s.Scan() {
		l := s.Text()
		parts[l] = struct{}{}
	}
	if err := s.Err(); err != nil {
		return err
	}

	partsToSplit := make([]string, 0, len(parts))
	for p := range parts {
		partsToSplit = append(partsToSplit, p)
	}
	sort.Strings(partsToSplit)

	buckets := make([]float64, j.nodeTotal)
	bucketItem := make([][]string, j.nodeTotal)
	for _, p := range partsToSplit {
		partRunningTime, previousResult := times[p]
		if !previousResult {
			partRunningTime = avgTime
		}
		minBucketIndex := minIndex(buckets)
		buckets[minBucketIndex] += partRunningTime
		bucketItem[minBucketIndex] = append(bucketItem[minBucketIndex], p)
	}
	for _, r := range bucketItem[j.nodeIndex] {
		fmt.Fprintln(os.Stdout, r)
	}
	return nil
}

func minIndex(buckets []float64) int {
	min := 0
	for i := 1; i < len(buckets); i++ {
		if buckets[i] < buckets[min] {
			min = i
		}
	}
	return min
}

func (j *junitAppend) main() error {
	if err := j.flagInit(); err != nil {
		return err
	}
	if len(j.flags.Args()) != 1 {
		fmt.Println(j.flags.Args())
		return errors.New("Must pass one argument as thing to do")
	}

	cmd := j.flags.Arg(0)

	cmdMap := map[string]func() error{
		"add":   j.addMsg,
		"split": j.split,
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
