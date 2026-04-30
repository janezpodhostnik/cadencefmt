package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/janezpodhostnik/cadencefmt/internal/format"
	toml "github.com/pelletier/go-toml/v2"
)

const FileName = ".cadencefmt.toml"

// Config holds formatting options parsed from a TOML config file.
// All fields are pointers so unset values can be distinguished from zero values.
type Config struct {
	LineWidth       *int    `toml:"line_width"`
	IndentCharacter *string `toml:"indent_character"`
	IndentCount     *int    `toml:"indent_count"`
	SortImports     *bool   `toml:"sort_imports"`
	StripSemicolons *bool   `toml:"strip_semicolons"`
	KeepBlankLines  *int    `toml:"keep_blank_lines"`
}

// Lookup walks up from startDir looking for a .cadencefmt.toml file.
// Returns the parsed config and the path where it was found.
// If no config file is found, returns a zero Config and empty path.
func Lookup(startDir string) (Config, string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return Config{}, "", err
	}
	for {
		path := filepath.Join(dir, FileName)
		if _, err := os.Stat(path); err == nil {
			cfg, err := ParseFile(path)
			return cfg, path, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Config{}, "", nil
		}
		dir = parent
	}
}

// ParseFile reads and parses a TOML config file.
func ParseFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}
	return Parse(data)
}

// Parse parses TOML bytes into a Config.
func Parse(data []byte) (Config, error) {
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}

// Apply merges non-nil config fields onto an Options struct.
// Fields that are nil (not set in config) keep their existing values.
func (c Config) Apply(opts format.Options) format.Options {
	if c.LineWidth != nil {
		opts.LineWidth = *c.LineWidth
	}
	if c.IndentCharacter != nil {
		opts.IndentCharacter = *c.IndentCharacter
	}
	if c.IndentCount != nil {
		opts.IndentCount = *c.IndentCount
	}
	if c.SortImports != nil {
		opts.SortImports = *c.SortImports
	}
	if c.StripSemicolons != nil {
		opts.StripSemicolons = *c.StripSemicolons
	}
	if c.KeepBlankLines != nil {
		opts.KeepBlankLines = *c.KeepBlankLines
	}
	return opts
}
