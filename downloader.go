//
// Copyright 2018-2025 Cristian Maglie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

func doHeadRequest(ctx context.Context, reqURL string, config Config) (*http.Response, error) {
	headReq, err := http.NewRequestWithContext(ctx, "HEAD", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("setting up HEAD request: %s", err)
	}
	if config.ExtraHeaders != nil {
		for k, v := range config.ExtraHeaders {
			headReq.Header.Set(k, v)
		}
	}
	// Should complete the HEAD call within the inactivity timeout
	config.HttpClient.Timeout = config.InactivityTimeout
	headResp, err := config.HttpClient.Do(headReq)
	if err != nil {
		return nil, fmt.Errorf("performing HEAD request: %s", err)
	}
	if _, err := io.Copy(io.Discard, headResp.Body); err != nil {
		return nil, err
	}
	if err := headResp.Body.Close(); err != nil {
		return nil, err
	}
	return headResp, nil
}

// Download downloads the specified url in the specified file.
// A download resume is tried if a file shorter than the requested url is already present.
func Download(ctx context.Context, file string, reqURL string) error {
	return DownloadWithConfig(ctx, file, reqURL, GetDefaultConfig())
}

// DownloadWithConfig applies an additional configuration to the http client and
// downloads download the specified url in the specified file.
// A previous download is resumed if the local file is shorter than the remote file,
// unless otherwise specified.
// The download is skipped if the local file has the same size of the remote file.
// The download is restarted from scratch if the local file is larger than the remote
// file or if the server doesn't support resuming.
func DownloadWithConfig(ctx context.Context, file string, reqURL string, config Config) error {
	clientCanResume := !config.DoNotResumeDownload

	// Gather information about local file
	var localSize int64
	if info, err := os.Stat(file); err == nil {
		localSize = info.Size()
	}

	// Perform a HEAD call to gather information about the server capabilities and remote file
	headResp, err := doHeadRequest(ctx, reqURL, config)
	if err != nil {
		return err
	}
	remoteSize := headResp.ContentLength // -1 if server doesn't send Content-Length
	serverCanResume := (headResp.Header.Get("Accept-Ranges") == "bytes") && (remoteSize != -1)

	// Perform acceptance checks
	var acceptError error
	if config.AcceptFunc != nil {
		acceptError = config.AcceptFunc(headResp)
	}
	if acceptError != nil {
		return acceptError
	}

	// If we are allowed to resume a download, check the local file size and decide how to proceed
	var completed int64
	if clientCanResume {
		if localSize == remoteSize {
			// Size matches: assume the file is already downloaded
			if config.PollFunction != nil {
				config.PollFunction(remoteSize, remoteSize)
			}
			return nil
		}
		if localSize < remoteSize {
			// Local file is smaller than remote file: resume download
			// Remote size is unknown: resume download anyway
			completed = localSize
		}
	}

	// Perform the actual GET request
	// Setup inactivity timeout for the GET request
	ctx, wdog := newWatchdog(ctx, config.InactivityTimeout)
	defer wdog.Cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return fmt.Errorf("setting up HTTP request: %s", err)
	}
	if config.ExtraHeaders != nil {
		for k, v := range config.ExtraHeaders {
			req.Header.Set(k, v)
		}
	}
	resumeDownload := clientCanResume && serverCanResume && completed > 0
	if resumeDownload {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", completed))
	}
	resp, err := config.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if !config.DoNotErrorOnNon2xxStatusCode {
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			return fmt.Errorf("server returned %s", resp.Status)
		}
	}

	// Open output file
	flags := os.O_WRONLY
	if resumeDownload {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_CREATE | os.O_TRUNC
	}
	out, err := os.OpenFile(file, flags, 0644)
	if err != nil {
		return fmt.Errorf("opening %s for writing: %s", file, err)
	}
	defer out.Close()

	var completedLock sync.Mutex
	if config.PollFunction != nil {
		update := func() {
			completedLock.Lock()
			_completed := completed
			completedLock.Unlock()
			config.PollFunction(_completed, remoteSize)
		}

		// send initial update
		update()

		var t *time.Timer
		t = time.AfterFunc(config.PollInterval, func() {
			// send intermediate updates
			update()
			t.Reset(config.PollInterval)
		})

		defer func() {
			t.Stop()
			// send final update
			update()
		}()
	}

	in := resp.Body
	buff := [4096]byte{}
	for {
		n, readErr := in.Read(buff[:])
		if n > 0 {
			if _, writeErr := out.Write(buff[:n]); writeErr != nil {
				// Error writing to file
				return writeErr
			}

			completedLock.Lock()
			completed += int64(n)
			completedLock.Unlock()

			// Extend inactivity timeout deadline
			wdog.Kick()
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}
