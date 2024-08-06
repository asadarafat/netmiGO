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

// IOSXRDeviceConnection represents a specific device type that uses a driver to connect and send commands.
type IOSXRDeviceConnection struct {
	DeviceConnection
	DeviceType string
	Prompt     string
}

func NewIOSXRDeviceConnection(connection *SSHConnModel, DeviceType string) (*IOSXRDeviceConnection, error) {
	return &IOSXRDeviceConnection{
		DeviceConnection: DeviceConnection{
			Connection: connection,
			Return:     "\n",
		},
		DeviceType: DeviceType,
	}, nil
}

// NewDevice initializes a new IOSXR device connection
func InitIOSXRDevice(Host string, Username string, Password string, Port uint8) (*IOSXRDeviceConnection, error) {

	// Create connection
	connection, err := InitTransport(Host, Username, Password, "ssh", Port)
	if err != nil {
		return nil, err
	}

	// Create the device
	return NewIOSXRDeviceConnection(connection, "cisco_iosxr")
}

func (iosxr *IOSXRDeviceConnection) Connect() error {
	if err := iosxr.DeviceConnection.ConnectXterm(); err != nil {
		return err
	}

	// Define the regex pattern to find the IOSXR device prompt
	// This pattern captures the prompt structure of "RP/0/RP0/CPU0:R11-P#"
	const promptPattern = `RP\/\d+\/RP\d+\/CPU\d+:[\w\-]+#`
	const expectedPromptSuffix = "#"

	prompt, err := iosxr.FindDevicePrompt(promptPattern, expectedPromptSuffix)
	if err != nil {
		return err
	}
	iosxr.Prompt = prompt

	log.Infof("iosxr.Prompt is: %s", prompt)

	return nil
}

func (iosxr *IOSXRDeviceConnection) SendCommand(command string, cliPromptMode string, timeout time.Duration) (string, error) {

	var outputBuffer bytes.Buffer
	var promptMode string
	var processedOutput string

	var err error

	stdin := iosxr.Connection.Writer
	stdout := iosxr.Connection.Reader

	scanner := bufio.NewScanner(stdout)
	done := make(chan bool)

	if cliPromptMode == "running" {
		promptMode = iosxr.Prompt // RP/0/RP0/CPU0:R11-P#

		go func() {
			defer func() { done <- true }()
			appearanceCount := 0
			for scanner.Scan() {
				line := cleanOutputIosxr(scanner.Text())
				outputBuffer.WriteString(line + "\n")
				log.Info("Received line: ", line)

				// Increment appearance count if the line contains the specific string
				if strings.Contains(line, promptMode) {

					log.Infof("line contain PromptMode, appearance count: %s", strconv.Itoa(appearanceCount))

					appearanceCount++
					if appearanceCount == 1 {
						log.Info("Detected second appearance of end marker")
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
		_, err := fmt.Fprintf(stdin, "%s\n\n", command)

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
				line := cleanOutputIosxr(scanner.Text())
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

func cleanOutputIosxr(output string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\[\?2004[lh]|\x1b\[\?25[hl]|\x1b\[\?12[hl]|\x1b\[\?7[hl]`)
	output = re.ReplaceAllString(output, "")

	return output
}

func (iosxr *IOSXRDeviceConnection) CopyRunningConfig(savedConfigFileName string, cliPromptMode string, timeout time.Duration) (string, error) {

	var outputBuffer bytes.Buffer
	var promptMode string
	var processedOutput string
	var output string

	stdin := iosxr.Connection.Writer
	stdout := iosxr.Connection.Reader

	scanner := bufio.NewScanner(stdout)
	done := make(chan bool)

	expectString := fmt.Sprintf("[/%s]?", savedConfigFileName)

	if cliPromptMode == "running" {
		promptMode = iosxr.Prompt // RP/0/RP0/CPU0:R11-P#

		go func() {
			defer func() { done <- true }()
			appearanceCount := 0
			for scanner.Scan() {
				line := cleanOutputIosxr(scanner.Text())
				// outputBuffer.WriteString(line + "\n")
				outputBuffer.WriteString(line + "\n")

				log.Info("Received line: ", line)

				if strings.Contains(line, expectString) {
					log.Info("expectString found: ", line)
					log.Info("Send Carriage Return")
					fmt.Fprintf(stdin, "\n") //return enter
				}
				// else if strings.Contains(line, "overwrite?") {
				// 	log.Info("expectString found: ", line)
				// 	fmt.Fprintf(stdin, "yes") //return enter
				// 	fmt.Fprintf(stdin, "\n")  //return enter

				// }

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

		commandSaveRunningConfig := fmt.Sprintf("copy running-config %s", savedConfigFileName)
		commands := []string{
			fmt.Sprintf("%s\n", commandSaveRunningConfig),
			"\n",
			"yes",
			"\n",
		}

		log.Infof("Sending commands: %s", commands)

		for _, cmd := range commands {
			if _, err := fmt.Fprintf(stdin, "%s", cmd); err != nil {
				fmt.Println("Error writing to stdin:", err)
				return "", err
			}
		}

		// Wait for the output to be read or timeout
		select {
		case <-done:
			log.Info("Reading completed")
		case <-time.After(timeout):
			log.Info("Timeout waiting for reading to complete")
		}

		output = outputBuffer.String()

		lines := strings.Split(output, "\n")               // Split the output string into lines
		trimmedLines := lines[1 : len(lines)-1]            // Remove the first and last two lines
		processedOutput = strings.Join(trimmedLines, "\n") // Join the remaining lines into a single string

		log.Debug(processedOutput)

	} else {
		log.Infof("Unspported cliPromptMode: %s", cliPromptMode)
		return "", nil
	}

	return output, nil

}
