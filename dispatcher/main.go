package main

import (
	"context"
	"flag"
	"math/rand"
	"net"
	"time"

	pb "github.com/minorhacks/bazel_remote_query/proto"

	"github.com/golang/glog"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	grpcPort = flag.String("grpc_port", "", "Port on which to start gRPC service")
)

type StaticDispatch struct {
}

func (d *StaticDispatch) GetQueryJob(ctx context.Context, req *pb.GetQueryJobRequest) (*pb.GetQueryJobResponse, error) {
	glog.Info("GetQueryJob() called")
	rand, err := uuid.NewRandom()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate UUID: %v", err)
	}
	return &pb.GetQueryJobResponse{
		Job: &pb.QueryJob{
			Id:    rand.String(),
			Query: "deps(//...)",
			Source: &pb.GitCommit{
				Repo:       "https://github.com/grpc/grpc",
				Committish: randomCommit(),
			},
		},
		NextPollTime: timestamppb.New(time.Now().Add(time.Minute)),
	}, nil
}

func (d *StaticDispatch) FinishQueryJob(ctx context.Context, req *pb.FinishQueryJobRequest) (*pb.FinishQueryJobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "FinishQueryJob not implemented")
}

func main() {
	flag.Parse()
	conn, err := net.Listen("tcp", net.JoinHostPort("", *grpcPort))
	exitIf(err)

	srv := grpc.NewServer()
	pb.RegisterQueryDispatcherServer(srv, &StaticDispatch{})

	glog.Infof("Listening on port %s", *grpcPort)
	srv.Serve(conn)
}

func exitIf(err error) {
	if err != nil {
		glog.Exit(err)
	}
}

