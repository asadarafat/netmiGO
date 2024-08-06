package main

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	netmigo "github.com/asadarafat/netmiGO/netmigo"
)

func main() {

	junos, _ := netmigo.InitJUNOSDevice("10.2.1.22", "admin", "admin@123", 22)
	junos.Connect()

	log.Infof("The Prompt is: %s", junos.Prompt)

	output1, _ := junos.SendCommand("show configuration | save /var/home/admin/config-asad-ganteng.cfg", "running", 10*time.Second)
	log.Info(output1)
	fmt.Println(output1)

	output2, _ := junos.SendCommand("set system services ssh sftp-server", "candidate", 10*time.Second)
	log.Info(output2)
	fmt.Println(output2)

	if err := junos.RetrieveFile("/var/home/admin/config-asad-ganteng.cfg", "examples/juniper_junos/running-config.json"); err != nil {
		log.Error(err)
	}

	if err := junos.RetrieveFileUsingSCP("/var/home/admin/config-asad-ganteng.cfg", "examples/juniper_junos/running-config.json"); err != nil {
		log.Error(err)
	}

	if err := junos.FileTransfer("examples/juniper_junos/running-config.json", "/var/home/admin/running-config.cfg"); err != nil {
		log.Error(err)
	}

	output3, _ := junos.SendCommand("load replace /var/home/admin/running-config.cfg", "candidate", 10*time.Second)
	log.Info(output3)

}
