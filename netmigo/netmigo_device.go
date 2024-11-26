package netmigo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	scp "github.com/bramvdbogaerde/go-scp"
	"github.com/bramvdbogaerde/go-scp/auth"
	"github.com/pkg/sftp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// DeviceConnection represents a device driver with connection and command capabilities.
type DeviceConnection struct {
	Connection *SSHConnModel
	Return     string
}

func (d *DeviceConnection) Connect() error {
	err := d.Connection.Connect()
	if err != nil {
		log.Errorf("Failed to connect: %v", err)
		return err
	}
	log.Info("Connected successfully")
	return nil
}

func (d *DeviceConnection) ConnectXterm() error {
	err := d.Connection.ConnectXterm()
	if err != nil {
		log.Errorf("Failed to connect via Xterm: %v", err)
		return err
	}
	log.Info("Connected via Xterm successfully")
	return nil
}

func (d *DeviceConnection) Disconnect() {
	if d.Connection != nil {
		d.Connection.Disconnect()
		log.Info("Disconnected successfully")
	} else {
		log.Warn("Disconnect called on a nil connection")
	}
}

func (d *DeviceConnection) SetTimeout(timeout uint8) {
	if d.Connection != nil {
		d.Connection.SetTimeout(timeout)
		log.Infof("Timeout set to %d seconds", timeout)
	} else {
		log.Warn("SetTimeout called on a nil connection")
	}
}

func (d *DeviceConnection) SendCommand(cmd string) (string, error) {
	return d.SendCommandPattern(cmd, d.Return)
}

func (d *DeviceConnection) FindDevicePrompt(regex string, pattern string) (string, error) {
	// Compile the regular expression and check for errors
	r, err := regexp.Compile(regex)
	if err != nil {
		log.Errorf("Failed to compile regex '%s': %v", regex, err)
		return "", fmt.Errorf("failed to compile regex: %v", err)
	}

	// Read until the specified pattern or read the available output
	var out string
	if pattern != "" {
		out, err = d.ReadUntil(pattern)
		if err != nil {
			log.Errorf("Failed to read until pattern '%s': %v", pattern, err)
			return "", err
		}
	} else {
		out, err = d.Connection.Read()
		if err != nil {
			log.Errorf("Failed to read from connection: %v", err)
			return "", err
		}
	}

	// Match the prompt using the regular expression
	if !r.MatchString(out) {
		log.Errorf("Failed to find prompt, pattern: '%s', output: '%s'", pattern, out)
		return "", errors.New("failed to find prompt, pattern: " + pattern + " , output: " + out)
	}

	// Find and return the matched prompt
	matches := r.FindStringSubmatch(out)
	if len(matches) > 0 {
		return matches[0], nil
	}

	log.Warnf("Prompt not found in output: '%s'", out)
	return "", errors.New("prompt not found in output")
}

func (d *DeviceConnection) ReadUntil(pattern string) (string, error) {
	outputChan := make(chan string)
	errorChan := make(chan error)

	go func() {
		buffChan := make(chan string)
		go readRoutine(d, pattern, buffChan)
		select {
		case recv := <-buffChan:
			outputChan <- recv
		case <-time.After(4 * time.Second):
			err := fmt.Errorf("timeout while reading, pattern not found: %s", pattern)
			log.Error(err)
			errorChan <- err
		}
		close(outputChan)
		close(errorChan)
	}()

	select {
	case result := <-outputChan:
		return result, nil
	case err := <-errorChan:
		return "", err
	}
}

func (d *DeviceConnection) SendCommandPattern(cmd string, expectPattern string) (string, error) {
	if d.Connection == nil {
		err := errors.New("not connected to device, make sure to call .Connect() first")
		log.Error(err)
		return "", err
	}

	cmd += d.Return
	d.Connection.Write(cmd)

	return d.ReadUntil(expectPattern)
}

func (d *DeviceConnection) SendCommandsSetPattern(cmds []string, expectPattern string) (string, error) {
	if d.Connection == nil {
		err := errors.New("not connected to device, make sure to call .Connect() first")
		log.Error(err)
		return "", err
	}
	var results string
	for _, cmd := range cmds {
		out, err := d.SendCommandPattern(cmd, expectPattern)
		if err != nil {
			log.Errorf("Error sending command '%s': %v", cmd, err)
			return "", err
		}
		results += out
	}
	return results, nil
}

func readRoutine(d *DeviceConnection, pattern string, buffChan chan<- string) {
	var result string
	for {
		outSlice, err := d.Connection.Read()
		if err != nil {
			log.Errorf("Error reading from connection: %v", err)
			break
		}

		cleanString := func(str string) string {
			re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
			return re.ReplaceAllString(str, "")
		}

		outSlice = cleanString(outSlice)
		result += outSlice

		r, err := regexp.Compile(pattern)
		if err != nil {
			log.Errorf("Failed to compile regex pattern '%s': %v", pattern, err)
			break
		}

		if r.MatchString(result) {
			break
		}
	}
	buffChan <- result
}

