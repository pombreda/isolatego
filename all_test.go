package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExecPyHelper(t *testing.T) {
	ijson := execPyHelper("test_sum", "", "1", "2")
	s := ijson.(float64)
	if s != 3 {
		t.Error(fmt.Sprintf("unexpected sum %v", s))
	}
}

func TestFileNameWithoutExtension(t *testing.T) {
	f := FileNameWithoutExtension
	assert.Equal(t, f("base.txt"), "base")
	assert.Equal(t, f("s/base.txt"), "base")
	assert.Equal(t, f("s/base.sub.txt"), "base.sub")
	assert.Equal(t, f("/whatever/base.sub..txt"), "base.sub.")
}
