package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SarawinT/user-management-api/config"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const validSecret = "0123456789abcdef0123456789abcdef"

func TestLoad_FileValues(t *testing.T) {
	path := writeFile(t, `
server:
  http_port: 9000
  grpc_port: 9001
  grpc_reflection: false
mongo:
  uri: mongodb://db:27017
jwt:
  secret: `+validSecret+`
  ttl: 15m
`)
	cfg, err := config.InitialConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.HTTPPort != 9000 || cfg.Server.GRPCPort != 9001 || cfg.Server.GRPCReflection {
		t.Fatalf("server: %+v", cfg.Server)
	}
	if cfg.Mongo.URI != "mongodb://db:27017" {
		t.Fatalf("mongo: %+v", cfg.Mongo)
	}
	if cfg.JWT.TTL != 15*time.Minute || cfg.JWT.Secret != validSecret {
		t.Fatalf("jwt: %+v", cfg.JWT)
	}
	// Keys absent from the file fall back to defaults.
	if cfg.Mongo.Database != "user_management" || cfg.Log.Level != "info" || cfg.JWT.Issuer != "user-management-api" {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	path := writeFile(t, "jwt:\n  secret: "+validSecret+"\nserver:\n  http_port: 9000\n")
	t.Setenv("JWT_SECRET", "env-secret-env-secret-env-secret-env")
	t.Setenv("SERVER_HTTP_PORT", "7000")
	t.Setenv("MONGO_DATABASE", "from_env") // key not in file: default overridden

	cfg, err := config.InitialConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWT.Secret != "env-secret-env-secret-env-secret-env" || cfg.Server.HTTPPort != 7000 || cfg.Mongo.Database != "from_env" {
		t.Fatalf("env overrides not applied: %+v", cfg)
	}
}

func TestLoad_Errors(t *testing.T) {
	tests := map[string]struct {
		content string
		want    []string
	}{
		"missing secret": {
			content: "server:\n  http_port: 8080\n",
			want:    []string{"jwt.secret is required"},
		},
		"bad duration": {
			content: "jwt:\n  secret: " + validSecret + "\n  ttl: soon\n",
			want:    []string{"parse config"},
		},
		"all problems reported together": {
			content: "mongo:\n  database: \"\"\nserver:\n  http_port: 70000\n",
			want:    []string{"jwt.secret is required", "mongo.database", "server.http_port"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := config.InitialConfig(writeFile(t, tt.content))
			if err == nil {
				t.Fatal("expected error")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not mention %q", err, w)
				}
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		_, err := config.InitialConfig(filepath.Join(t.TempDir(), "nope.yaml"))
		if err == nil || !strings.Contains(err.Error(), "config.example.yaml") {
			t.Fatalf("got %v", err)
		}
	})
}
