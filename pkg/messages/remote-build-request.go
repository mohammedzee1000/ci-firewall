package messages

type RequestType string

const (
	RequestTypePR                = RequestType("PR")
	RequestTypeBranch            = RequestType("BRANCH")
	RequestTypeTag               = RequestType("TAG")
	RequestParameterKind         = "KIND"
	RequestParameterTarget       = "TARGET"
	RequestParameterRunScript    = "RUN_SCRIPT"
	RequestParameterSetupScript  = "SETUP_SCRIPT"
	RequestParameterRcvQueueName = "RCV_QUEUE_NAME"
	RequestParameterRepoURL      = "REPO_URL"
)

type RemoteBuildRequestMessage struct {
	RepoURL          string      `json:"repourl"`
	Kind             RequestType `json:"kind"`
	Target           string      `json:"target"`
	SetupScript      string      `json:"setupscript"`
	RunScript        string      `json:"runscript"`
	ReceiveQueueName string      `json:"rcvident"`
	RunScriptURL     string      `json:"runscripturl"`
	MainBranch       string      `json:"mainbranch"`
	JenkinsProject   string      `json:"jenkinsproject"`
}

func NewRemoteBuildRequestMessage(repoURL string, kind RequestType, target, setupScript, runscript, receiveQueueName, runScriptURL, mainBranch, jobName string) *RemoteBuildRequestMessage {
	r := &RemoteBuildRequestMessage{
		RepoURL:          repoURL,
		Kind:             kind,
		Target:           target,
		SetupScript:      setupScript,
		RunScript:        runscript,
		ReceiveQueueName: receiveQueueName,
		RunScriptURL:     runScriptURL,
		MainBranch:       mainBranch,
		JenkinsProject:   jobName,
	}
	return r
}
