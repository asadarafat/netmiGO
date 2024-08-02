package main

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	netmigo "github.com/asadarafat/netmiGO/netmigo"
)

func main() {

	srl, _ := netmigo.InitSRLDevice("10.7.7.11", "admin", "NokiaSrl1!", 22)
	srl.Connect()

	output3, _ := srl.SendCommand("show version", "running", 10*time.Second)
	log.Info(output3)

	output4, _ := srl.SendCommand("info", "running", 10*time.Second)
	fmt.Println(output4)

}
