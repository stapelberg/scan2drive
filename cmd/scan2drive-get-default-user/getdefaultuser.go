package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/stapelberg/scan2drive/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	scan2driveAddress = flag.String("scan2drive_address",
		"localhost:7119",
		"host:port on which scan2drive is reachable")
)

func main() {
	log.Printf("flag.Parse\n")
	flag.Parse()

	log.Printf("grpc.Dial\n")
	conn, err := grpc.Dial(*scan2driveAddress, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("proto.NewScanClient\n")
	client := proto.NewScanClient(conn)
	log.Printf("client.DefaultUser\n")
	resp, err := client.DefaultUser(context.Background(), &proto.DefaultUserRequest{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.User)
}
