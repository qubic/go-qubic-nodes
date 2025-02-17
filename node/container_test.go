package node

import (
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
			name: "TestMaxTick_1",
			nodes: []*Node{
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1021,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1023,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1023,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1753,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1800,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				}, {
					Address:           "",
					Peers:             nil,
					LastTick:          100,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1945,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          2000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				}, {
					Address:           "",
					Peers:             nil,
					LastTick:          3000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
			},
			want:      3000,
			threshold: 50,
		},
		{
			name: "TestMaxTick_2",
			nodes: []*Node{
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1021,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1023,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1023,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1753,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1800,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				}, {
					Address:           "",
					Peers:             nil,
					LastTick:          100,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1945,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          2000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				}, {
					Address:           "",
					Peers:             nil,
					LastTick:          2050,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
			},
			want:      2050,
			threshold: 50,
		},
		{
			name: "TestMaxTick_3",
			nodes: []*Node{
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1021,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1023,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1023,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1753,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1800,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				}, {
					Address:           "",
					Peers:             nil,
					LastTick:          100,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1945,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          2000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				}, {
					Address:           "",
					Peers:             nil,
					LastTick:          2049,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
			},
			want:      2049,
			threshold: 50,
		},
		{
			name: "TestMaxTick_4",
			nodes: []*Node{
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1021,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1023,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1023,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          2500,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          5700,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				}, {
					Address:           "",
					Peers:             nil,
					LastTick:          100,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1945,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          2000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				}, {
					Address:           "",
					Peers:             nil,
					LastTick:          3000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
			},
			want:      5700,
			threshold: 50,
		},
		{
			name: "TestMaxTick_5",
			nodes: []*Node{
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
			},
			want:      1000,
			threshold: 50,
		},
		{
			name: "TestMaxTick_6",
			nodes: []*Node{
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1500,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
			},
			want:      1500,
			threshold: 50,
		},
		{
			name:      "TestMaxTick_7",
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
		name             string
		nodes            []*Node
		maxTickThreshold uint32
		reliableRange    uint32
		expectedLength   int
	}{
		{
			name: "TestReliableNodes_1",
			nodes: []*Node{
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1992,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1993,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1994,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1995,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1996,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1997,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				}, {
					Address:           "",
					Peers:             nil,
					LastTick:          1998,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1999,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          2000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				}, {
					Address:           "",
					Peers:             nil,
					LastTick:          2050,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
			},
			expectedLength:   6,
			maxTickThreshold: 50,
			reliableRange:    5,
		},
		{
			name: "TestReliableNodes_2",
			nodes: []*Node{
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1992,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1993,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1994,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1995,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1996,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1997,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				}, {
					Address:           "",
					Peers:             nil,
					LastTick:          1998,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          1999,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
				{
					Address:           "",
					Peers:             nil,
					LastTick:          2000,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				}, {
					Address:           "",
					Peers:             nil,
					LastTick:          2049,
					LastUpdate:        0,
					LastUpdateSuccess: true,
				},
			},
			expectedLength:   1,
			maxTickThreshold: 50,
			reliableRange:    5,
		},
	}
	for _, test := range testData {

		t.Run(test.name, func(t *testing.T) {

			maxTick := calculateMaxTick(test.nodes, test.maxTickThreshold)
			minTick := maxTick - test.reliableRange

			reliableNodes, _ := getReliableNodes(test.nodes, maxTick, minTick)

			if len(reliableNodes) != test.expectedLength {
				t.Fatalf("Expected %d reliable nodes. Got: %d", test.expectedLength, len(reliableNodes))
			}

			for _, node := range reliableNodes {
				if node.LastTick < minTick || node.LastTick > maxTick {
					t.Fatalf("Tick not in the expected range: [%d - %d] Got: %d", minTick, maxTick, node.LastTick)
				}
			}

		})

	}
}
