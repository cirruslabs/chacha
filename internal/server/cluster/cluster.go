package cluster

import (
	"github.com/cirruslabs/chacha/internal/config"
	"github.com/deckarep/golang-set/v2"
	"github.com/nspcc-dev/hrw/v2"
	"github.com/samber/lo"
)

type Cluster struct {
	secret string
	addr   string
	nodes  mapset.Set[string]
}

func New(secret string, addr string, nodes []config.Node) *Cluster {
	return &Cluster{
		secret: secret,
		addr:   addr,
		nodes: mapset.NewSet[string](lo.Map(nodes, func(node config.Node, _ int) string {
			return node.Addr
		})...),
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

	for node := range cluster.nodes.Iter() {
		allNodesHashable = append(allNodesHashable, hrw.HashableBytes(node))
	}

	hrw.Sort(allNodesHashable, hrw.WrapBytes([]byte(key)))

	return string(allNodesHashable[0])
}

func (cluster *Cluster) ContainsNode(node string) bool {
	return cluster.nodes.ContainsOne(node)
}

func (cluster *Cluster) Nodes() []string {
	return cluster.nodes.ToSlice()
}
