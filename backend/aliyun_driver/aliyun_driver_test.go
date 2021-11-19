package aliyun_driver

import (
	"context"
	"fmt"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"testing"
)

func TestNewFs(t *testing.T) {

}

var remoteName = "ali"
var token = "bf7744a65ef0451886e78fbda6b94f96"

func TestFs_GetAccessToken(t *testing.T) {

	config.FileSet(remoteName, "type", "aliyun-driver")
	config.FileSet(remoteName, "refresh_token", token)

	f, err := fs.NewFs(context.Background(), remoteName+":")

	fmt.Println(f, err)
}
