package hdfs

import "os"

// ReadDir reads the directory named by dirname and returns a list of sorted
// directory entries.
//
// The os.FileInfo values returned will not have block location attached to
// the struct returned by Sys().
func (c *Client) ReadDir(dirname string) ([]os.FileInfo, error) {
	f, err := c.Open(dirname)
	if err != nil {
		return nil, err
	}

	return f.Readdir(0)
}
