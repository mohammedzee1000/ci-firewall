package messages

const (
	KindBuild  = "Build"
	KindLog    = "Log"
	KindStatus = "Status"
	KindFinal  = "Final"
)

type Message struct {
	Kind  string `json:"Kind"`
	Build int    `json:"Build"`
	JenkinsProject string `json:"JenkinsProject"`
}

func newMessage(kind string, build int, jeninsproject string) *Message {
	return &Message{
		Kind:           kind,
		Build:          build,
		JenkinsProject: jeninsproject,
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

func (m *Message) IsFinal() bool {
	return m.Kind == KindFinal
}

type BuildMessage struct {
	*Message
}

func NewBuildMessage(build int, jenkinsjob string) *BuildMessage {
	return &BuildMessage{
		Message: newMessage(KindBuild, build, jenkinsjob),
	}
}

type LogsMessage struct {
	*Message
	Logs string `json:"Logs"`
}

func NewLogsMessage(build int, logs string, jenkinsproject string) *LogsMessage {
	return &LogsMessage{
		Message: newMessage(KindLog, build, jenkinsproject),
		Logs:    logs,
	}
}

type StatusMessage struct {
	*Message
	Success bool `json:"Success"`
}

func NewStatusMessage(build int, success bool, jenkinsproject string) *StatusMessage {
	return &StatusMessage{
		Message: newMessage(KindStatus, build, jenkinsproject),
		Success: success,
	}
}

type FinalMessage struct {
	*Message
}

func NewFinalMessage(build int, jenkinsproject string) *FinalMessage {
	return &FinalMessage{
		Message: newMessage(KindFinal, build, jenkinsproject),
	}
}
