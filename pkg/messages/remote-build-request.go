package messages

const (
	RequestTypePR                = "PR"
	RequestTypeBranch            = "BRANCH"
	RequestTypeTag               = "TAG"
	RequestParameterKind         = "KIND"
	RequestParameterTarget       = "TARGET"
	RequestParameterRunScript    = "RUN_SCRIPT"
	RequestParameterSetupScript  = "SETUP_SCRIPT"
	RequestParameterRcvQueueName = "RCV_QUEUE_NAME"
	RequesParameterRepoURL       = "REPO_URL"
)

type RemoteBuildRequestMessage struct {
	RepoURL        string `json:"repourl"`
	Kind           string `json:"kind"`
	Target         string `json:"target"`
	SetupScript    string `json:"setupscript"`
	RunScript      string `json:"runscript"`
	RcvIdent       string `json:"rcvident"`
	RunScriptURL   string `json:"runscripturl"`
	MainBranch     string `json:"mainbranch"`
	JenkinsProject string `json:"jenkinsproject"`
}

func NewRemoteBuildRequestMessage(repoURL, kind, target, setupscript, runscript, recieveQueueName, runscripturl, mainBranch, jobName string) *RemoteBuildRequestMessage {
	r := &RemoteBuildRequestMessage{
		RepoURL:        repoURL,
		Kind:           kind,
		Target:         target,
		SetupScript:    setupscript,
		RunScript:      runscript,
		RcvIdent:       recieveQueueName,
		RunScriptURL:   runscripturl,
		MainBranch:     mainBranch,
		JenkinsProject: jobName,
	}
	return r
}
