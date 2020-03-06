//
// Copyright 2018 Cristian Maglie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package downloader

import (
	"net/http"
	"sync"
)

// Config contains the configuration for the downloader
type Config struct {
	// RequestHeaders contains extra headers to add to the http request
	RequestHeaders http.Header

	// ProxyURL is the URL for a caching proxy to use to perform the request
	// or nil for no proxy
	ProxyURL string
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
