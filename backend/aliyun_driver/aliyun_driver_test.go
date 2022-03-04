package aliyun_driver

import (
	"context"
	"fmt"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/rc"
)

func init() {
	// config.FileSet(remoteName, "type", "aliyun-driver")
	// config.FileSet(remoteName, "refresh-token", token)
}

func TestConfig(t *testing.T) {
	param := rc.Params{"refresh-token": "11"}

	opt := config.UpdateRemoteOpt{}
	config.UpdateRemote(context.Background(), "aliyun", param, opt)
}

func TestNewFs(t *testing.T) {

	p := rc.Params{
		"keyyy": "aaaaa",
	}

	config.UpdateRemote(context.Background(), remoteName, p, config.UpdateRemoteOpt{
		NoObscure: true,
	})
}

var (
	remoteName = "aliyun"
	token      = "a7816d7f853842bea263c699c54351aa1"
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
