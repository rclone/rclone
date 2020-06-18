// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

// TODO: known issue:
//   this is incorrect since there's no good way to get such a path
//   since the exact previous key is
//     append(previousPrefix(cursor), infinite(0xFF)...)

// TODO commented until we will decide if we will support direction for objects listing
// func keyBefore(cursor string) string {
// 	if cursor == "" {
// 		return ""
// 	}

// 	before := []byte(cursor)
// 	if before[len(before)-1] == 0 {
// 		return string(before[:len(before)-1])
// 	}
// 	before[len(before)-1]--

// 	before = append(before, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f)
// 	return string(before)
// }

// func keyAfter(cursor string) string {
// 	return cursor + "\x00"
// }
