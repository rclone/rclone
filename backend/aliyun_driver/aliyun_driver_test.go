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
	config.FileSet(remoteName, "refresh-token", token)
}

func TestNewFs(t *testing.T) {
	i := 1
	fmt.Println(i)
	i = 2
	fmt.Println("11111111111")
	fmt.Println(i)
}

var (
	remoteName = "ali"
	token      = "a7816d7f853842bea263c699c54351aa"
	ctx        = context.Background()
)

func TestFs_List(t *testing.T) {
	f, err := fs.NewFs(ctx, remoteName+":")
	fmt.Println(err)
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

func Test_Open(t *testing.T) {
	f, _ := fs.NewFs(ctx, remoteName+":")
	o, _ := f.NewObject(ctx, "IMG_8351.livp")
	fmt.Println(o.Open(ctx))
}

func Test_INFO(t *testing.T) {
	f, _ := fs.NewFs(ctx, remoteName+":")
	fmt.Println(f)
	o, _ := f.NewObject(ctx, "IMG_0328.JPG")
	fmt.Println(o.(*Object).GetFileInfo(ctx))
}
