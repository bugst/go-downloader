//
// Copyright 2018 Cristian Maglie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package downloader

import (
	"net/http"
)

// Downloader is an asynchronous downloader
type Config struct {
	RequestHeaders               http.Header
}
