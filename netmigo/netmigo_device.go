package netmigo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/pkg/sftp"
)

// DeviceConnection represents a device driver with connection and command capabilities.
type DeviceConnection struct {
	Connection *SSHConnModel
	Return     string
}

func (d *DeviceConnection) Connect() error {
	return d.Connection.Connect()
}

func (d *DeviceConnection) Disconnect() {
	d.Connection.Disconnect()
}

func (d *DeviceConnection) SetTimeout(timeout uint8) {
	d.Connection.SetTimeout(timeout)
}

func (d *DeviceConnection) SendCommand(cmd string) (string, error) {
	return d.SendCommandPattern(cmd, d.Return)
}

func (d *DeviceConnection) SendConfigSet(cmds []string) (string, error) {
	results, _ := d.SendCommandPattern("configure exclusive", d.Return)
	cmds = append(cmds, "commit", "exit")
	out, err := d.SendCommandsSetPattern(cmds, d.Return)
	results += out
	return results, err
}

func (d *DeviceConnection) FindDevicePrompt(regex string, pattern string) (string, error) {
	var out string
	var err error
	r, _ := regexp.Compile(regex)

	if pattern != "" {
		out, err = d.ReadUntil(pattern)
		if err != nil {
			return "", err
		}
	} else {
		out, _ = d.Connection.Read()
	}
	if !r.MatchString(out) {
		return "", errors.New("failed to find prompt, pattern: " + pattern + " , output: " + out)
	}
	return r.FindStringSubmatch(out)[1], nil
}

func (d *DeviceConnection) ReadUntil(pattern string) (string, error) {
	outputChan := make(chan string)
	var err error

	go func() {
		buffChan := make(chan string)
		go readRoutine(d, pattern, buffChan)
		select {
		case recv := <-buffChan:
			outputChan <- recv
		case <-time.After(4 * time.Second):
			err = errors.New("timeout while reading, read pattern not found pattern: " + pattern)
			close(outputChan)
		}
	}()

	return <-outputChan, err
}

func (d *DeviceConnection) SendCommandPattern(cmd string, expectPattern string) (string, error) {
	if d.Connection == nil {
		return "", errors.New("not connected to device, make sure to call .Connect() first")
	}

	cmd += d.Return
	d.Connection.Write(cmd)
	return d.ReadUntil(expectPattern)
}

func (d *DeviceConnection) SendCommandsSetPattern(cmds []string, expectPattern string) (string, error) {
	if d.Connection == nil {
		return "", errors.New("not connected to device, make sure to call .Connect() first")
	}
	var results string
	for _, cmd := range cmds {
		out, _ := d.SendCommandPattern(cmd, expectPattern)
		results += out
	}
	return results, nil
}

func readRoutine(d *DeviceConnection, pattern string, buffChan chan<- string) {
	var result string
	result, err := d.Connection.Read()
	r, _ := regexp.Compile(pattern)
	for err == nil && !r.MatchString(result) {
		outSlice, _ := d.Connection.Read()
		result += outSlice
	}
	buffChan <- result
}

// FileTransfer transfers a file to the remote device using SFTP.
func (d *DeviceConnection) FileTransfer(localFile, remoteFile string) error {
	// Establish SFTP session
	sftpClient, err := d.NewSFTPClient()
	if err != nil {
		return fmt.Errorf("failed to establish SFTP session: %v", err)
	}
	defer sftpClient.Close()

	// Open the local file
	localFileReader, err := os.Open(localFile)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer localFileReader.Close()

	// Create the remote file
	remoteFileWriter, err := sftpClient.Create(remoteFile)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %v", err)
	}
	defer remoteFileWriter.Close()

	// Copy the file
	if _, err := io.Copy(remoteFileWriter, localFileReader); err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	return nil
}

// NewSFTPClient creates a new SFTP client using the existing SSH connection.
func (d *DeviceConnection) NewSFTPClient() (*sftp.Client, error) {
	sftpClient, err := sftp.NewClient(d.Connection.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to create SFTP client: %v", err)
	}
	return sftpClient, nil
}
