// Command vulcan-guest is the guest agent that runs inside Firecracker microVMs.
// It listens on vsock for workload requests from the host, executes them using
// the appropriate runtime, and streams results back over vsock.
//
// Build with: CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o vulcan-guest ./cmd/vulcan-guest
package main

import (
	"log"

	"github.com/mdlayher/vsock"

	fc "github.com/seantiz/vulcan/internal/backend/firecracker"
	"github.com/seantiz/vulcan/internal/guest"
)

func main() {
	guest.SetupInit()

	port := fc.DefaultVsockPort
	l, err := vsock.Listen(port, nil)
	if err != nil {
		log.Fatalf("vsock listen on port %d: %v", port, err)
	}
	defer l.Close()

	log.Printf("vulcan-guest listening on vsock port %d", port)

	agent := guest.New(l, fc.GuestWorkDir)
	if err := agent.Serve(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
