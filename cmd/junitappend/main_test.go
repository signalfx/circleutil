package main

import (
	"bytes"
	"encoding/xml"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDecode(t *testing.T) {
	buf := &bytes.Buffer{}
	ts := testSuite{}
	assert.NoError(t, xml.NewEncoder(buf).Encode(ts))
}
