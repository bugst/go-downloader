//
// Copyright 2018 Cristian Maglie. All rights reserved.
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

// Downloader is an asynchronous downloader
type Downloader struct {
	URL           string
	Done          chan struct{}
	Resp          *http.Response
	out           *os.File
	completed     int64
	completedLock sync.Mutex
	size          int64
	err           error
}

// Close the download
func (d *Downloader) Close() error {
	err1 := d.out.Close()
	err2 := d.Resp.Body.Close()
	if err1 != nil {
		return fmt.Errorf("closing output file: %s", err1)
	}
	if err2 != nil {
		return fmt.Errorf("closing input stream: %s", err2)
	}
	return nil
}

// Size return the size of the download (or -1 if the server doesn't provide it)
func (d *Downloader) Size() int64 {
	return d.size
}

// RunAndPoll starts the downloader copy-loop and calls the poll function every
// interval time to update progress.
func (d *Downloader) RunAndPoll(poll func(current int64), interval time.Duration) error {
	t := time.NewTicker(interval)
	defer t.Stop()

	go d.Run()
	for {
		select {
		case <-t.C:
			poll(d.Completed())
		case <-d.Done:
			poll(d.Completed())
			return d.Error()
		}
	}
}

// Run starts the downloader and waits until it completes the download.
// This method can be run in a goroutine to perform an asynchronous download;
// it will close the Done channel when the download is completed or an error occurs.
func (d *Downloader) Run() error {
	defer close(d.Done)

	d.completedLock.Lock()
	skip := (d.completed == d.size)
	d.completedLock.Unlock()
	if skip {
		return d.Error()
	}

	in := d.Resp.Body
	buff := [4096]byte{}
	for {
		n, err := in.Read(buff[:])
		if n > 0 {
			_, _ = d.out.Write(buff[:n])
			d.completedLock.Lock()
			d.completed += int64(n)
			d.completedLock.Unlock()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			d.err = err
			break
		}
	}
	_ = d.Close()
	return d.Error()
}

// Error returns the error during download or nil if no errors happened
func (d *Downloader) Error() error {
	return d.err
}

// Completed returns the bytes read so far
func (d *Downloader) Completed() int64 {
	d.completedLock.Lock()
	res := d.completed
	d.completedLock.Unlock()
	return res
}

// Download returns an asynchronous downloader that will download the specified url
// in the specified file. A download resume is tried if a file shorter than the requested
// url is already present.
func Download(file string, reqURL string) (*Downloader, error) {
	return DownloadWithConfig(file, reqURL, GetDefaultConfig())
}

// DownloadWithConfig applies an additional configuration to the http client and
// returns an asynchronous downloader that will download the specified url
// in the specified file. A download resume is tried if a file shorter than the requested
// url is already present.
func DownloadWithConfig(file string, reqURL string, config Config) (*Downloader, error) {
	return DownloadWithConfigAndContext(context.Background(), file, reqURL, config)
}

// DownloadWithConfigAndContext applies an additional configuration to the http client and
// returns an asynchronous downloader that will download the specified url
// in the specified file.
// A previous download is resumed if the local file is shorter than the remote file.
// The download is skipped if the local file has the same size of the remote file.
// The download is restarted from scratch if the local file is larger than the remote file.
func DownloadWithConfigAndContext(ctx context.Context, file string, reqURL string, config Config) (*Downloader, error) {
	clientCanResume := !config.DoNotResumeDownload

	// Gather information about local file
	var localSize int64
	if info, err := os.Stat(file); err == nil {
		localSize = info.Size()
	}

	// Perform a HEAD call to gather information about the server capabilities and remote file
	headReq, err := http.NewRequestWithContext(ctx, "HEAD", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("setting up HEAD request: %s", err)
	}
	if config.ExtraHeaders != nil {
		for k, v := range config.ExtraHeaders {
			headReq.Header.Set(k, v)
		}
	}
	headResp, err := config.HttpClient.Do(headReq)
	if err != nil {
		return nil, fmt.Errorf("performing HEAD request: %s", err)
	}
	remoteSize := headResp.ContentLength // -1 if server doesn't send Content-Length
	serverCanResume := (headResp.Header.Get("Accept-Ranges") == "bytes") && (remoteSize != -1)
	var acceptError error
	if config.AcceptFunc != nil {
		acceptError = config.AcceptFunc(headResp)
	}
	_, _ = io.Copy(io.Discard, headResp.Body)
	_ = headResp.Body.Close()
	if acceptError != nil {
		return nil, acceptError
	}

	// If we are allowed to resume a download, check the local file size and decide how to proceed
	var completed int64
	if clientCanResume {
		if localSize == remoteSize {
			// Size matches: assume the file is already downloaded
			return &Downloader{
				URL:       reqURL,
				Done:      make(chan struct{}),
				Resp:      headResp,
				completed: remoteSize,
				size:      remoteSize,
			}, nil
		}
		if localSize < remoteSize {
			// Local file is smaller than remote file: resume download
			// Remote size is unknown: resume download anyway
			completed = localSize
		}
	}

	// Perform the actual GET request
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("setting up HTTP request: %s", err)
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
		return nil, err
	}

	// Open output file
	flags := os.O_WRONLY
	if resumeDownload {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(file, flags, 0644)
	if err != nil {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("opening %s for writing: %s", file, err)
	}

	return &Downloader{
		URL:       reqURL,
		Done:      make(chan struct{}),
		Resp:      resp,
		out:       f,
		completed: completed,
		size:      remoteSize,
	}, nil
}
