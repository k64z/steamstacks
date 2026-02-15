package steamclient

import (
	"testing"
)

func TestParseCMList(t *testing.T) {
	fixture := `{
		"response": {
			"serverlist": [
				{"endpoint": "ext1-ord1.steamserver.net:27017", "type": "netfilter"},
				{"endpoint": "ext1-ord1.steamserver.net:443", "type": "websockets"},
				{"endpoint": "ext2-iad1.steamserver.net:27017", "type": "netfilter"},
				{"endpoint": "ext2-iad1.steamserver.net:443", "type": "websockets"}
			],
			"success": true,
			"message": ""
		}
	}`

	servers, err := parseCMList([]byte(fixture))
	if err != nil {
		t.Fatalf("parseCMList: %v", err)
	}

	if len(servers) != 4 {
		t.Fatalf("expected 4 servers, got %d", len(servers))
	}

	// Check types
	wsCount := 0
	tcpCount := 0
	for _, s := range servers {
		switch s.Type {
		case "websockets":
			wsCount++
		case "netfilter":
			tcpCount++
		}
	}

	if wsCount != 2 {
		t.Errorf("expected 2 websocket servers, got %d", wsCount)
	}
	if tcpCount != 2 {
		t.Errorf("expected 2 netfilter servers, got %d", tcpCount)
	}
}

func TestParseCMListEmpty(t *testing.T) {
	fixture := `{"response": {"serverlist": []}}`

	_, err := parseCMList([]byte(fixture))
	if err == nil {
		t.Error("expected error for empty server list")
	}
}

func TestParseCMListInvalidJSON(t *testing.T) {
	_, err := parseCMList([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
