package node

import (
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func TestNoPeerDiscovery_UpdatePeers(t *testing.T) {

	discovery := NoPeerDiscovery{}
	assert.Empty(t, discovery.FindNewPeers([]*Node{{}}, []string{"1.2.3.4", "2.3.4.5"}))

}

func TestPeerList_Contains(t *testing.T) {
	peerList := UpdatedPeerList{
		originalPeers: []string{"1.2.3.4", "2.3.4.5"},
		newPeers:      []string{"3.4.5.6", "4.5.6.7"},
		mutex:         sync.Mutex{},
	}

	assert.True(t, peerList.contains("1.2.3.4"))
	assert.True(t, peerList.contains("2.3.4.5"))
	assert.True(t, peerList.contains("3.4.5.6"))
	assert.True(t, peerList.contains("4.5.6.7"))
	assert.False(t, peerList.contains("5.6.7.8"))
}

func TestPeerList_AddIfNew(t *testing.T) {
	peerList := UpdatedPeerList{
		originalPeers: []string{"1.2.3.4", "2.3.4.5"},
		newPeers:      []string{"3.4.5.6", "4.5.6.7"},
		mutex:         sync.Mutex{},
	}

	assert.False(t, peerList.addIfNew("1.2.3.4"))
	assert.False(t, peerList.addIfNew("2.3.4.5"))
	assert.False(t, peerList.addIfNew("3.4.5.6"))
	assert.False(t, peerList.addIfNew("4.5.6.7"))

	assert.True(t, peerList.addIfNew("5.6.7.8"))
	assert.False(t, peerList.addIfNew("5.6.7.8")) // already added
}

func TestPeerList_IsAcceptedHost(t *testing.T) {

	peerList := UpdatedPeerList{
		originalPeers: []string{"1.2.3.4"},
		newPeers:      []string{"2.3.4.5"},
		excludedPeers: []string{"6.6.6.6"},
		mutex:         sync.Mutex{},
	}

	assert.True(t, peerList.isAcceptedHost("1.2.3.4"))
	assert.False(t, peerList.isAcceptedHost("6.6.6.6"))
}

func TestPublicPeerDiscovery_UpdatePeers(t *testing.T) {
	createNodeFunc := func(host string) (*Node, error) {
		if host == "6.6.6.6" {
			return nil, errors.Errorf("Error creating node [%s].", host)
		} else {
			// new node with 1 new working peers (6.7.8.9), 1 erroneous new peer (6.6.6.6)
			return createTestNodeWithPeers(host, []string{"1.2.3.4", "5.6.7.8", "6.6.6.6", "6.7.8.9"}),
				nil
		}
	}
	discovery := newPublicPeerDiscovery(createNodeFunc, []string{}, time.Hour)

	discoveredPeers := discovery.FindNewPeers([]*Node{
		createTestNodeWithPeers("1.2.3.4",
			[]string{"2.3.4.5", "3.4.5.6", "4.5.6.7", "5.6.7.8"}), //  3 new peers ("3.4.5.6", "4.5.6.7", "5.6.7.8")
	}, []string{"1.2.3.4", "2.3.4.5"})

	assert.Len(t, discoveredPeers, 4)

	hosts := getHosts(discoveredPeers)
	assert.Contains(t, hosts, "3.4.5.6")
	assert.Contains(t, hosts, "4.5.6.7")
	assert.Contains(t, hosts, "5.6.7.8")
	assert.Contains(t, hosts, "6.7.8.9")
}

func TestPublicPeerDiscovery_ExcludePeers(t *testing.T) {
	createNodeFunc := func(host string) (*Node, error) {
		return createTestNodeWithPeers(host, []string{"1.2.3.4", "6.6.6.6"}), nil // 6.6.6.6 excluded
	}
	discovery := newPublicPeerDiscovery(createNodeFunc, []string{" 6.6.6.6"}, time.Hour)

	discoveredPeers := discovery.FindNewPeers([]*Node{
		createTestNodeWithPeers("1.2.3.4", []string{"2.3.4.5", "3.4.5.6"}), // 3.4.5.6 new peer
	}, []string{"1.2.3.4", "2.3.4.5"})

	assert.Len(t, discoveredPeers, 1)
	hosts := getHosts(discoveredPeers)
	assert.Contains(t, hosts, "3.4.5.6")
}

func TestPublicPeerDiscovery_CleanupPeers(t *testing.T) {
	createNodeFunc := func(host string) (*Node, error) {
		return nil, nil
	}
	discovery := newPublicPeerDiscovery(createNodeFunc, []string{}, time.Nanosecond)
	time.Sleep(time.Millisecond)
	unhealthy := discovery.CleanupPeers([]*Node{}, []string{"2.3.4.5", "3.4.5.6"})
	assert.Len(t, unhealthy, 2)
	assert.Contains(t, unhealthy, "2.3.4.5")
	assert.Contains(t, unhealthy, "3.4.5.6")

	unhealthy = discovery.CleanupPeers([]*Node{createTestNode("2.3.4.5")}, []string{"2.3.4.5", "3.4.5.6"})
	assert.Len(t, unhealthy, 1)
	assert.Contains(t, unhealthy, "3.4.5.6")
}

func getHosts(discoveredPeers []*Node) []string {
	hosts := make([]string, len(discoveredPeers))
	for _, node := range discoveredPeers {
		hosts = append(hosts, node.Address)
	}
	return hosts
}
