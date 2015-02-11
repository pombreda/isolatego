// Copyright 2015 The Swarming Authors. All rights reserved.
// Use of this source code is governed by the Apache v2.0 license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
