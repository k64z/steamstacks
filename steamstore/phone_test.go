package steamstore

import (
	"os"
	"testing"
)

func TestParsePhoneInfo(t *testing.T) {
	data, err := os.ReadFile("testdata/account.html")
	if err != nil {
		t.Skipf("testdata/account.html not found: %v", err)
	}

	info, err := parsePhoneInfo(string(data))
	if err != nil {
		t.Fatalf("failed to parse phone info: %v", err)
	}

	if !info.HasPhone {
		t.Error("expected HasPhone to be true")
	}

	if info.PhoneEndingWith == "" {
		t.Error("expected PhoneEndingWith to be non-empty")
	}

	t.Logf("Phone ends with: %s", info.PhoneEndingWith)
}

func TestParsePhoneInfoNoPhone(t *testing.T) {
	html := `<html><body>No phone here</body></html>`

	info, err := parsePhoneInfo(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.HasPhone {
		t.Error("expected HasPhone to be false")
	}
}

func TestParsePhoneInfoVariousFormats(t *testing.T) {
	tests := []struct {
		name        string
		html        string
		hasPhone    bool
		endingWith  string
	}{
		{
			name: "standard format",
			html: `<div class="phone_header_description">
				Phone:
				<img src="icon.png" alt="" >
				<span class="account_data_field">Ends in 79</span>
			</div>`,
			hasPhone:   true,
			endingWith: "79",
		},
		{
			name: "different digits",
			html: `<div class="phone_header_description">
				Phone:
				<img src="icon.png" alt="" >
				<span class="account_data_field">Ends in 1234</span>
			</div>`,
			hasPhone:   true,
			endingWith: "1234",
		},
		{
			name:       "no phone section",
			html:       `<html><body>Account page without phone</body></html>`,
			hasPhone:   false,
			endingWith: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := parsePhoneInfo(tt.html)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if info.HasPhone != tt.hasPhone {
				t.Errorf("expected HasPhone=%v, got %v", tt.hasPhone, info.HasPhone)
			}

			if info.PhoneEndingWith != tt.endingWith {
				t.Errorf("expected PhoneEndingWith=%q, got %q", tt.endingWith, info.PhoneEndingWith)
			}
		})
	}
}
