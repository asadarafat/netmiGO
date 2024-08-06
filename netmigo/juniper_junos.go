package netmigo

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// JUNOSDeviceConnection represents a specific device type that uses a driver to connect and send commands.
type JUNOSDeviceConnection struct {
	DeviceConnection
	DeviceType string
	Prompt     string
}

func NewJUNOSDeviceConnection(connection *SSHConnModel, DeviceType string) (*JUNOSDeviceConnection, error) {
	return &JUNOSDeviceConnection{
		DeviceConnection: DeviceConnection{
			Connection: connection,
			Return:     "\n",
		},
		DeviceType: DeviceType,
	}, nil
}

// NewDevice initializes a new JUNOS device connection
func InitJUNOSDevice(Host string, Username string, Password string, Port uint8) (*JUNOSDeviceConnection, error) {

	// Create connection
	connection, err := InitTransport(Host, Username, Password, "ssh", Port)
	if err != nil {
		return nil, err
	}

	// Create the device
	return NewJUNOSDeviceConnection(connection, "juniper_junos")
}

func (junos *JUNOSDeviceConnection) Connect() error {
	if err := junos.DeviceConnection.ConnectXterm(); err != nil {
		return err
	}

	// Define the regex pattern to find the JUNOS device prompt
	// This pattern captures the prompt with variable content before the '>'
	const promptPattern = `[\w\-\.@]+>` // Matches strings like "admin@vmx-ne1>"
	const expectedPromptSuffix = ">"

	prompt, err := junos.FindDevicePrompt(promptPattern, expectedPromptSuffix)
	if err != nil {
		return err
	}
	junos.Prompt = prompt

	log.Infof("junos.Prompt is: %s", prompt)

	return nil
}

func (junos *JUNOSDeviceConnection) SendCommand(command string, cliPromptMode string, timeout time.Duration) (string, error) {

	var outputBuffer bytes.Buffer
	var promptMode string
	var processedOutput string

	var err error

	stdin := junos.Connection.Writer
	stdout := junos.Connection.Reader

	scanner := bufio.NewScanner(stdout)
	done := make(chan bool)

	if cliPromptMode == "running" {
		promptMode = junos.Prompt // admin@vmx-ne1>

		go func() {
			defer func() { done <- true }()
			appearanceCount := 0
			for scanner.Scan() {
				line := cleanOutputJunos(scanner.Text())
				outputBuffer.WriteString(line + "\n")
				log.Info("Received line: ", line)

				// Increment appearance count if the line contains the specific string
				if strings.Contains(line, promptMode) {
					log.Infof("line contain PromptMode, appearance count: %s", strconv.Itoa(appearanceCount))
					appearanceCount++
					if appearanceCount == 1 {
						log.Infof("line contain PromptMode, appearance count: %s", strconv.Itoa(appearanceCount))
						log.Infof("Detected %s appearance of end marker", strconv.Itoa(appearanceCount))
						done <- true
						return
					}
				}

			}
			if err := scanner.Err(); err != nil {
				log.Error("Error reading stdout:", err)
			}
		}()

		log.Infof("Sending command: %s", command)
		_, err := fmt.Fprintf(stdin, "%s | no-more \n\n", command)

		if err != nil {
			fmt.Println("Error writing to stdin:", err)
			return "", err
		}

		// Wait for the output to be read or timeout
		select {
		case <-done:
			log.Info("Reading completed")
		case <-time.After(timeout):
			log.Info("Timeout waiting for reading to complete")
		}

		output := outputBuffer.String()

		lines := strings.Split(output, "\n")               // Split the output string into lines
		trimmedLines := lines[1 : len(lines)-1]            // Remove the first and last two lines
		processedOutput = strings.Join(trimmedLines, "\n") // Join the remaining lines into a single string
		// fmt.Print(processedOutput)

		log.Info("Final output: ")
		log.Debug(output)

	} else if cliPromptMode == "candidate" {
		promptMode = "[edit]"

		go func() {
			defer func() { done <- true }()
			appearanceCount := 0
			for scanner.Scan() {
				line := cleanOutputJunos(scanner.Text())
				outputBuffer.WriteString(line + "\n")
				log.Info("Received line: ", line+"\n")

				// Increment appearance count if the line contains the specific string
				if strings.Contains(line, promptMode) || strings.Contains(line, "commit complete") {
					appearanceCount++
					if appearanceCount == 4 {
						log.Info("Detected second appearance of end marker")
						done <- true
						return
					}
				}

			}
			if err = scanner.Err(); err != nil {
				log.Error("Error reading stdout:", err)
			}
		}()

		log.Infof("Sending command: %s", command)

		_, err = fmt.Fprintf(stdin, "%s\n", "configure")
		if err != nil {
			fmt.Println("Error writing to stdin:", err)
			return "", err
		}

		_, err = fmt.Fprintf(stdin, "%s\n", command)
		if err != nil {
			fmt.Println("Error writing to stdin:", err)
			return "", err
		}

		_, err = fmt.Fprintf(stdin, "%s\n", "commit")
		if err != nil {
			fmt.Println("Error writing to stdin:", err)
			return "", err
		}

		// Wait for the output to be read or timeout
		select {
		case <-done:
			log.Info("Reading completed")
		case <-time.After(timeout):
			log.Info("Timeout waiting for reading to complete")
		}

		output := outputBuffer.String()

		lines := strings.Split(output, "\n")               // Split the output string into lines
		trimmedLines := lines[10 : len(lines)-2]           // Remove the first and last two lines
		processedOutput = strings.Join(trimmedLines, "\n") // Join the remaining lines into a single string
		// fmt.Print(processedOutput)

		log.Info("Final output: ")
		log.Debug(output)

	} else {
		log.Infof("Unspported cliPromptMode: %s", cliPromptMode)
		return "", nil
	}

	return processedOutput, nil

}

func cleanOutputJunos(output string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\[\?2004[lh]|\x1b\[\?25[hl]|\x1b\[\?12[hl]|\x1b\[\?7[hl]`)
	output = re.ReplaceAllString(output, "")

	return output
}
