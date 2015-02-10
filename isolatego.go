package main

import (
	"encoding/json"
	"fmt"
	"github.com/maruel/subcommands"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
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
	path := "helper.py"
	fullArgs := append([]string{path, run}, args...)
	bytes := execCommand("python", input, fullArgs...)
	var v interface{}
	if err := json.Unmarshal(bytes, &v); err != nil {
		log.Printf("bad json [%v] from (%s)\n", err, bytes)
		panic(err)
	}
	return v
}

func execPyHelperTyped(v interface{}, run string, input string, args ...string) {
	path := "/s/swarming/client/helper.py"
	fullArgs := append([]string{path, run}, args...)
	bytes := execCommand("python", input, fullArgs...)
	if err := json.Unmarshal(bytes, &v); err != nil {
		log.Printf("bad json [%v] from (%s)\n", err, bytes)
		panic(err)
	}
}

var application = &subcommands.DefaultApplication{
	Name:  "isolatego",
	Title: "isolate.py but faster",
	// Commands will be shown in this exact order, so you'll likely want to put
	// them in alphabetical order or in logical grouping.
	Commands: []*subcommands.Command{
		cmdbatcharchive,
		subcommands.CmdHelp,
	},
}

var cmdbatcharchive = &subcommands.Command{
	UsageLine: "batcharchive file1 file2 ...",
	ShortDesc: "batcharchive",
	LongDesc:  "batcharchive",
	CommandRun: func() subcommands.CommandRun {
		c := &batcharchiveRun{}
		c.Flags.StringVar(&c.server, "-isolate-server", "https://isolateserver-dev.appspot.com/", "")
		c.Flags.StringVar(&c.namespace, "-namespace", "testing", "")
		c.Flags.StringVar(&c.dumpJson, "-dump-json", "debug_dump.json",
			"Write isolated Digestes of archived trees to this file as JSON")
		// TODO: blacklist
		return c
	},
}

type batcharchiveRun struct {
	subcommands.CommandRunBase
	server    string
	namespace string
	dumpJson  string
}

func (c *batcharchiveRun) Run(a subcommands.Application, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: at least one isolate file required.\n", a.GetName())
		return 1
	}
	if c.server == "" {
		fmt.Fprintf(os.Stderr, "%s: server must be specified.\n", a.GetName())
		return 1
	}
	if c.namespace == "" {
		fmt.Fprintf(os.Stderr, "%s: namespace must be specified.\n", a.GetName())
		return 1
	}
	var trees []Tree
	for _, genJsonPath := range args {
		if _, err := os.Stat(genJsonPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "no such file: %s", genJsonPath)
			return 1
		}
		jsonData, err := ioutil.ReadFile(genJsonPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read file: %s %s", genJsonPath, err)
			return 1
		}
		var data GenJson
		//TODO(tandrii): more robust parsing;
		if err := json.Unmarshal(jsonData, &data); err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse json file: %s %s", genJsonPath, err)
			return 1
		}
		if data.Version != ISOLATED_GEN_JSON_VERSION {
			fmt.Fprintf(os.Stderr, "Invalid version %d in %s", data.Version, genJsonPath)
			return 1
		}
		if isDir, err := IsDirectory(data.Dir); !isDir || err != nil {
			fmt.Fprintf(os.Stderr, "Invalid dir %s in %s", data.Dir, genJsonPath)
			return 1
		}
		trees = append(trees, Tree{data.Dir, parse_archive_command_line(data.Args, data.Dir)})
	}
	isolatedDigestes, err := isolate_and_archive(trees, c.server, c.namespace)
	if c.dumpJson != "" {
		var jsonData = []byte("{}")
		if isolatedDigestes != nil {
			jsonData, _ = json.MarshalIndent(isolatedDigestes, "", "  ")
		}
		_ = ioutil.WriteFile(c.dumpJson, jsonData, 0644)
	}
	// TODO(tandrii): error codes
	if err != nil {
		return 2
	}
	return 0
}

func main_isolate() int {
	return subcommands.Run(application, nil)
}

func IsDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	return fileInfo.IsDir(), err
}

type GenJson struct {
	Args    []string
	Dir     string
	Version int
}

type Tree struct {
	cwd  string
	opts ArchiveOptions
}

const ISOLATED_GEN_JSON_VERSION = 1

func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return v.(string)
}

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

