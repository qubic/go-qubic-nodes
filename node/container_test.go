package node

import (
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMaxTick(t *testing.T) {

	testData := []struct {
		name      string
		nodes     []*Node
		threshold uint32
		want      uint32
	}{
		{
			name: "TestMaxTick_last_element",
			nodes: []*Node{
				{
					LastTick: 1000,
				},
				{
					LastTick: 1021,
				},
				{
					LastTick: 1023,
				},
				{
					LastTick: 1023,
				},
				{
					LastTick: 1753,
				},
				{
					LastTick: 1800,
				}, {
					LastTick: 100,
				},
				{
					LastTick: 1945,
				},
				{
					LastTick: 2000,
				}, {
					LastTick: 3000,
				},
			},
			want:      3000,
			threshold: 50,
		},
		{
			name: "TestMaxTick_element_in_middle_of_slice",
			nodes: []*Node{
				{
					LastTick: 1021,
				},
				{
					LastTick: 1000,
				},
				{
					LastTick: 1023,
				},
				{
					LastTick: 1023,
				},
				{
					LastTick: 2500,
				},
				{
					LastTick: 5700,
				}, {
					LastTick: 100,
				},
				{
					LastTick: 1945,
				},
				{
					LastTick: 2000,
				}, {
					LastTick: 3000,
				},
			},
			want:      5700,
			threshold: 50,
		},
		{
			name: "TestMaxTick_one_element",
			nodes: []*Node{
				{
					LastTick: 1000,
				},
			},
			want:      1000,
			threshold: 50,
		},
		{
			name: "TestMaxTick_first_element",
			nodes: []*Node{
				{
					LastTick: 1500,
				},
				{
					LastTick: 1000,
				},
			},
			want:      1500,
			threshold: 50,
		},
		{
			name:      "TestMaxTick_no_elements",
			nodes:     []*Node{},
			want:      0,
			threshold: 50,
		},
	}

	for _, test := range testData {
		t.Run(test.name, func(t *testing.T) {
			got := calculateMaxTick(test.nodes, test.threshold)
			if got != test.want {
				t.Fatalf("Want: %d Got: %d\n", test.want, got)
			}
		})
	}

}

func TestReliableNodes(t *testing.T) {

	testData := []struct {
		name                  string
		nodes                 []*Node
		maxTickThreshold      uint32
		reliableRange         uint32
		expectedReliableNodes []*Node
	}{
		{
			name: "TestReliableNodes_only_one",
			nodes: []*Node{
				{
					LastTick: 1992,
				},
				{
					LastTick: 1993,
				},
				{
					LastTick: 1994,
				},
				{
					LastTick: 1995,
				},
				{
					LastTick: 1996,
				},
				{
					LastTick: 1997,
				}, {
					LastTick: 1998,
				},
				{
					LastTick: 1999,
				},
				{
					LastTick: 2000,
				}, {
					LastTick: 2050,
				},
			},
			expectedReliableNodes: []*Node{
				{
					LastTick: 2050,
				},
			},
			maxTickThreshold: 50,
			reliableRange:    5,
		},
		{
			name: "TestReliableNodes_3_elements",
			nodes: []*Node{
				{
					LastTick: 2049,
				},
				{
					LastTick: 2048,
				},
				{
					LastTick: 2050,
				},
				{
					LastTick: 2044,
				},
			},
			expectedReliableNodes: []*Node{
				{
					LastTick: 2048,
				},
				{
					LastTick: 2049,
				},
				{
					LastTick: 2050,
				},
			},
			maxTickThreshold: 50,
			reliableRange:    5,
		},
		{
			name:                  "TestReliableNodes_no_elements",
			nodes:                 []*Node{},
			expectedReliableNodes: []*Node{},
			maxTickThreshold:      50,
			reliableRange:         5,
		},
	}
	for _, test := range testData {

		t.Run(test.name, func(t *testing.T) {

			maxTick := calculateMaxTick(test.nodes, test.maxTickThreshold)
			minTick := maxTick - test.reliableRange

			reliableNodes, _ := getReliableNodes(test.nodes, maxTick, minTick)

			diff := cmp.Diff(reliableNodes, test.expectedReliableNodes)
			require.Empty(t, diff)

			for _, node := range reliableNodes {
				if node.LastTick < minTick || node.LastTick > maxTick {
					t.Fatalf("Tick not in the expected range: [%d - %d] Got: %d", minTick, maxTick, node.LastTick)
				}
			}

		})

	}
}

func TestContainer_GetReliableNodesWithMinimumTick(t *testing.T) {
	testData := []struct {
		name                  string
		nodes                 []*Node
		minimumTick           uint32
		expectedReliableNodes []*Node
	}{
		{
			name: "TestContainer_GetReliableNodesWithMinimumTick_1_node_below_minimum",
			nodes: []*Node{
				{
					LastTick: 1992,
				},
			},
			minimumTick:           1993,
			expectedReliableNodes: []*Node{},
		},
		{
			name: "TestContainer_GetReliableNodesWithMinimumTick_2_node_below_minimum",
			nodes: []*Node{
				{
					LastTick: 1992,
				},
				{
					LastTick: 1991,
				},
			},
			minimumTick:           1993,
			expectedReliableNodes: []*Node{},
		},
		{
			name: "TestContainer_GetReliableNodesWithMinimumTick_1_node_above_minimum",
			nodes: []*Node{
				{
					LastTick: 1992,
				},
				{
					LastTick: 1991,
				},
				{
					LastTick: 1994,
				},
			},
			minimumTick: 1993,
			expectedReliableNodes: []*Node{
				{
					LastTick: 1994,
				},
			},
		},
		{
			name: "TestContainer_GetReliableNodesWithMinimumTick_1_node_equal_minimum",
			nodes: []*Node{
				{
					LastTick: 1992,
				},
				{
					LastTick: 1991,
				},
				{
					LastTick: 1993,
				},
			},
			minimumTick: 1993,
			expectedReliableNodes: []*Node{
				{
					LastTick: 1993,
				},
			},
		},
		{
			name: "TestContainer_GetReliableNodesWithMinimumTick_2_node_equal_and_above_minimum",
			nodes: []*Node{
				{
					LastTick: 1992,
				},
				{
					LastTick: 1991,
				},
				{
					LastTick: 1993,
				},
				{
					LastTick: 1994,
				},
			},
			minimumTick: 1993,
			expectedReliableNodes: []*Node{
				{
					LastTick: 1993,
				},
				{
					LastTick: 1994,
				},
			},
		},
	}

	for _, test := range testData {
		t.Run(test.name, func(t *testing.T) {
			container := &Container{}
			container.ReliableNodes = test.nodes
			reliableNodesAtMinimumTick := container.GetReliableNodesWithMinimumTick(test.minimumTick)
			diff := cmp.Diff(reliableNodesAtMinimumTick, test.expectedReliableNodes)
			require.Empty(t, diff)
		})
	}
}