func randomCommit() string {
	var hashes = []string{
		"59f714915973cef331764f7d1bee09a178955c33",
		"e7ddd7b436fde93f777f428b2cf8c0d16b98dc9a",
		"bea59115695b5c714b45e4cb92c56550988973da",
		"b9d904da5830218f53fd18a1a408c098667f0b97",
		"136055b043dcf2b15f69f535f659e4090cf25b9f",
		"050eb4343048ccfa81912496be47c4ef71bd9ea5",
		"371d231b538626eb4436ed050d62b3e9201173fa",
		"e9ad1d5f4742f972d6abba7deda8d7eebc5a5eac",
		"98fc0260e35028c6c49f00b28ce4a84659fa529e",
		"dd14f803c3beb433c41be91672c6a7a6eeb8727c",
		"55e870dae61ead220e0c74213410d90f8d21179f",
		"6273832210804ba1d443b28786e90478f93de29f",
		"0d3a9121f7a04e8400c65520cd3f7e076612e431",
		"387dbb92bd4a2b36e48554792c8d3a684722b58f",
		"7fb4998029bc10d0fc85de02570a711f83061f5d",
		"214e3f3622a62b3d05c8a0c2d78132ff4c31133f",
		"1a983ed013708a1338d39e3131f8d4eb1474eb70",
		"e9cf2894da6d3d09318c8a4a2892ddaf8f606726",
		"55b0405c86230c0d699c1c00aea2d83b5b5c59cd",
		"fc077c277163bf845b4808c5cae10ab48089a4c3",
		"25bade9be55ddfa105336bf979764a82acc8682a",
		"c33963d371715ae502455fb040cc7ac7554552f4",
		"5e989cf78d160fbe7279abd3168eda0627fa0ba2",
		"5a3cd992b0b5fa164eb2fce1c0a3d00c62672e28",
		"14cdbc68af7e7be0b054d704514d03d09d72793d",
		"99752b173cfa2fba81dedb482ee4fd74b2a46bb0",
		"61987ec3a21e7d40101335d657e71a15e72a20a5",
		"9a12b0def8871b4d588f870628ed9a1ab1db256b",
		"1b52d7b8af05b85e0de7e65d4825233938367d22",
		"714c78bd051772786ae051116290f24a3465628c",
		"47aa3c23ac1f5db0ae5d1744c8d5431ee1d3d53f",
		"da1977f98c089762197a8b86387b87a38208e729",
		"3052919efa278f2204a883abcfecee219de987d1",
		"83e8f5f0bab7c1c75f8bc95042cab8d5c9515bc7",
		"d61433ecb2639f850ae4e92d842e5166ca946705",
		"3a4058c07af52a53b0388e77b15205f7ab51e4cb",
		"94c538cd55b9cbdd66f072a426c9d3ad2474554b",
		"63bd1ba8112e489f35ab28f2fcedb0a8ee510c44",
		"c64d6f3f57a73ad33261a4991e2ac8a97330d495",
		"07a75427bd1447535708a3e33f26d31bca18074b",
		"5850cba2957cc894477e735a74aa6c246b499ff4",
		"88a706eaacccedc73aa7c4bb4182636609a19673",
		"d76397480c1bc62b7982eb1b60b8e8b5ffdd048f",
		"9f5735b604d78e1568e7676c771d4f07f425300d",
		"61b34dfaee80d36b505fbb488cc73b2afd803c2e",
		"997aae5cc4e0e8f6ab1207392c396635884cdd99",
		"58de9394aacb7b47ae2cf27cd6bc5e038d1f641b",
		"60c56f7d018ff3aaae77611897edc0dc3bbc013a",
		"05af494b282542304c9fa60d19e8aa1b9f474621",
		"71b355624f5a8edc6c9a01fd11b40f38a09c3659",
		"0b799404966751fa91558863e11dc87cbd70f50f",
		"e53ea89bca138f03d8282aaa624b1f255587b75f",
		"696e4928e6579325f3e203dec642a885d44cd59d",
		"6d6380de587a174b890a99145d854766f14e1ae2",
		"2c6ab431a1e73a6f839717471c67a0842514b389",
		"519ae7d7952b853ddf43d4c623eb898b6495bfa3",
		"1fd3850502f9f681b0bc2b1e503a125d66d46efc",
		"869ed910d5cf26ca24544202ef3f4a4cfa1ad4ea",
		"96c19e8c9890edd125aa542c8ae628d13f1e29c5",
		"817420318aeb534e96d1de7facfc4173ff1659bc",
		"311648e532b55a924a334844a22993860a1b6323",
		"f3897a5f7ab666f015af2ce05b56eed10243f804",
		"ab2193d8c178c41d2b097dc8b35a54bfdd9e928b",
		"e145c068f27adc885292d45e5bba1ee13cc13566",
		"06e2fcc6c7c5ab13a8bf086291d5b8779e4d8f5c",
		"5c35dbed931afeec4ad6f9c9e0b0392e0e20febb",
		"c0e2fe9ea040b6e1e901b540eb29f318d9c1c541",
		"9796969b0c1dabe60353a9a1c523fc13a81316d7",
		"1a76218baaa834892dadaabf7d5df429e89490ba",
		"caacc7e3aadd816430461b6657ff66ea3c8d70f3",
		"33aaa5503278978fe0a327e79f070ee1a378fc5c",
		"b03601fa25d1b7f0db519691846ed3a04b421c2f",
		"2223877d417ca1a6fd06db7dc585bd1d35e540a0",
		"1973b4f9b16443bec6788f85c482c88d99564b1a",
		"a6419dde0691c07793b05397c3c72f44046ed365",
		"217ab2a7938d2afad47ba08573832ccfce497f21",
		"7ea775f60709b6fe509c5c709e09c0e25fb03d7a",
		"b9ca51ef52e2286e9f088bbfbfdfe0d94c03df4a",
		"ee84fb9beb9fd02e055a3cf239a3e1a3614442eb",
		"ec1aa1f26c9384f5f5372ed0fe5218ef2c6760cd",
		"91cf96c6eea3e06adbaae5d0def3359f798ff058",
		"f688f5fcf27674b70c6d80bec92e73d2dccf25e9",
		"2fd632a4c1d22abc218cfb0e04f896a1d6081ec1",
		"f40fe16aab643f89ade3b0e61f098bffa6c77090",
		"17f3880ee15a19ddafd37a1f542d4beac3d7f117",
		"58612ba1555ba5ebe5a62be0087be33fbd8b426b",
		"3d84874dcd30f263b71c1e84a4ff1b7b34b4798b",
		"3cb64325976bc5d57717e9b2b975692930603397",
		"4b8ea48c35015951e6d628edd37e7ce9b339c0a7",
		"14c078bcdc3a6969dc05362848ddf658ef0debcd",
		"683b6061449db4110fe533917f77d3de04dce1ec",
		"69f87d2d27854d169573fab0accbf87779e0094d",
		"276dc89fd9021fbf24e02b7fc4fe93318da1b3b0",
		"08451810a673fa892f08e4d00b2cc59cc6b80f90",
		"b35605d1da2923cb82624286feabf132cedd83ec",
		"775362a2cea21363339d73215e3b9a1394ad55b2",
		"f11d86c403e45ac4d13662f71d79512cd7e26bde",
		"ee2b75e33740d1a88c0e2aeec1b14435e17a889e",
		"18a8f6aad95d5c2f73f70c3a7fbc58a429da9d56",
		"114e83d875859f36883cc10ffa089eb534aecaff",
	}
	return hashes[rand.Intn(len(hashes))]
}
