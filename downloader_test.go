//
// Copyright 2018 Cristian Maglie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package downloader_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	d, err := downloader.Download(tmpFile, "https://go.bug.st/test.txt")
	require.NoError(t, err)
	require.Equal(t, int64(0), d.Completed())
	require.Equal(t, int64(8052), d.Size())
	require.NoError(t, d.Run())
	require.Equal(t, int64(8052), d.Completed())
	require.Equal(t, int64(8052), d.Size())

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

	d, err := downloader.Download(tmpFile, "https://go.bug.st/test.txt")
	require.Equal(t, int64(3506), d.Completed())
	require.Equal(t, int64(8052), d.Size())
	require.NoError(t, err)
	require.NoError(t, d.Run())
	require.Equal(t, int64(8052), d.Completed())
	require.Equal(t, int64(8052), d.Size())

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

	d, err := downloader.Download(tmpFile, "https://go.bug.st/test.txt")
	require.NoError(t, err)
	require.Equal(t, int64(8052), d.Completed())
	require.Equal(t, int64(8052), d.Size())
	require.NoError(t, d.Run())
	require.Equal(t, int64(8052), d.Completed())

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

	d, err := downloader.DownloadWithConfig(tmpFile, "https://go.bug.st/test.txt", downloader.Config{
		DoNotResumeDownload: true,
	})
	require.Equal(t, int64(0), d.Completed())
	require.Equal(t, int64(8052), d.Size())
	require.NoError(t, err)
	require.NoError(t, d.Run())
	require.Equal(t, int64(8052), d.Completed())
	require.Equal(t, int64(8052), d.Size())

	file1, err := os.ReadFile("testdata/test.txt")
	require.NoError(t, err)
	file2, err := os.ReadFile(tmpFile)
	require.NoError(t, err)
	require.Equal(t, file1, file2)
}

func TestInvalidRequest(t *testing.T) {
	tmpFile := makeTmpFile(t)

	d, err := downloader.Download(tmpFile, "asd://go.bug.st/test.txt")
	require.Error(t, err)
	require.Nil(t, d)
	fmt.Println("ERROR:", err)

	d, err = downloader.Download(tmpFile, "://")
	require.Error(t, err)
	require.Nil(t, d)
	fmt.Println("ERROR:", err)
}

func TestRunAndPool(t *testing.T) {
	tmpFile := makeTmpFile(t)

	d, err := downloader.Download(tmpFile, "https://downloads.arduino.cc/cores/avr-1.6.20.tar.bz2")
	require.NoError(t, err)
	prevCurr := int64(0)
	callCount := 0
	callback := func(curr int64) {
		require.True(t, prevCurr <= curr)
		prevCurr = curr
		callCount++
	}
	require.NoError(t, d.RunAndPoll(callback, time.Millisecond))
	fmt.Printf("callback called %d times\n", callCount)
	require.Greater(t, callCount, 10)
	require.Equal(t, int64(4897949), d.Completed())
}

func TestErrorOnFileOpening(t *testing.T) {
	unaccessibleFile := filepath.Join(os.TempDir(), "nonexistentdir", "test.txt")
	d, err := downloader.Download(unaccessibleFile, "http://go.bug.st/test.txt")
	require.Error(t, err)
	require.Nil(t, d)
}

type roundTripper struct {
	UserAgent string
	transport http.Transport
}

func (r *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header["User-Agent"] = []string{r.UserAgent}
	return r.transport.RoundTrip(req)
}

// TestApplyUserAgentHeaderUsingConfig test uses the https://postman-echo.com/ service
func TestApplyUserAgentHeaderUsingConfig(t *testing.T) {
	type echoBody struct {
		Headers map[string]string
	}

	tmpFile := makeTmpFile(t)
	defer os.Remove(tmpFile)

	httpClient := http.Client{
		Transport: &roundTripper{UserAgent: "go-downloader / 0.0.0-test"},
	}
	config := downloader.Config{
		HttpClient: httpClient,
	}

	d, err := downloader.DownloadWithConfig(tmpFile, "https://postman-echo.com/headers", config)
	require.NoError(t, err)

	testEchoBody := echoBody{}
	body, err := io.ReadAll(d.Resp.Body)
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

	ctx, cancel := context.WithCancel(context.Background())
	d, err := downloader.DownloadWithConfigAndContext(ctx, tmpFile, server.URL+"/slow", downloader.Config{})
	require.NoError(t, err)

	// Cancel in two seconds
	startTime := time.Now()
	go func() {
		time.Sleep(2 * time.Second)
		cancel()
	}()

	// Run slow download
	max := int64(0)
	err = d.RunAndPoll(func(curr int64) {
		fmt.Println("Downloaded", curr, "bytes. Elapsed:", time.Since(startTime))
		max = curr
	}, 100*time.Millisecond)
	require.EqualError(t, err, "context canceled")
	require.True(t, max < 210)
	fmt.Println("Context canceled successfully")
}
