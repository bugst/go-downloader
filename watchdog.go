//
// Copyright 2018-2025 Cristian Maglie. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package downloader

import (
	"context"
	"os"
	"time"
)

type watchdog struct {
	ctx     context.Context
	cancel  context.CancelCauseFunc
	timer   *time.Timer
	timeout time.Duration
}

func newWatchdog(parent context.Context, timeout time.Duration) (context.Context, watchdog) {
	ctx, cancel := context.WithCancelCause(parent)
	var timer *time.Timer
	if timeout > 0 {
		timer = time.AfterFunc(timeout, func() {
			// Cancel the context with a clear, standard error.
			cancel(os.ErrDeadlineExceeded)
		})
	}
	return ctx, watchdog{
		ctx:     ctx,
		cancel:  cancel,
		timer:   timer,
		timeout: timeout,
	}
}

func (wd *watchdog) Kick() {
	if wd.timeout > 0 {
		wd.timer.Reset(wd.timeout)
	}
}

func (wd *watchdog) Cancel() {
	if wd.timeout > 0 {
		wd.timer.Stop()
	}
	wd.cancel(nil)
}
