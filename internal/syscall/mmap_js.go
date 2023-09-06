// +build js wasi wasm

// Copyright 2020 Evan Shaw. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.


package syscall

import "syscall"

func mmap(fd, length int) ([]byte, error) {
	return nil, syscall.EINVAL
}
