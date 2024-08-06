package main

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	netmigo "github.com/asadarafat/netmiGO/netmigo"
)

func main() {

	iosxr, _ := netmigo.InitIOSXRDevice("10.2.1.111", "clab", "clab@123", 22)
	iosxr.Connect()

	log.Infof("The Prompt is: %s", iosxr.Prompt)

	output1, _ := iosxr.SendCommand("show interfaces summary", "running", 10*time.Second)
	log.Info(output1)
	fmt.Println(output1)

	output2, _ := iosxr.CopyRunningConfig("config-asad-ganteng.cfg", "running", 10*time.Second)
	log.Info(output2)
	fmt.Println(output2)

	if err := iosxr.RetrieveFile("/misc/scratch/config-asad-ganteng.cfg", "./examples/cisco_iosxr/running-config.json"); err != nil {
		log.Error(err)
	}

	iosxr.Disconnect()

	iosxr.Connect()

	if err := iosxr.FileTransfer("./examples/cisco_iosxr/running-config.json", "/misc/scratch/config-asad-ganteng-sekali.cfg"); err != nil {
		log.Error(err)
	}

}
