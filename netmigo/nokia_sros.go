package netmigo

import "errors"

// SROSDeviceConnection represents a specific device type that uses a driver to connect and send commands.
type SROSDeviceConnection struct {
	DeviceConnection
	DeviceType string
	Prompt     string
}

func NewSROSDeviceConnection(connection *SSHConnModel, DeviceType string) (*SROSDeviceConnection, error) {
	return &SROSDeviceConnection{
		DeviceConnection: DeviceConnection{
			Connection: connection,
			Return:     "\n",
		},
		DeviceType: DeviceType,
	}, nil
}

func (sros *SROSDeviceConnection) Connect() error {
	if err := sros.DeviceConnection.Connect(); err != nil {
		return err
	}
	prompt, err := sros.FindDevicePrompt("\\*?([ABCD]:\\S*@?\\S+)[#>%]", "#")
	if err != nil {
		return err
	}
	sros.Prompt = prompt
	return sros.sessionPreparation()
}

func (sros *SROSDeviceConnection) SendCommand(cmd string) (string, error) {
	return sros.SendCommandPattern(cmd, sros.Prompt)
}

func (sros *SROSDeviceConnection) SendConfigSet(cmds []string) (string, error) {
	results, _ := sros.SendCommandPattern("configure exclusive", sros.Prompt)
	cmds = append(cmds, "commit", "exit")
	out, err := sros.SendCommandsSetPattern(cmds, sros.Prompt)
	results += out
	return results, err
}

func (sros *SROSDeviceConnection) sessionPreparation() error {
	if _, err := sros.SendCommand("environment no more"); err != nil {
		return errors.New("failed to disable pagination: " + err.Error())
	}
	if _, err := sros.SendCommand("environment more false"); err != nil {
		return errors.New("failed to disable pagination: " + err.Error())
	}
	return nil
}

// NewDevice initializes a new SROS device connection
func InitSROSDevice(Host string, Username string, Password string, Port uint8) (*SROSDeviceConnection, error) {

	// Create connection
	connection, err := InitTransport(Host, Username, Password, "ssh", Port)
	if err != nil {
		return nil, err
	}

	// Create the device
	return NewSROSDeviceConnection(connection, "nokia_sros")
}
