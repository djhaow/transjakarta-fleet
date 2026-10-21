package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "# comment\n\nMQTT_BROKER=tcp://localhost:1883\nRABBITMQ_URL = \"amqp://guest:guest@localhost:5672/\"\nbroken line without equals\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MQTT_BROKER", "tcp://override:1883")
	t.Cleanup(func() { os.Unsetenv("RABBITMQ_URL") })

	applyEnvFile(path)

	if got := os.Getenv("MQTT_BROKER"); got != "tcp://override:1883" {
		t.Errorf("existing variable overwritten: %q", got)
	}
	if got := os.Getenv("RABBITMQ_URL"); got != "amqp://guest:guest@localhost:5672/" {
		t.Errorf("RABBITMQ_URL = %q, want quotes stripped", got)
	}
}

func TestApplyEnvFileMissing(t *testing.T) {
	applyEnvFile(filepath.Join(t.TempDir(), "no-such-file"))
}
