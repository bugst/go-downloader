//
// Copyright 2018 Cristian Maglie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package downloader

import (
	"net/http"
	"sync"
	"time"
)

// Config contains the configuration for the downloader
type Config struct {
	// HttpClient to use to perform HTTP requests
	HttpClient http.Client
	// DoNotResumeDownload set to true to disallow resuming downloads.
	DoNotResumeDownload bool
	// ExtraHeaders to add to the HTTP requests.
	ExtraHeaders map[string]string
	// AcceptFunc is an optional function that will be called
	// when the HTTP HEAD request is done, before starting the download.
	// If the function returns an error, the download is aborted.
	AcceptFunc func(head *http.Response) error
	// DoNotErrorOnNon2xxStatusCode set to true to not return an error
	// if the server returns a non-2xx status code.
	DoNotErrorOnNon2xxStatusCode bool
	// InactivityTimeout is the duration after which, if no data is received,
	// the download is aborted. If set to 0, no timeout is applied.
	InactivityTimeout time.Duration
}

var defaultConfig Config = Config{}
var defaultConfigLock sync.Mutex

// SetDefaultConfig sets the configuration that will be used by the Download
// function.
func SetDefaultConfig(newConfig Config) {
	defaultConfigLock.Lock()
	defer defaultConfigLock.Unlock()
	defaultConfig = newConfig
}

// GetDefaultConfig returns a copy of the default configuration. The default
// configuration can be changed using the SetDefaultConfig function.
func GetDefaultConfig() Config {
	defaultConfigLock.Lock()
	defer defaultConfigLock.Unlock()

	// deep copy struct
	return defaultConfig
}
