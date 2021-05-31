package nodefile

import (
	"fmt"
	"github.com/mohammedzee1000/ci-firewall/pkg/ci-firewall/cli/genericclioptions"
	"github.com/mohammedzee1000/ci-firewall/pkg/node"
	"github.com/spf13/cobra"
)

const NodefileAddNodeRecommendedCommandName = "addnode"

type NodeFileAddNodeOptions struct {
	nodefile string
	name string
	user string
	address string
	port int
	baseos string
	arch string
	password string
	privatekeyfile string
	tags []string
	group string
}

func newNodeFileAddNodeOptions() *NodeFileAddNodeOptions {
	return &NodeFileAddNodeOptions{}
}

func (nfano *NodeFileAddNodeOptions) Complete(name string, cmd *cobra.Command, args []string) error {
	return nil
}

func (nfano *NodeFileAddNodeOptions) Validate() (err error) {
	if nfano.name == "" {
		return fmt.Errorf("please provide a node name")
	}
	if nfano.user == "" {
		return fmt.Errorf("please provide a username")
	}
	if nfano.address == "" {
		return fmt.Errorf("please provide an address")
	}
	if nfano.baseos == "" {
		return fmt.Errorf("please provide a baseos")
	}
	if nfano.arch == "" {
		return fmt.Errorf("please provide an arch")
	}
	if nfano.password == "" && nfano.privatekeyfile == "" {
		return fmt.Errorf("please provide password or privat key file path to node")
	}
	return nil
}

func (nfano *NodeFileAddNodeOptions) Run() (err error) {
	err = node.AddNodeToFile(nfano.nodefile, nfano.name, nfano.user, nfano.address, nfano.baseos, nfano.arch, nfano.password, nfano.privatekeyfile, nfano.tags, nfano.port, nfano.group)
	if err != nil {
		return fmt.Errorf("failed to add node info to file : %w", err)
	}
	fmt.Printf("succesfully added node to %s\n", nfano.nodefile)
	return nil
}

func NewCmdNodeFileAddNode(name, fullname string) *cobra.Command {
	o := newNodeFileAddNodeOptions()
	cmd := &cobra.Command{
		Use: name,
		Short: "add node to node file",
		Run: func(cmd *cobra.Command, args []string) {
			genericclioptions.GenericRun(o, cmd, args)
		},
	}
	cmd.Flags().StringVar(&o.nodefile, "nodefile", "nodes.json", "The path of the node json file to add node to")
	cmd.Flags().StringVar(&o.name, "name", "", "The name of the node to add")
	cmd.Flags().StringVar(&o.user, "user", "", "The ssh username of node")
	cmd.Flags().StringVar(&o.address, "address", "", "The ip or hostname of node")
	cmd.Flags().IntVar(&o.port, "port", 0, "The ssh port on the node. Defaults to 22")
	cmd.Flags().StringVar(&o.baseos, "baseos", "linux", "The baseos of the node. Like linux|windows|mac etc. Defaults to linux")
	cmd.Flags().StringVar(&o.arch, "arch", "amd64", "The architecture of the node. Defaults to x86_64")
	cmd.Flags().StringVar(&o.password, "password", "", "The password to use to login to node")
	cmd.Flags().StringVar(&o.privatekeyfile, "privatekeyfile", "", "The path to the private key file")
	cmd.Flags().StringArrayVar(&o.tags, "tag", []string{}, "The tags to add to the node")
	cmd.Flags().StringVar(&o.group, "group", "", "the group to which the machine belongs. Defaults to \"\"")
	return cmd
}