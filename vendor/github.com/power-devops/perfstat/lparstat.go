// +build aix

package perfstat

/*
#cgo LDFLAGS: -lperfstat

#include <libperfstat.h>
*/
import "C"

import (
	"fmt"
)

func PartitionStat() (*PartitionConfig, error) {
	var part C.perfstat_partition_config_t

	rc := C.perfstat_partition_config(nil, &part, C.sizeof_perfstat_partition_config_t, 1)
	if rc != 1 {
		return nil, fmt.Errorf("perfstat_partition_config() error")
	}
	p := perfstatpartitionconfig2partitionconfig(part)
	return &p, nil

}
