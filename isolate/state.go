// Copyright 2015 The Swarming Authors. All rights reserved.
// Use of this source code is governed by the Apache v2.0 license that can be
// found in the LICENSE file.

package isolate

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const ISOLATED_GEN_JSON_VERSION = 1

type ArchiveOptions struct {
	Isolate             string
	Isolated            string
	Subdir              string
	Ignore_broken_items bool
	Blacklist           []string
	Path_variables      map[string]string
	Extra_variables     map[string]string
	Config_variables    map[string]interface{}
}

type FileMetadata struct {
	meta map[string]string
}

func (m *FileMetadata) IsSymlink() bool {
	return m.meta["l"] != ""
}
func (m *FileMetadata) IsHighPriority() bool {
	return m.meta["priority"] == "0"
}
func (m *FileMetadata) GetDigest() string {
	return m.meta["h"]
}

func (m *FileMetadata) GetSize() int64 {
	v, _ := strconv.ParseInt(m.meta["s"], 10, 64)
	return v
}

type FileAsset struct {
	*FileMetadata
	fullPath string
}

func prepare_for_archival(opts ArchiveOptions, cwd string) (
	fileAssets chan FileAsset, isolatedDigest string, err error) {
	cstate, err := load_complete_state(opts, cwd, opts.Subdir, false)
	log.Printf("cstate %v", cstate)
	fileAssets = make(chan FileAsset)
	go func() {
		// TODO:
		close(fileAssets)
	}()
	return
}

func load_complete_state(opts ArchiveOptions, cwd string, subdir string,
	skipUpdate bool) (cstate CompleteState, err error) {
	if opts.Isolated != "" {
		// Load the previous state if it was present. Namely, "foo.isolated.state".
		// Note: this call doesn't load the .isolate file.
		cstate = load_files(opts.Isolated)
	} else {
		// Constructs a dummy object that cannot be saved. Useful for temporary
		// commands like 'run'. There is no directory containing a .isolated file so
		// specify the current working directory as a valid directory.
		wd, err := os.Getwd()
		NOERROR(err)
		cstate = CompleteState{"", CreateSavedState(wd)}
	}
	isolate := ""
	if opts.Isolate != "" {
		if cstate.savedState.isolate_file != "" {
			CHECK(skipUpdate)
			isolate = ""
		} else {
			isolate = cstate.savedState.isolateFilepath
		}
	} else {
		isolate = opts.Isolate
		relIsolate := SafeRelPath(opts.Isolate, cstate.savedState.isolatedBasedir)
		CHECK(relIsolate == cstate.savedState.isolate_file)
	}
	if !skipUpdate {
		// Then load the .isolate and expands directories.
		cstate.load_isolate(cwd, isolate, opts.Path_variables, opts.Config_variables,
			opts.Extra_variables, opts.Blacklist, opts.Ignore_broken_items)
	}

	if subdir != "" {
		// This is tricky here. If it is a path, take it from the root_dir. If
		// it is a variable, it must be keyed from the directory containing the
		// .isolate file. So translate all variables first.
		translated_path_variables := make(map[string]string)
		for k, v := range cstate.savedState.Path_variables {
			translated_path_variables[k] = filepath.Clean(
				filepath.Join(cstate.savedState.relative_cwd, v))
		}
		subdir = isolate_format_eval_variables(subdir, translated_path_variables)
		subdir = strings.Replace(subdir, "/", string(os.PathSeparator), -1)
	}

	if !skipUpdate {
		cstate.filesToMetadata(subdir)
	}
	return
}

type SavedStateMembers struct {
	// Value of sys.platform so that the file is rejected if loaded from a
	// different OS. While this should never happen in practice, users are ...
	// "creative".
	OS string
	// Algorithm used to generate the hash. The only supported value is at the
	// time of writting 'sha-1'.
	Algo string
	// List of included .isolated files. Used to support/remember 'slave'
	// .isolated files. Relative path to isolated_basedir.
	Child_isolated_files []string
	// Cache of the processed command. This value is saved because .isolated
	// files are never loaded by isolate.py so it's the only way to load the
	// command safely.
	Command []string
	// GYP variables that are used to generate conditions. The most frequent
	// example is 'OS'.
	Config_variables map[string]interface{}
	// GYP variables that will be replaced in 'command' and paths but will not be
	// considered a relative directory.
	Extra_variables map[string]string
	// Cache of the files found so the next run can skip hash calculation.
	files []string
	// Path of the original .isolate file. Relative path to isolated_basedir.
	isolate_file string
	// GYP variables used to generate the .isolated files paths based on path
	// variables. Frequent examples are DEPTH and PRODUCT_DIR.
	Path_variables map[string]string
	// If the generated directory tree should be read-only. Defaults to 1.
	read_only bool
	// Relative cwd to use to start the command.
	relative_cwd string
	// Root directory the files are mapped from.
	root_dir string
	// Version of the saved state file format. Any breaking change must update
	// the value.
	version string
}

type SavedState struct {
	SavedStateMembers

	cwd             string
	isolateFilepath string
	isolatedBasedir string
}

func CreateSavedState(cwd string) SavedState {
	var s SavedState
	s.cwd = cwd
	return s
}

type CompleteState struct {
	isolatedFilepath string
	savedState       SavedState
}

func (cstate *CompleteState) load_isolate(
	cwd, isolate string,
	Path_variables map[string]string,
	Config_variables map[string]interface{},
	Extra_variables map[string]string,
	Blacklist []string,
	Ignore_broken_items bool) {
	// TODO
}

func (cstate *CompleteState) filesToMetadata(subdir string) {
	// TODO
}

func load_files(isolated string) CompleteState {
	CHECK(filepath.IsAbs(isolated))
	isolatedBasedir := filepath.Dir(isolated)
	var members SavedStateMembers
	loadJsonFileVar(isolatedfile_to_state(isolated), &members)
	savedState := SavedState{members, isolatedBasedir, "", ""}
	return CompleteState{isolated, savedState}
}

// hacky helpers
func isolate_format_eval_variables(subdir string,
	translated_path_variables map[string]string) string {
	bytes, err := json.Marshal(translated_path_variables)
	NOERROR(err)
	v := execPyHelper("isolate_format_eval_variables", string(bytes), subdir)
	log.Printf("isolate_format_eval_variables subdir %s => %s", subdir, v)
	return v.(string)
}

func isolatedfile_to_state(isolated string) string {
	return isolated + ".state"
}
