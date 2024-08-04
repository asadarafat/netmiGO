package netmigo

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// SRLDeviceConnection represents a specific device type that uses a driver to connect and send commands.
type SRLDeviceConnection struct {
	DeviceConnection
	DeviceType string
	Prompt     string
}

func NewSRLDeviceConnection(connection *SSHConnModel, DeviceType string) (*SRLDeviceConnection, error) {
	return &SRLDeviceConnection{
		DeviceConnection: DeviceConnection{
			Connection: connection,
			Return:     "\n",
		},
		DeviceType: DeviceType,
	}, nil
}

func (srl *SRLDeviceConnection) SendCommand(command string, cliPromptMode string, timeout time.Duration) (string, error) {

	var outputBuffer bytes.Buffer
	var promptMode string
	var processedOutput string

	stdin := srl.Connection.Writer
	stdout := srl.Connection.Reader

	scanner := bufio.NewScanner(stdout)
	done := make(chan bool)

	if cliPromptMode == "running" {
		promptMode = "-{ [OLD STARTUP] + running }--[  ]--"
		go func() {
			defer func() { done <- true }()
			appearanceCount := 0
			for scanner.Scan() {
				line := cleanOutput(scanner.Text())
				outputBuffer.WriteString(line + "\n")
				log.Info("Received line:", line)

				// Increment appearance count if the line contains the specific string
				if strings.Contains(line, promptMode) {
					appearanceCount++
					if appearanceCount == 2 {
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
		_, err := fmt.Fprintf(stdin, "%s\n", command)
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
		trimmedLines := lines[2 : len(lines)-2]            // Remove the first and last two lines
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

// NewDevice initializes a new SRL device connection
func InitSRLDevice(Host string, Username string, Password string, Port uint8) (*SRLDeviceConnection, error) {

	// Create connection
	connection, err := InitTransport(Host, Username, Password, "ssh", Port)
	if err != nil {
		return nil, err
	}

	// Create the device
	return NewSRLDeviceConnection(connection, "nokia_srl")
}

func (srl *SRLDeviceConnection) Connect() error {
	if err := srl.DeviceConnection.ConnectXterm(); err != nil {
		return err
	}

	// Define the regex pattern to find the SROS device prompt
	const promptPattern = "\\*?([ABCD]:\\S*@?\\S+)[#>%]"
	const expectedPromptSuffix = "#"

	// const promptPattern = `(?im)^--{(\s\[[\w\s]+\]){0,5}[\+\*\s]{1,}running\s}--\[.+?\]--\s*\n[abcd]:\S+#\s*$`
	// const expectedPromptSuffix = ``

	prompt, err := srl.FindDevicePrompt(promptPattern, expectedPromptSuffix)
	if err != nil {
		return err
	}
	srl.Prompt = prompt

	log.Infof("sros.Prompt is: %s", prompt)

	return nil
}

func (srl *SRLDeviceConnection) SendCommands(commands []string, prompt string) (string, error) {

	var outputBuffer bytes.Buffer

	stdin := srl.Connection.Writer
	stdout := srl.Connection.Reader

	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			line := cleanOutput(scanner.Text())
			outputBuffer.WriteString(line + "\n")
			// fmt.Println(line)
			// if strings.Contains(line, prompt) {
			// 	stdin.Write([]byte("quit\n"))
			// }
		}
	}()

	for _, cmd := range commands {
		fmt.Fprintf(stdin, "%s\n", cmd)
		time.Sleep(4 * time.Second)
	}

	// stdin.Close()
	return outputBuffer.String(), nil
}

func cleanOutput(output string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\[\?2004[lh]|\x1b\[\?25[hl]|\x1b\[\?12[hl]|\x1b\[\?7[hl]`)
	output = re.ReplaceAllString(output, "")

	// reRunning := regexp.MustCompile(`--\{ \[OLD STARTUP\] \+ running \}--\[  \]--`)
	// output = reRunning.ReplaceAllString(output, "")

	return output
}
