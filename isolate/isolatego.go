// Copyright 2015 The Swarming Authors. All rights reserved.
// Use of this source code is governed by the Apache v2.0 license that can be
// found in the LICENSE file.

package isolate

import (
	"log"
)

type GenJson struct {
	Args    []string
	Dir     string
	Version int
}

type Tree struct {
	cwd  string
	opts ArchiveOptions
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
