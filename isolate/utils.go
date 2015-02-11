// Copyright 2015 The Swarming Authors. All rights reserved.
// Use of this source code is governed by the Apache v2.0 license that can be
// found in the LICENSE file.

package isolate

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
)

func execCommand(program string, input string, args ...string) []byte {
	cmd := exec.Command(program, args...)
	cmd.Stderr = os.Stderr
	inwriter, _ := cmd.StdinPipe()
	inwriter.Write([]byte(input))
	out, err := cmd.Output()
	if err != nil {
		log.Printf("ERROR: %v with output %v\n", err, out)
		panic(err)
	}
	return out
}

func execPyHelper(run string, input string, args ...string) interface{} {
	var v interface{}
	execPyHelperTyped(&v, run, input, args...)
	return v
}

func execPyHelperTyped(v interface{}, run string, input string, args ...string) {
	path := "python_helper.py"
	fullArgs := append([]string{path, run}, args...)
	bytes := execCommand("python", input, fullArgs...)
	if err := json.Unmarshal(bytes, &v); err != nil {
		log.Printf("bad json [%v] from (%s)\n", err, bytes)
		panic(err)
	}
}

func IsDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	return fileInfo.IsDir(), err
}

func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return v.(string)
}

func FileNameWithoutExtension(path string) string {
	fname := filepath.Base(path)
	return strings.TrimSuffix(fname, filepath.Ext(fname))
}

// TODO: can this be done in generic way? Generic seems a swear wor din Go :(
func flattenFC(chans [](chan FileAsset)) chan FileAsset {
	BUF_SIZE := 10 // TODO(tandrii): const OR determine best?
	out := make(chan FileAsset, BUF_SIZE)
	go func() {
		cases := make([]reflect.SelectCase, len(chans))
		for i, ch := range chans {
			cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv,
				Chan: reflect.ValueOf(ch)}
		}
		remaining := len(cases)
		for remaining > 0 {
			chosen, value, ok := reflect.Select(cases)
			if !ok {
				cases[chosen].Chan = reflect.ValueOf(nil)
				remaining -= 1
				continue
			}
			fa := value.Interface().(FileAsset)
			out <- fa
		}
		close(out)
	}()
	return out
}

func SafeRelPath(basepath, targpath string) string {
	path, err := filepath.Rel(basepath, targpath)
	NOERROR(err)
	return path
}

func CHECK(cond bool) {
	if !cond {
		panic("fail")
	}
}

func NOERROR(err error) {
	if err != nil {
		panic(fmt.Sprintf("fail due to error %s", err))
	}
}

func loadJsonFileVar(filepath string, v interface{}) {
	jsonData, err := ioutil.ReadFile(filepath)
	NOERROR(err)
	err = json.Unmarshal(jsonData, &v)
	NOERROR(err)
}

func loadJsonFile(filepath string) interface{} {
	var v interface{}
	loadJsonFileVar(filepath, &v)
	return v
}

func writeJsonFile(filepath string, v interface{}) {
	jsonData, err := ioutil.ReadFile(filepath)
	NOERROR(err)
	err = json.Unmarshal(jsonData, &v)
	NOERROR(err)
}
