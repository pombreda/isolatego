// Copyright 2015 The Swarming Authors. All rights reserved.
// Use of this source code is governed by the Apache v2.0 license that can be
// found in the LICENSE file.

package main

import (
	"log"
	"os"

	"github.com/shishkander/isolatego/isolate"
)

func main() {
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	os.Exit(isolate.Run())
}
