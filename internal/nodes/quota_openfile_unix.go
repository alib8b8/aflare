// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌​​‌​‌​‌‌​​‌‌​​‌‌​​‌‌​​​‌​‌​‌‌​​​​​‌​‌​‌‌‌‌​‌​​​​​​​​​​​​​​​​‌‌‌‌‌‌​‌‌​‌‌‌​‌‌⁠
// Unix-only: O_NOFOLLOW is available via syscall. The os package does not
// re-export it, so we pull it here on non-Windows builds.

//go:build !windows

package nodes

import "syscall"

// quotaOpenNoFollow refuses to open a path that is a symlink at the kernel
// level (defense-in-depth on top of the Lstat + validateNoSymlink checks).
const quotaOpenNoFollow = syscall.O_NOFOLLOW
