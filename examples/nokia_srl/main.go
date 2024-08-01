package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	netmigo "github.com/asadarafat/netmiGO/netmigo"
	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	client *ssh.Client
	config *ssh.ClientConfig
	host   string
	port   string
}

func NewSSHClient(username, password, host, port string) *SSHClient {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return &SSHClient{config: config, host: host, port: port}
}

func (s *SSHClient) Connect() error {
	client, err := ssh.Dial("tcp", s.host+":"+s.port, s.config)
	if err != nil {
		return err
	}
	s.client = client
	return nil
}

func cleanOutput(output string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(output, "")
}

func (s *SSHClient) InteractiveSession(commands []string, prompt string) error {
	session, err := s.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
		return err
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}

	session.Stderr = os.Stderr

	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	if err := session.Shell(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			text := cleanOutput(scanner.Text())
			fmt.Println(text)
			if strings.Contains(text, prompt) {
				stdin.Write([]byte("quit\n"))
			}
		}
	}()

	for _, cmd := range commands {
		fmt.Fprintf(stdin, "%s\n", cmd)
		time.Sleep(2 * time.Second)
	}

	stdin.Close()
	return session.Wait()
}

func (s *SSHClient) Close() error {
	return s.client.Close()
}

func main() {

	srl, _ := netmigo.InitSRLDevice("10.7.7.11", "admin", "NokiaSrl1!", 22)
	srl.Connect()

	output3, _ := srl.SendCommand("show version", "running", 10*time.Second)
	log.Info(output3)

	output4, _ := srl.SendCommand("info", "running", 10*time.Second)
	fmt.Println(output4)

}
