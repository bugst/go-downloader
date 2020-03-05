//
// Copyright 2018 Cristian Maglie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package downloader

import (
	"net/http"
)

// Config contains the configuration for the downloader
type Config struct {
	// RequestHeaders contains extra headers to add to the http request
	RequestHeaders http.Header
}
