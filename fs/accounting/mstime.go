package accounting

import (
	"strconv"
	"strings"
	"time"
)

// msTime is helper type for representing unix epoch timestamp in milliseconds.
type msTime time.Time

func (t msTime) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(time.Time(t).UnixNano()/1e6, 10)), nil
}

func (t *msTime) UnmarshalJSON(s []byte) (err error) {
	r := strings.Replace(string(s), `"`, ``, -1)

	q, err := strconv.ParseInt(r, 10, 64)
	if err != nil {
		return err
	}
	*(*time.Time)(t) = time.Unix(q/1000, (q-q/1000)*1000)
	return
}

func (t msTime) String() string { return time.Time(t).String() }
