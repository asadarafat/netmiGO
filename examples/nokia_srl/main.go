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

	// if err := srl.FileTransfer("examples/nokia_srl/srl-router-config.json", "/home/admin/srl-router-config.json"); err != nil {
	// 	log.Error(err)
	// }

	output2, _ := srl.SendCommand("info flat", "running", 10*time.Second)
	log.Info(output2)
	fmt.Println(output2)

	output3, _ := srl.SendCommand("show version", "running", 10*time.Second)
	log.Info(output3)
	fmt.Println(output3)

	output4, _ := srl.SendCommand("save file running-config.json from running", "running", 10*time.Second)
	log.Info(output4)
	fmt.Println(output4)

	if err := srl.RetrieveFile("/home/admin/running-config.json", "examples/nokia_srl/running-config.json"); err != nil {
		log.Error(err)
	}

	if err := srl.FileTransfer("examples/nokia_srl/running-config.json", "/home/admin/running-config.json"); err != nil {
		log.Error(err)
	}

	output5, _ := srl.SendCommand("set system name host-name asad3", "candidate", 10*time.Second)
	log.Info(output5)
	fmt.Println(output5)

	output6, _ := srl.SendCommand("load file running-config.json auto-commit", "candidate", 10*time.Second)
	log.Info(output6)
	fmt.Println(output6)

}
