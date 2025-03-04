package cluster

import (
	"github.com/cirruslabs/chacha/internal/config"
	"github.com/nspcc-dev/hrw/v2"
)

type Cluster struct {
	secret string
	addr   string
	nodes  []config.Node
}

func New(secret string, addr string, nodes []config.Node) *Cluster {
	return &Cluster{
		secret: secret,
		addr:   addr,
		nodes:  nodes,
	}
}

func (cluster *Cluster) Secret() string {
	return cluster.secret
}

func (cluster *Cluster) LocalNode() string {
	return cluster.addr
}

func (cluster *Cluster) TargetNode(key string) string {
	var allNodesHashable []hrw.HashableBytes

	for _, node := range cluster.nodes {
		allNodesHashable = append(allNodesHashable, hrw.HashableBytes(node.Addr))
	}

	hrw.Sort(allNodesHashable, hrw.WrapBytes([]byte(key)))

	return string(allNodesHashable[0])
}
