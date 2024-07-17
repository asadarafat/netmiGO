package netmigo

import (
	"errors"
	"fmt"
	"io"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// SSHConnModel represents an SSH connection to a device.
type SSHConnModel struct {
	Addr     string
	Username string
	Password string
	Client   *ssh.Client
	Reader   io.Reader
	Writer   io.WriteCloser
	Timeout  uint8
}

// Supported ciphers for SSH connections.
var ciphers = []string{
	"aes256-ctr", "aes128-ctr", "aes128-cbc", "3des-cbc",
	"aes192-ctr", "aes192-cbc", "aes256-cbc", "aes128-gcm@openssh.com",
}

func NewSSHConnModel(hostname, username, password string, port uint8) (*SSHConnModel, error) {
	addr := fmt.Sprintf("%s:%d", hostname, port)
	return &SSHConnModel{
		Addr:     addr,
		Username: username,
		Password: password,
		Timeout:  6, // Default timeout is 6 seconds
	}, nil
}

func (c *SSHConnModel) SetTimeout(timeout uint8) {
	c.Timeout = timeout
}

// Connect establishes an SSH connection to the device.
func (c *SSHConnModel) Connect() error {
	interactive := getInteractiveCallBack(c.Password)
	sshConfig := &ssh.ClientConfig{
		User:            c.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(c.Password), ssh.KeyboardInteractive(interactive)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(c.Timeout) * time.Second,
	}
	sshConfig.Ciphers = append(sshConfig.Ciphers, ciphers...)

	conn, err := ssh.Dial("tcp", c.Addr, sshConfig)
	if err != nil {
		return errors.New("failed to connect to device: " + err.Error())
	}
	c.Client = conn

	session, err := c.Client.NewSession()
	if err != nil {
		return errors.New("failed to start a new session: " + err.Error())
	}

	reader, _ := session.StdoutPipe()
	writer, _ := session.StdinPipe()

	c.Reader = reader
	c.Writer = writer

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("vt100", 0, 200, modes); err != nil {
		return errors.New("failed to request Pty: " + err.Error())
	}
	if err := session.Shell(); err != nil {
		return errors.New("failed to invoke shell: " + err.Error())
	}
	return nil
}

// Disconnect closes the SSH connection.
func (c *SSHConnModel) Disconnect() {
	if err := c.Client.Close(); err != nil {
		log.Println("warning, device close failed: ", err)
	}
}

// Read reads data from the SSH connection.
func (c *SSHConnModel) Read() (string, error) {
	buff := make([]byte, 204800)
	n, err := c.Reader.Read(buff)
	return string(buff[:n]), err
}

// Write writes a command to the SSH connection.
func (c *SSHConnModel) Write(cmd string) int {
	code, _ := c.Writer.Write([]byte(cmd))
	return code
}

// getInteractiveCallBack returns a callback function for SSH keyboard-interactive authentication.
func getInteractiveCallBack(password string) ssh.KeyboardInteractiveChallenge {
	return func(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
		answers = make([]string, len(questions))
		for n := range questions {
			answers[n] = password
		}
		return answers, nil
	}
}

// InitTransport initializes a transport connection based on the protocol.
func InitTransport(host, username, password, protocol string, port uint8) (*SSHConnModel, error) {
	switch protocol {
	case "ssh":
		conn, err := NewSSHConnModel(host, username, password, port)
		if err != nil {
			return nil, err
		}
		return conn, nil
	default:
		return nil, errors.New("unsupported protocol: " + protocol)
	}
}