func parse_archive_command_line(args []string, path string) ArchiveOptions {
	pathWithArgs := append([]string{path}, args...)
	var a ArchiveOptions
	execPyHelperTyped(&a, "parse_archive_command_line", "", pathWithArgs...)
	//log.Printf("parsed %s successfully", path)
	return a
	/*
		parser := flag.NewFlagSet("archive cmd", flag.PanicOnError)
		opts := ArchiveOptions{}
		parser.StringVar(&opts.isolate, "", ".isolate file to load the dependency data from")
		parser.BoolVar(&opts.ignoreBrokenItems, false, // TODO: take from ENV
		"Indicates that invalid entries in the isolated file to be only be logged and not stop processing. Defaults to True if env var ISOLATE_IGNORE_BROKEN_ITEMS is set")
	*/
}

func FileNameWithoutExtension(path string) string {
	fname := filepath.Base(path)
	return strings.TrimSuffix(fname, filepath.Ext(fname))
}

func flattenFC(chans [](chan FileAsset)) chan FileAsset {
	BUF_SIZE := 10 // TODO(tandrii): const OR determine best?
	out := make(chan FileAsset, BUF_SIZE)
	go func() {
		cases := make([]reflect.SelectCase, len(chans))
		for i, ch := range chans {
			cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
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

func isolate_and_archive(trees []Tree, namespace string, server string) (
	isolatedDigestes map[string]string, rerr error) {
	var fileAssets [](chan FileAsset)
	isolatedDigestes = make(map[string]string)
	good := 0
	for _, tree := range trees {
		targetName := FileNameWithoutExtension(tree.opts.Isolated)
		fc, isolatedDigest, err := prepare_for_archival(tree.opts, tree.cwd)
		if err != nil {
			log.Printf("failed isolating %s: %v", targetName, err)
			// continue anyway
		} else {
			good++
		}
		fileAssets = append(fileAssets, fc)
		isolatedDigestes[targetName] = isolatedDigest
		log.Printf("prepared %s => %s", targetName, isolatedDigest)
	}
	if good == 0 {
		// All bad, nothing to do
		return
	}
	log.Printf("uploading %d good isolated targets to %s : %s.", good, server, namespace)
	rerr = upload_tree(server, namespace, flattenFC(fileAssets))
	if rerr != nil {
		log.Printf("failed while uploading files %v", rerr)
	}
	return
}

func upload_tree(server string, namespace string, fileAssets chan FileAsset) error {
	items := make([]FileToUpload, 0)
	seen := make(map[string]bool)
	skipped := 0
	for fa := range fileAssets {
		if !fa.IsSymlink() && !seen[fa.fullPath] {
			seen[fa.fullPath] = true
			items = append(items, fa.ToUpload())
		} else {
			skipped++
		}
	}
	log.Printf("Skipped %d duplicated entries", skipped)
	s := Storage{GetStorageApi(server, namespace)}
	s.upload(items)
	return nil
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
	load_file_json(isolatedfile_to_state(isolated), &members)
	savedState := SavedState{members, isolatedBasedir, "", ""}
	return CompleteState{isolated, savedState}
}

func load_file_json(filepath string, v interface{}) {
	jsonData, err := ioutil.ReadFile(filepath)
	NOERROR(err)
	err = json.Unmarshal(jsonData, &v)
	NOERROR(err)
}

func SafeRelPath(basepath, targpath string) string {
	path, err := filepath.Rel(basepath, targpath)
	NOERROR(err)
	return path
}

//
// Types for storage
//

type UploadItem struct {
	Digest           string
	Size             int64
	HighPriority     bool
	CompressionLevel int
}

type FileToUpload struct {
	UploadItem
	Path string
}

func (fa *FileAsset) ToUpload() FileToUpload {
	// TODO: get_zip_compression_level
	return FileToUpload{
		UploadItem: UploadItem{
			Digest:           fa.meta["h"],
			Size:             fa.GetSize(),
			HighPriority:     fa.IsHighPriority(),
			CompressionLevel: 6,
		},
		Path: fa.fullPath,
	}
}

//
// STORAGE & Isolate Server API
//

type Storage struct {
	api StorageApier
}

func (storage *Storage) upload(items []FileToUpload) {
	// TODO
}

type StorageApier interface {
	// TODO api
}

func GetStorageApi(server, namespace string) StorageApier {
	// TODO(tandrii):
	return StorageApiLog{server, namespace}
}

type StorageApiLog struct {
	server, namespace string
	// TODO dummy API
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
