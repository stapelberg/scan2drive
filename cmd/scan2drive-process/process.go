package main

import (
	"flag"
	"log"

	"github.com/stapelberg/scan2drive/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	scan2driveAddress = flag.String("scan2drive_address",
		"localhost:7119",
		"host:port on which scan2drive is reachable")
	user = flag.String("user",
		"",
		"User under which to perform the processing. See scan2drive-get-default-user(1)")
)

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatal("Syntax: scan2drive-process <dir>")
	}

	conn, err := grpc.Dial(*scan2driveAddress, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatal(err)
	}
	client := proto.NewScanClient(conn)
	if _, err := client.ProcessScan(context.Background(), &proto.ProcessScanRequest{User: *user, Dir: flag.Arg(0)}); err != nil {
		log.Fatal(err)
	}
}
