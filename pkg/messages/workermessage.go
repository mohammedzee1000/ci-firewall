package messages

const (
	KindBuild  = "Build"
	KindLog    = "Log"
	KindStatus = "Status"
	KindFinal  = "Final"
	KindCancel = "Cancel"
)

type Message struct {
	Kind  string `json:"Kind"`
	Build int    `json:"Build"`
	JenkinsProject string `json:"JenkinsProject"`
}

func newMessage(kind string, build int, jenkinsProject string) *Message {
	return &Message{
		Kind:           kind,
		Build:          build,
		JenkinsProject: jenkinsProject,
	}
}

func (m *Message) IsBuild() bool {
	return m.Kind == KindBuild
}

func (m *Message) IsLog() bool {
	return m.Kind == KindLog
}

func (m *Message) IsStatus() bool {
	return m.Kind == KindStatus
}

func (m *Message) IsFinal() bool {
	return m.Kind == KindFinal
}

func (m *Message) IsCancel() bool {
	return m.Kind == KindCancel
}

type BuildMessage struct {
	*Message
}

func NewBuildMessage(build int, jenkinsProject string) *BuildMessage {
	return &BuildMessage{
		Message: newMessage(KindBuild, build, jenkinsProject),
	}
}

type LogsMessage struct {
	*Message
	Logs string `json:"Logs"`
}

func NewLogsMessage(build int, logs string, jenkinsProject string) *LogsMessage {
	return &LogsMessage{
		Message: newMessage(KindLog, build, jenkinsProject),
		Logs:    logs,
	}
}

type StatusMessage struct {
	*Message
	Success bool `json:"Success"`
}

func NewStatusMessage(build int, success bool, jenkinsProject string) *StatusMessage {
	return &StatusMessage{
		Message: newMessage(KindStatus, build, jenkinsProject),
		Success: success,
	}
}

type FinalMessage struct {
	*Message
}

func NewFinalMessage(build int, jenkinsProject string) *FinalMessage {
	return &FinalMessage{
		Message: newMessage(KindFinal, build, jenkinsProject),
	}
}

type CancelMessage struct {
	*Message
}

func NewCancelMessage(build int, jenkinsProject string) *CancelMessage  {
	return &CancelMessage{
		Message: newMessage(KindCancel, build, jenkinsProject),
	}
}