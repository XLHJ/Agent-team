package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func freePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	p := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return p
}

func run(hubBin string) {
	port := freePort()
	token := "selftest3-token"
	dir, _ := os.MkdirTemp("", "acp-st3-")
	defer os.RemoveAll(dir)
	hub := exec.Command(hubBin,
		"-addr", "127.0.0.1:"+strconv.Itoa(port),
		"-token", token,
		"-state-dir", dir,
		"-schemas", "../../../protocol/schemas",
		"-signatures", "../../../harness-sdk/detect/signatures",
	)
	hub.Stdout = os.Stdout
	hub.Stderr = os.Stderr
	if err := hub.Start(); err != nil {
		fmt.Println("hub start:", err)
		os.Exit(2)
	}
	defer func() { _ = hub.Process.Kill() }()
	waitReady("http://127.0.0.1:" + strconv.Itoa(port) + "/health")

	p3 := &panelTest{base: "127.0.0.1:" + strconv.Itoa(port), token: token}
	ctx := context.Background()
	p3.checkTokenMissing(ctx)
	p3.checkOrigin(ctx)
	p3.checkSinglePanel(ctx)
	p3.checkMethods(ctx)
	p3.checkBusinessErrors(ctx)
	p3.checkEventPush(ctx)
}

func waitReady(url string) {
	for i := 0; i < 200; i++ {
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Println("hub never became ready")
	os.Exit(2)
}
