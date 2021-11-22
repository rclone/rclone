package aliyun_driver

import (
	"context"
	"fmt"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
)

func TestNewFs(t *testing.T) {

}

var remoteName = "ali"
var token = "c5354c51ee3c43099b4b9ae9ec2f0129"

func TestFs_GetAccessToken(t *testing.T) {

	config.FileSet(remoteName, "type", "aliyun-driver")
	config.FileSet(remoteName, "refresh_token", token)

	ctx := context.Background()
	f, _ := fs.NewFs(ctx, remoteName+":")

	fmt.Println(f.List(ctx, "root"))
}
