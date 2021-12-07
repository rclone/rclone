package aliyun_driver

import (
	"context"
	"fmt"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
)

func init() {
	config.FileSet(remoteName, "type", "aliyun-driver")
	config.FileSet(remoteName, "refresh_token", token)
}

func TestNewFs(t *testing.T) {

}

var (
	remoteName = "ali"
	token      = "3b906b06c0104c1ebf32d74ae5647b19"
	ctx        = context.Background()
)

func TestFs_List(t *testing.T) {
	f, _ := fs.NewFs(ctx, remoteName+":")
	fmt.Println(f.List(ctx, ""))
}

func TestFs_Mkdir(t *testing.T) {
	f, _ := fs.NewFs(ctx, remoteName+":")

	fmt.Println(f.Mkdir(ctx, "aaaaaa11"))
}

func TestFs_Rmdir(t *testing.T) {
	f, _ := fs.NewFs(ctx, remoteName+":")

	fmt.Println(f.Rmdir(ctx, "619ca4f22700c18a76814a3a8be95a215793c473"))
}

func Test_About(t *testing.T) {
	f, _ := fs.NewFs(ctx, remoteName+":")
	fmt.Println(f.Features().About(ctx))
}

func Test_T(t *testing.T) {
	f, _ := fs.NewFs(ctx, remoteName+":")
	fmt.Println(f.(fs.Abouter))
}
