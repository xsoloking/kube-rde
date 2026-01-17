package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
)

func main() {
	l, err := net.Listen("tcp", ":3000")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Mock Service listening on :3000")

	// 只接受一次连接用于测试
	conn, err := l.Accept()
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		fmt.Printf("🎉 MOCK RECEIVED: %s\n", scanner.Text())
	}
	_ = conn.Close() //nolint:errcheck // cleanup error not critical
}
