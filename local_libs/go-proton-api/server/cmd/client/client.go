package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/rclone/go-proton-api/server/proto"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	app := cli.NewApp()

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "host",
			Usage: "host to connect to",
			Value: "localhost",
		},
		&cli.IntFlag{
			Name:  "port",
			Usage: "port to connect to",
			Value: 8080,
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:   "info",
			Action: getInfoAction,
		},
		{
			Name: "auth",
			Subcommands: []*cli.Command{
				{
					Name:   "revoke",
					Action: revokeUserAction,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "userID",
							Usage:    "user ID to revoke",
							Required: true,
						},
					},
				},
			},
		},
		{
			Name: "user",
			Subcommands: []*cli.Command{
				{
					Name:   "create",
					Action: createUserAction,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "username",
							Usage:    "username of the account",
							Required: true,
						},
						&cli.StringFlag{
							Name:     "password",
							Usage:    "password of the account",
							Required: true,
						},
					},
				},
			},
		},
		{
			Name: "address",
			Subcommands: []*cli.Command{
				{
					Name:   "create",
					Action: createAddressAction,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "userID",
							Usage:    "ID of the user to create the address for",
							Required: true,
						},
						&cli.StringFlag{
							Name:     "email",
							Usage:    "email of the account",
							Required: true,
						},
						&cli.StringFlag{
							Name:     "password",
							Usage:    "password of the account",
							Required: true,
						},
					},
				},
				{
					Name:   "remove",
					Action: removeAddressAction,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "userID",
							Usage:    "ID of the user to remove the address from",
							Required: true,
						},
						&cli.StringFlag{
							Name:     "addressID",
							Usage:    "ID of the address to remove",
							Required: true,
						},
					},
				},
			},
		},
		{
			Name: "label",
			Subcommands: []*cli.Command{
				{
					Name:   "create",
					Action: createLabelAction,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "userID",
							Usage:    "ID of the user to create the label for",
							Required: true,
						},
						&cli.StringFlag{
							Name:     "name",
							Usage:    "name of the label",
							Required: true,
						},
						&cli.StringFlag{
							Name:  "parentID",
							Usage: "the ID of the parent label",
						},
						&cli.BoolFlag{
							Name:  "exclusive",
							Usage: "Create an exclusive label (i.e. a folder)",
						},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func getInfoAction(c *cli.Context) error {
	client, err := newServerClient(c)
	if err != nil {
		return err
	}

	res, err := client.GetInfo(c.Context, &proto.GetInfoRequest{})
	if err != nil {
		return err
	}

	return pretty(c.App.Writer, res)
}

func createUserAction(c *cli.Context) error {
	client, err := newServerClient(c)
	if err != nil {
		return err
	}

	res, err := client.CreateUser(c.Context, &proto.CreateUserRequest{
		Username: c.String("username"),
		Password: []byte(c.String("password")),
	})
	if err != nil {
		return err
	}

	return pretty(c.App.Writer, res)
}

func revokeUserAction(c *cli.Context) error {
	client, err := newServerClient(c)
	if err != nil {
		return err
	}

	res, err := client.RevokeUser(c.Context, &proto.RevokeUserRequest{
		UserID: c.String("userID"),
	})
	if err != nil {
		return err
	}

	return pretty(c.App.Writer, res)
}

func createAddressAction(c *cli.Context) error {
	client, err := newServerClient(c)
	if err != nil {
		return err
	}

	res, err := client.CreateAddress(c.Context, &proto.CreateAddressRequest{
		UserID:   c.String("userID"),
		Email:    c.String("email"),
		Password: []byte(c.String("password")),
	})
	if err != nil {
		return err
	}

	return pretty(c.App.Writer, res)
}

func removeAddressAction(c *cli.Context) error {
	client, err := newServerClient(c)
	if err != nil {
		return err
	}

	res, err := client.RemoveAddress(c.Context, &proto.RemoveAddressRequest{
		UserID: c.String("userID"),
		AddrID: c.String("addressID"),
	})
	if err != nil {
		return err
	}

	return pretty(c.App.Writer, res)
}

func createLabelAction(c *cli.Context) error {
	client, err := newServerClient(c)
	if err != nil {
		return err
	}

	var labelType proto.LabelType

	if c.Bool("exclusive") {
		labelType = proto.LabelType_FOLDER
	} else {
		labelType = proto.LabelType_LABEL
	}

	res, err := client.CreateLabel(c.Context, &proto.CreateLabelRequest{
		UserID: c.String("userID"),
		Name:   c.String("name"),
		Type:   labelType,
	})
	if err != nil {
		return err
	}

	return pretty(c.App.Writer, res)
}

func newServerClient(c *cli.Context) (proto.ServerClient, error) {
	cc, err := grpc.DialContext(
		c.Context,
		net.JoinHostPort(c.String("host"), fmt.Sprint(c.Int("port"))),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	return proto.NewServerClient(cc), nil
}

func pretty[T any](w io.Writer, v T) error {
	enc, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	if _, err := w.Write(enc); err != nil {
		return err
	}

	return nil
}
