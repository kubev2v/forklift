// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gob

// tooBig provides a sanity check for sizes; used in several places. Upper limit
// of is 1GB on 32-bit systems, 8GB on 64-bit, allowing room to grow a little
// without overflow.
const tooBig = (1 << 30) << (^uint(0) >> 62)
