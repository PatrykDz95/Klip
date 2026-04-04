package p2p

import "testing"

func TestExtractField(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		key    string
		want   string
	}{
		{
			name:   "finds existing key",
			fields: []string{"x=1", "device_id=abc123", "y=2"},
			key:    "device_id",
			want:   "abc123",
		},
		{
			name:   "returns empty for missing key",
			fields: []string{"x=1", "y=2"},
			key:    "device_id",
			want:   "",
		},
		{
			name:   "does not match prefix collisions",
			fields: []string{"device_id_backup=abc", "device_id=real"},
			key:    "device_id",
			want:   "real",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractField(tt.fields, tt.key)
			if got != tt.want {
				t.Fatalf("extractField() = %q, want %q", got, tt.want)
			}
		})
	}
}
