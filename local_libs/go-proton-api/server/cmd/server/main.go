package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/rclone/go-proton-api"
	"github.com/rclone/go-proton-api/server"
	"github.com/rclone/go-proton-api/server/proto"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

func main() {
	app := cli.NewApp()

	app.Flags = []cli.Flag{
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Usage:   "port to serve gRPC on",
			Value:   8080,
		},
		&cli.BoolFlag{
			Name: "tls",
		},
	}

	app.Action = run

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	s := server.New(server.WithTLS(c.Bool("tls")))
	defer s.Close()

	return newService(s).run(c.Int("port"))
}

type service struct {
	proto.UnimplementedServerServer

	server *server.Server

	gRPCServer *grpc.Server
}

func newService(server *server.Server) *service {
	s := &service{
		server: server,

		gRPCServer: grpc.NewServer(),
	}

	proto.RegisterServerServer(s.gRPCServer, s)

	return s
}

func (s *service) GetInfo(ctx context.Context, req *proto.GetInfoRequest) (*proto.GetInfoResponse, error) {
	return &proto.GetInfoResponse{
		HostURL:  s.server.GetHostURL(),
		ProxyURL: s.server.GetProxyURL(),
	}, nil
}

func (s *service) CreateUser(ctx context.Context, req *proto.CreateUserRequest) (*proto.CreateUserResponse, error) {
	userID, addrID, err := s.server.CreateUser(req.Username, req.Password)
	if err != nil {
		return nil, err
	}

	return &proto.CreateUserResponse{
		UserID: userID,
		AddrID: addrID,
	}, nil
}

func (s *service) RevokeUser(ctx context.Context, req *proto.RevokeUserRequest) (*proto.RevokeUserResponse, error) {
	if err := s.server.RevokeUser(req.UserID); err != nil {
		return nil, err
	}

	return &proto.RevokeUserResponse{}, nil
}

func (s *service) CreateAddress(ctx context.Context, req *proto.CreateAddressRequest) (*proto.CreateAddressResponse, error) {
	addrID, err := s.server.CreateAddress(req.UserID, req.Email, req.Password)
	if err != nil {
		return nil, err
	}

	return &proto.CreateAddressResponse{
		AddrID: addrID,
	}, nil
}

func (s *service) RemoveAddress(ctx context.Context, req *proto.RemoveAddressRequest) (*proto.RemoveAddressResponse, error) {
	if err := s.server.RemoveAddress(req.UserID, req.AddrID); err != nil {
		return nil, err
	}

	return &proto.RemoveAddressResponse{}, nil
}

func (s *service) CreateLabel(ctx context.Context, req *proto.CreateLabelRequest) (*proto.CreateLabelResponse, error) {
	var labelType proton.LabelType

	switch req.Type {
	case proto.LabelType_FOLDER:
		labelType = proton.LabelTypeFolder

	case proto.LabelType_LABEL:
		labelType = proton.LabelTypeLabel
	}

	labelID, err := s.server.CreateLabel(req.UserID, req.Name, req.ParentID, labelType)
	if err != nil {
		return nil, err
	}

	return &proto.CreateLabelResponse{
		LabelID: labelID,
	}, nil
}

func (s *service) run(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}

	return s.gRPCServer.Serve(listener)
}
