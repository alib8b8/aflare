// Copyright (c) 2026 aflare Contributors
//
// Windows has no O_NOFOLLOW equivalent in syscall. We define the flag as 0
// here so the OpenFile call in router_quota.go compiles cross-platform.
// Defense-in-depth is still provided by the Lstat symlink check at finalPath
// and the validateNoSymlink walk, plus the random tmp suffix making a
// planted symlink at the tmp path near-impossible.

//go:build windows

package nodes

// quotaOpenNoFollow is 0 on Windows: there is no O_NOFOLLOW equivalent.
const quotaOpenNoFollow = 0
