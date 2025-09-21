package servers

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"

	slicer_grpc "slicer-api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"slicer-api/slicer"
)

type server struct {
	slicer_grpc.UnimplementedSlicerServer
}

func (s *server) Slice(stream slicer_grpc.Slicer_SliceServer) error {
	var in bytes.Buffer
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "recv error: %v", err)
		}
		if data := req.GetFileData(); len(data) > 0 {
			if _, werr := in.Write(data); werr != nil {
				return status.Errorf(codes.Internal, "buffer write error: %v", werr)
			}
		}
	}

	if in.Len() == 0 {
		return status.Error(codes.InvalidArgument, "no input data received")
	}

	app := os.Getenv("SLICER_APP")
	if app == "" {
		return status.Error(codes.FailedPrecondition, "SLICER_APP is not set")
	}

	sliceResponse, err := slicer.Slice(in.Bytes(), app)
	if err != nil {
		return status.Errorf(codes.Internal, "slicing failed: %v", err)
	}

	file, err := os.Open(sliceResponse.OutputPath)
	if err != nil {
		return status.Errorf(codes.Internal, "open output file error: %v", err)
	}
	defer file.Close()
	defer os.RemoveAll(filepath.Dir(sliceResponse.OutputPath))

	const bufferSize = 1024 * 1024
	buffer := make([]byte, bufferSize)
	for {
		i, readErr := file.Read(buffer)
		if i > 0 {
			if err := stream.Send(&slicer_grpc.SliceResponse{
				SlicedData: buffer[:i],
				Status:     "Success",
			}); err != nil {
				return status.Errorf(codes.Unavailable, "send error: %v", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return status.Errorf(codes.Internal, "read error: %v", readErr)
		}
	}

	return nil
}

func RunGRPC(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	grpcServer := grpc.NewServer()
	slicer_grpc.RegisterSlicerServer(grpcServer, &server{})
	log.Printf("gRPC server listening on %s", lis.Addr().String())
	return grpcServer.Serve(lis)
}
