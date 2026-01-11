//
// Copyright 2018-2025 Cristian Maglie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package main

import (
	"fmt"
	"log"
	"net/http"

	"go.bug.st/downloader/v3"
)

func main() {
	if err := downloader.DownloadWithConfig("test.txt", "https://go.bug.st/test.txt", downloader.Config{
		AcceptFunc: func(head *http.Response) error {
			if head.ContentLength > 2000 {
				return fmt.Errorf("insufficient space for download")
			}
			return nil
		},
	}); err != nil {
		log.Fatal(err)
	}

	panic("Unreachable, should exit on AcceptFunc")
}
