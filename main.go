package main

import (
	"encoding/json"
	"fmt"
	"github.com/maruel/subcommands"
	"io/ioutil"
	"log"
	"os"
)

func main() {
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	retCode := subcommands.Run(application, nil)
	log.Printf("RETURN CODE = %d", retCode)
	os.Exit(retCode)
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