// NewSFTPClient creates a new SFTP client using the existing SSH connection.
func (d *DeviceConnection) NewSFTPClient() (*sftp.Client, error) {
	if d.Connection == nil || d.Connection.Client == nil {
		err := errors.New("SSH connection is not established")
		log.Error(err)
		return nil, err
	}

	sftpClient, err := sftp.NewClient(d.Connection.Client)
	if err != nil {
		log.Errorf("Failed to create SFTP client: %v", err)
		return nil, fmt.Errorf("failed to create SFTP client: %v", err)
	}
	return sftpClient, nil
}

// RetrieveFile downloads a file from the remote device using SFTP.
func (d *DeviceConnection) RetrieveFile(remoteFile, localFile string) error {
	// Establish SFTP session
	sftpClient, err := d.NewSFTPClient()
	if err != nil {
		log.Infof("Failed to establish SFTP session. Fallback to SFTP with io.ReadAll method..")
		return d.RetrieveFileReadAll(remoteFile, localFile)
	}
	defer sftpClient.Close()

	// Open the remote file
	remoteFileReader, err := sftpClient.Open(remoteFile)
	if err != nil {
		log.Errorf("Failed to open remote file '%s': %v", remoteFile, err)
		return fmt.Errorf("failed to open remote file: %v", err)
	}
	defer remoteFileReader.Close()

	// Create the local file
	localFileWriter, err := os.Create(localFile)
	if err != nil {
		log.Infof("Failed to create local file '%s': %v", localFile, err)
		log.Infof("Fallback to SFTP with io.ReadAll method..")
		return d.RetrieveFileReadAll(remoteFile, localFile)
	}
	defer localFileWriter.Close()

	// Copy the file from the remote device to the local machine
	if _, err := io.Copy(localFileWriter, remoteFileReader); err != nil {
		log.Errorf("Failed to copy file from '%s' to '%s': %v", remoteFile, localFile, err)
		log.Infof("Fallback to SFTP with io.ReadAll method..")
		return d.RetrieveFileReadAll(remoteFile, localFile)
	}

	log.Infof("File retrieved successfully from '%s' to '%s'", remoteFile, localFile)
	return err
}

// RetrieveFile downloads a file from the remote device using SFTP ReadAll.
func (d *DeviceConnection) RetrieveFileReadAll(remoteFile, localFile string) error {
	// Establish SFTP session
	sftpClient, err := d.NewSFTPClient()
	if err != nil {
		log.Infof("Failed to establish SFTP session. Fallback to SCP..")
		return d.RetrieveFileUsingSCP(remoteFile, localFile)
	}
	defer sftpClient.Close()

	// Open the remote file
	remoteFileReader, err := sftpClient.Open(remoteFile)
	if err != nil {
		log.Errorf("Failed to open remote file '%s': %v", remoteFile, err)
		return fmt.Errorf("failed to open remote file: %v", err)
	}
	defer remoteFileReader.Close()

	// io.ReadAll
	data, err := io.ReadAll(remoteFileReader)
	if err != nil {
		log.Errorf("Failed to read remote file: %v", err)
	}

	// Create the local file
	err = os.WriteFile(localFile, data, 0644)
	if err != nil {
		log.Errorf("Failed to write to local file: %v", err)
	}
	log.Infof("File retrieved successfully using SFTP ReadAll method from '%s' to '%s'", remoteFile, localFile)
	return nil
}

