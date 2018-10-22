//
// Copyright 2018 Cristian Maglie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package downloader

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func makeTmpFile(t *testing.T) string {
	tmp, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	require.NoError(t, tmp.Close())
	tmpFile := tmp.Name()
	require.NoError(t, os.Remove(tmpFile))
	return tmpFile
}

func TestDownload(t *testing.T) {
	tmpFile := makeTmpFile(t)
	defer os.Remove(tmpFile)

	d, err := Download(tmpFile, "https://go.bug.st/test.txt")
	require.NoError(t, err)
	require.Equal(t, int64(0), d.Completed())
	require.Equal(t, int64(8052), d.Size())
	require.NoError(t, d.Run())
	require.Equal(t, int64(8052), d.Completed())
	require.Equal(t, int64(8052), d.Size())

	file1, err := ioutil.ReadFile("testdata/test.txt")
	require.NoError(t, err)
	file2, err := ioutil.ReadFile(tmpFile)
	require.NoError(t, err)
	require.Equal(t, file1, file2)
}

func TestResume(t *testing.T) {
	tmpFile := makeTmpFile(t)
	defer os.Remove(tmpFile)

	part, err := ioutil.ReadFile("testdata/test.txt.part")
	require.NoError(t, err)
	err = ioutil.WriteFile(tmpFile, part, 0644)
	require.NoError(t, err)

	d, err := Download(tmpFile, "https://go.bug.st/test.txt")
	require.Equal(t, int64(3506), d.Completed())
	require.Equal(t, int64(8052), d.Size())
	require.NoError(t, err)
	require.NoError(t, d.Run())
	require.Equal(t, int64(8052), d.Completed())
	require.Equal(t, int64(8052), d.Size())

	file1, err := ioutil.ReadFile("testdata/test.txt")
	require.NoError(t, err)
	file2, err := ioutil.ReadFile(tmpFile)
	require.NoError(t, err)
	require.Equal(t, file1, file2)
}

func TestNoResume(t *testing.T) {
	tmpFile := makeTmpFile(t)
	defer os.Remove(tmpFile)

	part, err := ioutil.ReadFile("testdata/test.txt.part")
	require.NoError(t, err)
	err = ioutil.WriteFile(tmpFile, part, 0644)
	require.NoError(t, err)

	d, err := Download(tmpFile, "https://go.bug.st/test.txt", NoResume)
	require.Equal(t, int64(0), d.Completed())
	require.Equal(t, int64(8052), d.Size())
	require.NoError(t, err)
	require.NoError(t, d.Run())
	require.Equal(t, int64(8052), d.Completed())
	require.Equal(t, int64(8052), d.Size())

	file1, err := ioutil.ReadFile("testdata/test.txt")
	require.NoError(t, err)
	file2, err := ioutil.ReadFile(tmpFile)
	require.NoError(t, err)
	require.Equal(t, file1, file2)
}

func TestInvalidRequest(t *testing.T) {
	tmpFile := makeTmpFile(t)
	defer os.Remove(tmpFile)

	d, err := Download(tmpFile, "asd://go.bug.st/test.txt")
	require.Error(t, err)
	require.Nil(t, d)
	fmt.Println("ERROR:", err)

	d, err = Download(tmpFile, "://")
	require.Error(t, err)
	require.Nil(t, d)
	fmt.Println("ERROR:", err)
}

func TestRunAndPool(t *testing.T) {
	tmpFile := makeTmpFile(t)
	defer os.Remove(tmpFile)

	d, err := Download(tmpFile, "https://downloads.arduino.cc/cores/avr-1.6.20.tar.bz2")
	require.NoError(t, err)
	prevCurr := int64(0)
	callCount := 0
	callback := func(curr int64) {
		require.True(t, prevCurr <= curr)
		prevCurr = curr
		callCount++
	}
	d.RunAndPoll(callback, time.Millisecond)
	fmt.Printf("callback called %d times\n", callCount)
	require.True(t, callCount > 10)
	require.Equal(t, int64(4897949), d.Completed())
}

func TestErrorOnFileOpening(t *testing.T) {
	tmpFile := makeTmpFile(t)
	defer os.Remove(tmpFile)

	require.NoError(t, ioutil.WriteFile(tmpFile, []byte{}, 0000))
	d, err := Download(tmpFile, "http://go.bug.st/test.txt")
	require.Error(t, err)
	require.Nil(t, d)
}
