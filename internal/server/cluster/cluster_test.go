package cluster_test

import (
	"github.com/cirruslabs/chacha/internal/config"
	"github.com/cirruslabs/chacha/internal/server/cluster"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestClusterLocalNode(t *testing.T) {
	cluster := cluster.New("doesn't matter", "other2", []config.Node{
		{Addr: "other1"},
		{Addr: "other2"},
		{Addr: "other3"},
	})
	require.Equal(t, "other2", cluster.LocalNode())
}

func TestClusterStabilityAdd(t *testing.T) {
	cluster1 := cluster.New("doesn't matter", "other2", []config.Node{
		{Addr: "other1"},
		{Addr: "other2"},
		{Addr: "other3"},
	})
	cluster2 := cluster.New("doesn't matter", "other2", []config.Node{
		{Addr: "other0"},
		{Addr: "other1"},
		{Addr: "other2"},
		{Addr: "other3"},
	})

	require.Equal(t, cluster1.TargetNode("test"), cluster2.TargetNode("test"))
}

func TestClusterStabilityRemove(t *testing.T) {
	cluster1 := cluster.New("doesn't matter", "other2", []config.Node{
		{Addr: "other1"},
		{Addr: "other2"},
		{Addr: "other3"},
	})
	cluster2 := cluster.New("doesn't matter", "other2", []config.Node{
		{Addr: "other2"},
		{Addr: "other3"},
	})

	require.Equal(t, cluster1.TargetNode("test"), cluster2.TargetNode("test"))
}
