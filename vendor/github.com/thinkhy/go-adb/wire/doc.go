/*
Package wire implements the low-level part of the client/server wire protocol.
It also implements the "sync" wire format for file transfers.

This package is not intended to be used directly. adb.Adb and adb.Device
use it to abstract away the bit-twiddling details of the protocol. You should only ever
need to work with the goadb package. Also, this package's API may change more frequently
than goadb's.

The protocol spec can be found at
https://android.googlesource.com/platform/system/core/+/master/adb/OVERVIEW.TXT.
*/
package wire
