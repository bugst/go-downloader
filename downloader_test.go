//
// Copyright 2018-2025 Cristian Maglie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package downloader_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.bug.st/downloader/v3"
)

func makeTmpFile(t *testing.T) string {
	tmp, err := os.CreateTemp("", "")
	require.NoError(t, err)
	require.NoError(t, tmp.Close())
	tmpFile := tmp.Name()
	require.NoError(t, os.Remove(tmpFile))
	t.Cleanup(func() {
		os.Remove(tmpFile)
	})
	return tmpFile
}

func TestDownload(t *testing.T) {
	tmpFile := makeTmpFile(t)

	initialSize := int64(0)
	fullSize := int64(8052)
	finalSize := int64(0)
	count := 0
	err := downloader.DownloadWithConfig(t.Context(), tmpFile, "https://go.bug.st/test.txt", downloader.Config{
		PollInterval: time.Second,
		PollFunction: func(current, size int64) {
			if count == 0 {
				initialSize = current
			}
			fullSize = size
			finalSize = current
			count++
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), initialSize)
	require.Equal(t, int64(8052), fullSize)
	require.Equal(t, int64(8052), finalSize)

	file1, err := os.ReadFile("testdata/test.txt")
	require.NoError(t, err)
	file2, err := os.ReadFile(tmpFile)
	require.NoError(t, err)
	require.Equal(t, file1, file2)
}

func TestResume(t *testing.T) {
	tmpFile := makeTmpFile(t)

	part, err := os.ReadFile("testdata/test.txt.part")
	require.NoError(t, err)
	err = os.WriteFile(tmpFile, part, 0644)
	require.NoError(t, err)

	initialSize := int64(0)
	fullSize := int64(8052)
	finalSize := int64(0)
	count := 0
	err = downloader.DownloadWithConfig(t.Context(), tmpFile, "https://go.bug.st/test.txt", downloader.Config{
		PollInterval: time.Second,
		PollFunction: func(current, size int64) {
			if count == 0 {
				initialSize = current
			}
			fullSize = size
			finalSize = current
			count++
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(3506), initialSize)
	require.Equal(t, int64(8052), fullSize)
	require.Equal(t, int64(8052), finalSize)

	file1, err := os.ReadFile("testdata/test.txt")
	require.NoError(t, err)
	file2, err := os.ReadFile(tmpFile)
	require.NoError(t, err)
	require.Equal(t, file1, file2)
}

func TestResumeOnAlreadyCompletedFile(t *testing.T) {
	tmpFile := makeTmpFile(t)

	full, err := os.ReadFile("testdata/test.txt")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpFile, full, 0644))

	initialSize := int64(0)
	fullSize := int64(8052)
	finalSize := int64(0)
	count := 0
	err = downloader.DownloadWithConfig(t.Context(), tmpFile, "https://go.bug.st/test.txt", downloader.Config{
		PollInterval: time.Second,
		PollFunction: func(current, size int64) {
			if count == 0 {
				initialSize = current
			}
			fullSize = size
			finalSize = current
			count++
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(8052), initialSize)
	require.Equal(t, int64(8052), fullSize)
	require.Equal(t, int64(8052), finalSize)
	require.NoError(t, err)

	// Check file content is unchanged
	file2, err := os.ReadFile(tmpFile)
	require.NoError(t, err)
	require.Equal(t, full, file2)
}

func TestNoResume(t *testing.T) {
	tmpFile := makeTmpFile(t)

	part, err := os.ReadFile("testdata/test.txt.part")
	require.NoError(t, err)
	err = os.WriteFile(tmpFile, part, 0644)
	require.NoError(t, err)

	initialSize := int64(0)
	fullSize := int64(8052)
	finalSize := int64(0)
	count := 0
	err = downloader.DownloadWithConfig(t.Context(), tmpFile, "https://go.bug.st/test.txt", downloader.Config{
		DoNotResumeDownload: true,
		PollInterval:        time.Second,
		PollFunction: func(current, size int64) {
			if count == 0 {
				initialSize = current
			}
			fullSize = size
			finalSize = current
			count++
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), initialSize)
	require.Equal(t, int64(8052), fullSize)
	require.Equal(t, int64(8052), finalSize)

	file1, err := os.ReadFile("testdata/test.txt")
	require.NoError(t, err)
	file2, err := os.ReadFile(tmpFile)
	require.NoError(t, err)
	require.Equal(t, file1, file2)
}

func TestInvalidRequest(t *testing.T) {
	tmpFile := makeTmpFile(t)

	err := downloader.Download(t.Context(), tmpFile, "asd://go.bug.st/test.txt")
	require.Error(t, err)
	fmt.Println("ERROR:", err)
}

func TestRunAndPool(t *testing.T) {
	tmpFile := makeTmpFile(t)

	prevCurr := int64(0)
	callCount := 0
	callback := func(curr, size int64) {
		require.True(t, prevCurr <= curr)
		prevCurr = curr
		callCount++
	}

	config := downloader.GetDefaultConfig()
	config.PollFunction = callback
	config.PollInterval = time.Millisecond
	err := downloader.DownloadWithConfig(t.Context(), tmpFile, "https://downloads.arduino.cc/cores/avr-1.6.20.tar.bz2", config)
	require.NoError(t, err)
	fmt.Printf("callback called %d times\n", callCount)
	require.Greater(t, callCount, 10)
	require.Equal(t, int64(4897949), prevCurr)
}

func TestErrorOnFileOpening(t *testing.T) {
	unaccessibleFile := filepath.Join(os.TempDir(), "nonexistentdir", "test.txt")
	err := downloader.Download(t.Context(), unaccessibleFile, "http://go.bug.st/test.txt")
	require.Error(t, err)
}

// TestApplyUserAgentHeaderUsingConfig test uses the https://postman-echo.com/ service
func TestApplyUserAgentHeaderUsingConfig(t *testing.T) {
	tmpFile := makeTmpFile(t)

	err := downloader.DownloadWithConfig(t.Context(), tmpFile, "https://postman-echo.com/headers", downloader.Config{
		ExtraHeaders: map[string]string{
			"User-Agent": "go-downloader / 0.0.0-test",
		},
	})
	require.NoError(t, err)

	testEchoBody := struct {
		Headers map[string]string
	}{}
	body, err := os.ReadFile(tmpFile)
	require.NoError(t, err)
	err = json.Unmarshal(body, &testEchoBody)
	require.NoError(t, err)
	require.Equal(t, "go-downloader / 0.0.0-test", testEchoBody.Headers["user-agent"])
}

func TestContextCancelation(t *testing.T) {
	slowHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", "500")
			return
		}
		for i := range 50 {
			fmt.Fprintf(w, "Hello %02d.\n", i)
			w.(http.Flusher).Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/slow", slowHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	tmpFile := makeTmpFile(t)

	max := int64(0)
	startTime := time.Now()
	callback := func(curr, size int64) {
		fmt.Println("Downloaded", curr, "bytes of", size, ". Elapsed:", time.Since(startTime))
		max = curr
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel in two seconds
	go func() {
		time.Sleep(2 * time.Second)
		cancel()
	}()

	// Run slow download
	err := downloader.DownloadWithConfig(ctx, tmpFile, server.URL+"/slow", downloader.Config{
		PollInterval: 100 * time.Millisecond,
		PollFunction: callback,
	})
	require.EqualError(t, err, "context canceled")
	require.True(t, max < 210)
	fmt.Println("Context canceled successfully")
}

func TestTimeoutOnHEADCall(t *testing.T) {
	timeoutHandler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		fmt.Fprintln(w, "This is a slow response")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/timeout", timeoutHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	tmpFile := makeTmpFile(t)

	config := downloader.Config{
		InactivityTimeout: 1 * time.Second,
	}

	startTime := time.Now()
	err := downloader.DownloadWithConfig(t.Context(), tmpFile, server.URL+"/timeout", config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "context deadline exceeded")
	elapsed := time.Since(startTime)
	require.True(t, elapsed < 5*time.Second)
	fmt.Println("Download aborted due to timeout as expected after", elapsed)
}

func TestTimeoutOnGETCall(t *testing.T) {
	slowHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", "5000")
			return
		}
		time.Sleep(3 * time.Second) // Delay response sending any data
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", slowHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	tmpFile := makeTmpFile(t)

	config := downloader.Config{
		InactivityTimeout: 1 * time.Second,
	}

	startTime := time.Now()
	err := downloader.DownloadWithConfig(t.Context(), tmpFile, server.URL+"/slow", config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "i/o timeout")
	elapsed := time.Since(startTime)
	require.True(t, elapsed < 5*time.Second)
	fmt.Println("Download aborted due to timeout as expected after", elapsed)
}

func TestInactivityTimeout(t *testing.T) {
	// Create a handler that sends some data, then becomes inactive
	inactiveHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			// For HEAD requests, just return Content-Length
			w.Header().Set("Content-Length", "50")
			return
		}
		w.(http.Flusher).Flush() // Ensure headers are sent immediately

		// Send initial burst of data (500 ms)
		for range 5 {
			time.Sleep(100 * time.Millisecond)
			fmt.Fprintf(w, "AAAAA")
			w.(http.Flusher).Flush()
		}
		// Rest inactive for 1000 ms
		time.Sleep(1000 * time.Millisecond)
		// Send more data (500 ms)
		for range 5 {
			time.Sleep(100 * time.Millisecond)
			fmt.Fprintf(w, "AAAAA")
			w.(http.Flusher).Flush()
		}
		// 2000 ms total time
	}

	// Start test server
	mux := http.NewServeMux()
	mux.HandleFunc("/inactive", inactiveHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	// Request download with inactivity timeout of 500ms
	// (should timeout due to inactivity)
	tmpFile := makeTmpFile(t)
	startTime := time.Now()
	err := downloader.DownloadWithConfig(
		t.Context(), tmpFile, server.URL+"/inactive",
		downloader.Config{
			InactivityTimeout: 500 * time.Millisecond,
		},
	)
	elapsed := time.Since(startTime)

	// Check that we got a timeout error
	require.Error(t, err)
	require.Contains(t, err.Error(), "i/o timeout")

	// Check that it took around 1 second (initial burst of data 500ms + 500ms second timeout)
	require.InEpsilon(t, 1000*time.Millisecond, elapsed, 0.05, "Elapsed time should be around 1 second, but is %v", elapsed)
}
