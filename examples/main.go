package main

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	netmigo "github.com/asadarafat/netmiGO/netmigo"
)

func main() {
	exampleBasicSROS()
}

func exampleBasicSROS() {

	Router10, err := netmigo.InitSROSDevice("10.2.1.109", "admin", "admin", 22)
	if err != nil {
		log.Error(err)
	}

	start := time.Now().UnixMilli()
	// Open session with Router10
	if err := Router10.Connect(); err != nil {
		log.Error(err)
	}
	// Capture the end time in milliseconds
	end := time.Now().UnixMilli()
	duration := end - start
	log.Infof("connect to router duration: %v milliseconds", duration)

	output2, _ := Router10.SendCommand("show uptime")
	// Transfer a file
	if err := Router10.FileTransfer("examples/clab-nokia-ServiceProvider-R09-PE-ASBR-running.cfg", "cf3:/clab-nokia-ServiceProvider-R09-PE-ASBR-running.cfg"); err != nil {
		log.Error(err)
	}

	// Send command
	// Capture the start time in milliseconds
	start = time.Now().UnixMilli()

	output1, _ := Router10.SendCommand("show port")

	// Capture the end time in milliseconds
	end = time.Now().UnixMilli()
	duration = end - start
	log.Infof("show port command execution duration: %v milliseconds", duration)

	// Send config command for classic CLI
	_, _ = Router10.SendCommand("show version")
	_, _ = Router10.SendCommand("admin save")

	// Send a set of config commands
	commands := []string{"show version", "load full-replace cf3:clab-nokia-ServiceProvider-R09-PE-ASBR-running.cfg"}
	output3, _ := Router10.SendConfigSet(commands)

	output4, _ := Router10.SendCommand("show uptime")

	// Capture the start time in milliseconds
	start = time.Now().UnixNano()
	Router10.Disconnect()
	// Capture the end time in milliseconds
	end = time.Now().UnixNano()
	duration = end - start
	log.Infof("disconnect from router duration: %v nanoseconds", duration)

	fmt.Println(output1)
	fmt.Println(output2)
	fmt.Println(output3)
	fmt.Println(output4)

}
