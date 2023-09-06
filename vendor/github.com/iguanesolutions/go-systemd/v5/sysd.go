package sysd

import "os"

// GetInvocationID returns the systemd invocation ID.
// If exists is false, we have not been launched by systemd.
// Present since systemd v232: https://github.com/systemd/systemd/blob/v232/NEWS#L254
func GetInvocationID() (ID string, exists bool) {
	return os.LookupEnv("INVOCATION_ID")
}
