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

func TestFs_List(t *testing.T) {

	config.FileSet(remoteName, "type", "aliyun-driver")
	config.FileSet(remoteName, "refresh_token", token)

	ctx := context.Background()
	f, _ := fs.NewFs(ctx, remoteName+":")

	fmt.Println(f.List(ctx, "root"))
}

func TestFs_Mkdir(t *testing.T) {

	config.FileSet(remoteName, "type", "aliyun-driver")
	config.FileSet(remoteName, "refresh_token", "cea174e44c3f4ee8b519eef353868ac5")

	ctx := context.Background()
	f, _ := fs.NewFs(ctx, remoteName+":")

	fmt.Println(f.Mkdir(ctx, "aaaaaa11"))
}

func TestFs_Rmdir(t *testing.T) {

	config.FileSet(remoteName, "type", "aliyun-driver")
	config.FileSet(remoteName, "refresh_token", "cea174e44c3f4ee8b519eef353868ac5")

	ctx := context.Background()
	f, _ := fs.NewFs(ctx, remoteName+":")

	fmt.Println(f.Rmdir(ctx, "619ca4f22700c18a76814a3a8be95a215793c473"))
}
