package messages

const (
	KindBuild    = "Build"
	KindLog      = "Log"
	KindStatus   = "Status"
	KindFinalize = "Finalize"
)

type Message struct {
	Kind  string `json:"Kind"`
	Build int    `json:"Build"`
}

func newMessage(kind string, build int) *Message {
	return &Message{
		Kind:  kind,
		Build: build,
	}
}

func (m *Message) IsBuild() bool {
	return m.Kind == KindBuild
}

func (m *Message) ISLog() bool {
	return m.Kind == KindLog
}

func (m *Message) IsStatus() bool {
	return m.Kind == KindStatus
}

func (m *Message) IsFinalize() bool {
	return m.Kind == KindFinalize
}

type BuildMessage struct {
	*Message
}

func NewBuildMessage(build int) *BuildMessage {
	return &BuildMessage{
		Message: newMessage(KindBuild, build),
	}
}

type LogsMessage struct {
	*Message
	Logs string `json:"Logs"`
}

func NewLogsMessage(build int, logs string) *LogsMessage {
	return &LogsMessage{
		Message: newMessage(KindLog, build),
		Logs:    logs,
	}
}

type StatusMessage struct {
	*Message
	Success bool `json:"Success"`
}

func NewStatusMessage(build int, success bool) *StatusMessage {
	return &StatusMessage{
		Message: newMessage(KindStatus, build),
		Success: success,
	}
}

type FinalizeMessage struct {
	*Message
}

func NewFinalizeMessage(build int) *FinalizeMessage {
	return &FinalizeMessage{
		Message: newMessage(KindFinalize, build),
	}
}
