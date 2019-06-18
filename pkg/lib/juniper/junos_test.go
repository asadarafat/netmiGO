package juniper

import (
	"strings"
	"testing"
)

type mockDriver struct {
	ReadSideEffect func() string
	CmdCalls       *string
	PatternCalls   *string
	PromptRegex    *string
	GenericCalls   *string
}

func (c mockDriver) Connect() {

}
func (c mockDriver) Disconnect() {
	*c.GenericCalls = "disconnect"

}
func (c mockDriver) SendCommand(cmd string, expectPattern string) (string, error) {
	if c.CmdCalls != nil {
		*c.CmdCalls += cmd + ", "
	} else {
		c.CmdCalls = &cmd
	}

	return c.ReadUntil(expectPattern), nil

}
func (c mockDriver) SendCommandsSet(cmds []string, expectPattern string) (string, error) {
	for _, cmd := range cmds {
		c.SendCommand(cmd, expectPattern)
	}
	return c.ReadUntil(expectPattern), nil

}

func (c mockDriver) FindDevicePrompt(regex string, pattern string) string {
	*c.PromptRegex = regex
	return c.ReadUntil(pattern)

}

func (c mockDriver) ReadUntil(pattern string) string {
	*c.PatternCalls += pattern + ", "

	return c.ReadSideEffect()

}

func TestJunOSDevice_Connect(t *testing.T) {

	// [1] test happy scenario with login -> userMode -> enableMode
	mockD := mockDriver{}
	var cmdCalls, patternCalls, promptRegexCall string
	mockD.CmdCalls = &cmdCalls
	mockD.PatternCalls = &patternCalls
	mockD.PromptRegex = &promptRegexCall

	callsCount := 0
	mockD.ReadSideEffect = func() string {
		callsCount += 1
		switch callsCount {
		case 1:
			return "@jun189"
		case 2:
			return "lorem ipsum 123\nswitch>lorem"
		case 3:
			return "lorem ipsum 123\nswitch>lorem"
		default:
			return ""

		}

	}
	base := JunOSDevice{"host", "password", "junos", "", mockD}
	base.Connect()

	if base.Prompt != "@jun189" {
		t.Error("Driver.FindDevicePrompt was not called")
	}
	expected := "%, @jun189, @jun189, "

	if patternCalls != expected {
		t.Errorf("wrong Juniper Pattern calls, Expected: (%s) Got: (%s)", expected, patternCalls)
	}

	expected = "cli, set cli screen-length 0, "

	if cmdCalls != expected {
		t.Errorf("wrong juniper commands calls, Expected: (%s) Got: (%s)", expected, cmdCalls)
	}

	expected = "(@.*)[#>%]"

	if promptRegexCall != expected {
		t.Errorf("wrong Juniper prompt regex calls, Expected: (%s) Got: (%s)", expected, promptRegexCall)
	}

}

func TestJunOSDevice_Disconnect(t *testing.T) {
	mockD := mockDriver{}
	var genericCalls string
	mockD.GenericCalls = &genericCalls

	base := JunOSDevice{"host", "password", "juniper", "", mockD}

	base.Disconnect()

	if genericCalls != "disconnect" {
		t.Error("Driver.Connect was not called")
	}

}

func TestJunOSDevice_SendCommand(t *testing.T) {
	mockD := mockDriver{}
	var cmdCalls, patternCalls, promptRegexCall string
	mockD.CmdCalls = &cmdCalls
	mockD.PatternCalls = &patternCalls
	mockD.PromptRegex = &promptRegexCall
	mockD.ReadSideEffect = func() string {
		return "show interfaces brief\n" +
			"Physical interface: cbp0, Enabled,\n" +
			"Physical interface: demux0, Enabled,\n" +
			"switch1# "

	}

	base := JunOSDevice{"host", "password", "juniper", "@jun189", mockD}
	result, _ := base.SendCommand("show interfaces brief")

	if !strings.Contains(result, "Physical interface: cbp0, Enabled,") &&
		!strings.Contains(result, "Physical interface: demux0, Enabled,") {
		t.Error("wrong result returned")
	}

	expected := "show interfaces brief, "

	if cmdCalls != expected {
		t.Errorf("wrong commands calls, expected: (%s) got: (%s)", expected, cmdCalls)
	}

}

func TestJunOSDevice_SendConfigSet(t *testing.T) {
	mockD := mockDriver{}
	var cmdCalls, patternCalls, promptRegexCall string
	mockD.CmdCalls = &cmdCalls
	mockD.PatternCalls = &patternCalls
	mockD.PromptRegex = &promptRegexCall
	mockD.ReadSideEffect = func() string {
		return "show interfaces brief\n" +
			"Physical interface: cbp0, Enabled,\n" +
			"Physical interface: demux0, Enabled,\n" +
			"switch1# "

	}

	base := JunOSDevice{"host", "password", "juniper", "jun189", mockD}
	cmds := []string{"set routing-options static route 192.168.47.0/24 next-hop 172.16.1.2"}
	base.SendConfigSet(cmds)

	expected := "configure, set routing-options static route 192.168.47.0/24 next-hop 172.16.1.2, commit, exit, "


	if cmdCalls != expected {
		t.Errorf("wrong commands calls, expected: (%s) got: (%s)", expected, cmdCalls)
	}

}