package monotime

import "time"

var initTime = time.Now()

func Now() time.Time { return initTime.Add(elapsed()) }
