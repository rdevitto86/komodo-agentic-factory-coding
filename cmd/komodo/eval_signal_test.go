//go:build unix

package main

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

// TestEvalCasesInterruptCancelsAHungCloneInsteadOfHangingOrDying proves an interrupt during --cases cancels
// the case's context, so a hung clone is killed and the command returns instead of outliving the signal.
func TestEvalCasesInterruptCancelsAHungCloneInsteadOfHangingOrDying(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{}, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		accepted <- struct{}{}
		// The git fetch waits on a reply this server never sends, until its process is killed.
		_, _ = conn.Read(make([]byte, 1))
	}()
	url := fmt.Sprintf("http://%s/repo.git", listener.Addr().String())
	suite := evalSuite(t, url, 1)
	root := evalRoot(t)
	work := t.TempDir()

	oldExit := exit
	done := make(chan int, 1)
	exit = func(code int) { done <- code }
	t.Cleanup(func() { exit = oldExit })

	go runEval(root, []string{"--cases", "--suite", suite, "--work", work})

	select {
	case <-accepted:
	case <-time.After(10 * time.Second):
		t.Fatal("the clone never reached the hung git server")
	}

	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}

	select {
	case code := <-done:
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("an interrupt never stopped the hung clone")
	}
}