// RetrieveFileUsingSCP downloads a file from the remote device using SCP.
func (d *DeviceConnection) RetrieveFileUsingSCP(remoteFile, localFile string) error {
	// Create SSH client configuration
	sshConfig, err := auth.PasswordKey(d.Connection.Username, d.Connection.Password, ssh.InsecureIgnoreHostKey())
	if err != nil {
		log.Errorf("Failed to create SSH config: %v", err)
		return fmt.Errorf("failed to create SSH config: %v", err)
	}

	// Create SCP client
	client := scp.NewClient(d.Connection.Addr, &sshConfig)

	// Connect to the remote server
	err = client.Connect()
	if err != nil {
		log.Errorf("Failed to connect via SCP: %v", err)
		return fmt.Errorf("failed to connect via SCP: %v", err)
	}
	defer client.Close()

	// Open the local file for writing
	localFileWriter, err := os.Create(localFile)
	if err != nil {
		log.Errorf("Failed to create local file '%s': %v", localFile, err)
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer localFileWriter.Close()

	// Copy the remote file to the local file
	err = client.CopyFromRemote(context.Background(), localFileWriter, remoteFile)
	if err != nil {
		log.Errorf("Failed to copy file via SCP from '%s' to '%s': %v", remoteFile, localFile, err)
		return fmt.Errorf("failed to copy file via SCP: %v", err)
	}

	log.Infof("File retrieved successfully via SCP from '%s' to '%s'", remoteFile, localFile)
	return nil
}

// FileTransfer transfers a file to the remote device using SFTP.
func (d *DeviceConnection) FileTransfer(localFile, remoteFile string) error {
	// Establish SFTP session
	sftpClient, err := d.NewSFTPClient()
	if err != nil {
		log.Infof("Failed to establish SFTP session. Fallback to SCP..")
		return d.FileTransferUsingSCP(localFile, remoteFile)
	}
	defer sftpClient.Close()

	// Open the local file
	localFileReader, err := os.Open(localFile)
	if err != nil {
		log.Errorf("Failed to open local file '%s': %v", localFile, err)
		log.Infof("Fallback to SCP method..")
		return d.FileTransferUsingSCP(remoteFile, localFile)
	}
	defer localFileReader.Close()

	// Create the remote file
	remoteFileWriter, err := sftpClient.Create(remoteFile)
	if err != nil {
		log.Errorf("Failed to create remote file '%s': %v", remoteFile, err)
		log.Infof("Fallback to SCP..")
		return d.FileTransferUsingSCP(remoteFile, localFile)
	}
	defer remoteFileWriter.Close()

	// Copy the file
	if _, err := io.Copy(remoteFileWriter, localFileReader); err != nil {
		log.Errorf("Failed to copy file from '%s' to '%s': %v", localFile, remoteFile, err)
		log.Infof("Fallback to SCP..")
		return d.FileTransferUsingSCP(remoteFile, localFile)
	}

	log.Infof("File transferred successfully using SFTP from '%s' to '%s'", localFile, remoteFile)
	return err
}

// FileTransfer transfers a file to the remote device using SFTP ReadAll.
func (d *DeviceConnection) FileTransferReadAll(localFile, remoteFile string) error {
	// Establish SFTP session
	sftpClient, err := d.NewSFTPClient()

	log.Info("sftpClient create succesfully")

	if err != nil {
		log.Infof("Failed to establish SFTP session. Fallback to SCP..")
		return d.FileTransferUsingSCP(localFile, remoteFile)
	}
	defer sftpClient.Close()

	// Open the local file
	localFileReader, err := os.Open(localFile)
	if err != nil {
		log.Errorf("Failed to open local file '%s': %v", localFile, err)
		log.Infof("Fallback to SCP method..")
		return d.FileTransferUsingSCP(localFile, remoteFile)
	}
	defer localFileReader.Close()

	// Read the entire content of the local file
	localFileContent, err := io.ReadAll(localFileReader)
	if err != nil {
		log.Errorf("Failed to read local file '%s': %v", localFile, err)
		log.Infof("Fallback to SCP method..")
		return d.FileTransferUsingSCP(localFile, remoteFile)
	}

	// Create the remote file
	remoteFileWriter, err := sftpClient.Create(remoteFile)
	if err != nil {
		log.Errorf("Failed to create remote file '%s': %v", remoteFile, err)
		log.Infof("Fallback to SCP..")
		return d.FileTransferUsingSCP(localFile, remoteFile)
	}
	defer remoteFileWriter.Close()

	// Write the file content to the remote file
	if _, err := remoteFileWriter.Write(localFileContent); err != nil {
		log.Errorf("Failed to write to remote file '%s': %v", remoteFile, err)
		log.Infof("Fallback to SCP..")
		return d.FileTransferUsingSCP(localFile, remoteFile)
	}

	log.Infof("File transferred successfully using SFTP from '%s' to '%s'", localFile, remoteFile)
	return err
}

// TransferFileUsingSCP uploads a file to the remote device using SCP.
func (d *DeviceConnection) FileTransferUsingSCP(localFile, remoteFile string) error {
	// Create SSH client configuration
	sshConfig, err := auth.PasswordKey(d.Connection.Username, d.Connection.Password, ssh.InsecureIgnoreHostKey())
	if err != nil {
		log.Errorf("Failed to create SSH config: %v", err)
		return fmt.Errorf("failed to create SSH config: %v", err)
	}

	// Create SCP client
	client := scp.NewClient(d.Connection.Addr, &sshConfig)

	// Connect to the remote server
	err = client.Connect()
	if err != nil {
		log.Errorf("Failed to connect via SCP: %v", err)
		return fmt.Errorf("failed to connect via SCP: %v", err)
	}
	defer client.Close()

	// Open the local file for reading
	localFileReader, err := os.Open(localFile)
	if err != nil {
		log.Errorf("Failed to open local file '%s': %v", localFile, err)
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer localFileReader.Close()

	// Copy the local file to the remote file
	err = client.CopyFromFile(context.Background(), *localFileReader, remoteFile, "0655")
	if err != nil {
		log.Errorf("Failed to copy file via SCP from '%s' to '%s': %v", localFile, remoteFile, err)
		return fmt.Errorf("failed to copy file via SCP: %v", err)
	}

	log.Infof("File transferred successfully via SCP from '%s' to '%s'", localFile, remoteFile)
	return nil
}
