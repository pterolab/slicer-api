## slicer-api

A small Go service that exposes HTTP and gRPC endpoints to use a slicer CLI (for example [OrcaSlicer](https://github.com/SoftFever/OrcaSlicer)) to slice a 3MF file over the network.

This repo implements two ways for slicing:

- A minimal gRPC server with bi-directional streaming (see `proto/slicing.proto`)
- An HTTP endpoint with streaming

## Features

- Slice a 3MF file using the profiles in the 3MF into a `.gcode.3mf` file using [OrcaSlicer](https://github.com/SoftFever/OrcaSlicer)
- Run a gRPC or HTTP server to handle slicing requests

## Requirements

- Go (to build/run the server)
- OrcaSlicer (works on macOS and Linux)

## Configuration

The service is configured via environment variables:

- `SLICER_APP` - Path to the slicer binary to execute
- `SERVER_MODE` (optional) - Set to `'grpc'` or `'http'` to choose the server type. Defaults to `'http'` if unset
- `PORT` - Port number for the server to listen on

## Run locally

First, configure the environment variables in the `.env` file. You can use the provided `.env.example` as a template. Then run it:

```bash
go run .
```

## Usage

### HTTP

Submit a 3MF via stream to the HTTP API as `POST /slice`. The response will be a streamed 3MF file that includes the G-code. If an error occurs, it will return it as `text/plain`.

### gRPC usage

The gRPC API is defined in `proto/slicing.proto`. The server implements a streaming `Slicer/Slice` RPC. You can stream a 3MF file to it, and it will return the sliced 3MF file that includes the G-code.

## Security

This project implements only limited security measures and assumes it will run behind another service or reverse proxy. It will create a temporary directory at the runtime location and create the needed files by itself. It doesn't check the file type and will always save as 3MF with fixed names.

Keep in mind for running in public:

- There is no authentication/authorization implemented by default.
- The slicer can't be run in a container (at least in my tests). It is best to run it in a VM with limited access.

## Roadmap

Planned improvements and ideas:

- Add more slicing options
- Add Bambu Studio and PrusaSlicer slicers
- Harden security
- Add tests and build process
- Add setup script

Contributions and feedback are welcome.
