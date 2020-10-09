package node

type Node struct {
	Name        string `json:"name"`
	User        string `json:"user"`
	Address     string `json:"address"`
	BaseOS      string `json:"baseos"`
	Arch        string `json:"arch"`
	SSHPassword string `json:"password"`
	SSHKey      string `json:"privatekey"`
}

type NodeList struct {
	Nodes []Node `json:"nodes"`
}
