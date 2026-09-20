//go:build !windows

package transaction

import "time"

func testOldTime() time.Time { return time.Unix(1, 0) }
