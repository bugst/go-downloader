//
// Copyright 2025 Cristian Maglie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"go.bug.st/downloader/v3"
)

func main() {
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	d, err := downloader.Download(filepath.Join(tmp, "test.txt"), "https://go.bug.st/test.txt")
	if err != nil {
		log.Fatal(err)
	}

	progressCB := func(current int64) {
		fmt.Printf("Downloaded %d / %d bytes (%.2f%%)\n", current, d.Size(), float64(current)*100.0/float64(d.Size()))
	}
	if err := d.RunAndPoll(progressCB, time.Millisecond); err != nil {
		log.Fatal(err)
	}
}
