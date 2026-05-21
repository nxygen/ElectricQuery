package config

import (
	"os"
	"testing"
)

func TestParseRawConfigSupportsHOCONComments(t *testing.T) {
	raw := []byte(`
// service settings
app {
  host = "127.0.0.1"
  port = 9090
  jwt_secret = "01234567890123456789012345678901"
}

# log settings
log {
  level = "debug"
  path = "logs/test.log"
  console = true
}

database {
  driver = "mysql"
  mysql {
    host = "db.example.com"
    port = 3307
    dbname = "electricquery_test"
  }
}

power_checker {
  login_url = "http://ydgl.xzcit.cn/web/Default.aspx"
  user_agent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"
}
`)

	cfg, err := parseRawConfig(raw)
	if err != nil {
		t.Fatalf("parseRawConfig() error = %v", err)
	}

	if cfg.App.Host != "127.0.0.1" || cfg.App.Port != 9090 {
		t.Fatalf("unexpected app config: %+v", cfg.App)
	}
	if cfg.Log.Level != "debug" || cfg.Log.Console != true {
		t.Fatalf("unexpected log config: %+v", cfg.Log)
	}
	if cfg.Database.Driver != "mysql" || cfg.Database.MySQL.Host != "db.example.com" || cfg.Database.MySQL.Port != 3307 {
		t.Fatalf("unexpected database config: %+v", cfg.Database)
	}
	if cfg.PowerChecker.UserAgent != "Mozilla/5.0 (Windows NT 10.0; Win64; x64)" {
		t.Fatalf("unexpected user_agent: %q", cfg.PowerChecker.UserAgent)
	}
}

func TestStripHOCONCommentsKeepsURLs(t *testing.T) {
	raw := []byte(`{
  // top-level comment
  "power_checker": {
    "login_url": "http://ydgl.xzcit.cn/web/Default.aspx", # inline comment
    "user_agent": "Mozilla/5.0"
  }
}`)

	cfg, err := parseRawConfig(raw)
	if err != nil {
		t.Fatalf("parseRawConfig() error = %v", err)
	}

	if cfg.PowerChecker.LoginURL != "http://ydgl.xzcit.cn/web/Default.aspx" {
		t.Fatalf("unexpected login_url: %q", cfg.PowerChecker.LoginURL)
	}
}

func TestParseRawConfigKeepsJSONCompatibility(t *testing.T) {
	raw := []byte(`{
  "app": {
    "host": "0.0.0.0",
    "port": 8080,
    "jwt_secret": "01234567890123456789012345678901"
  },
  "log": {
    "level": "info",
    "path": "logs/app.log",
    "console": true
  },
  "database": {
    "driver": "sqlite",
    "sqlite": {
      "path": "data/electricquery.db"
    }
  }
}`)

	cfg, err := parseRawConfig(raw)
	if err != nil {
		t.Fatalf("parseRawConfig() error = %v", err)
	}

	if cfg.App.Host != "0.0.0.0" || cfg.Database.SQLite.Path != "data/electricquery.db" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestExampleConfigParsesAsHOCON(t *testing.T) {
	raw, err := os.ReadFile("../../application.conf.example")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	cfg, err := parseRawConfig(raw)
	if err != nil {
		t.Fatalf("parseRawConfig() error = %v", err)
	}

	if cfg.App.Port != 8080 || cfg.Database.SQLite.Path != "data/electricquery.db" {
		t.Fatalf("unexpected example config: %+v", cfg)
	}
}
