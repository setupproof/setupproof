package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Path     string
	Version  int
	Defaults Defaults
	Files    []string
	Env      Env
	Blocks   []Block
}

type Defaults struct {
	Runner        *string
	Image         *string
	Timeout       *string
	RequireBlocks *bool
	Strict        *bool
	Isolated      *bool
	Network       *bool
}

type Env struct {
	Allow []string
	Pass  []EnvPass
}

type EnvPass struct {
	Name     string
	Secret   *bool
	Required *bool
}

type Block struct {
	File     string
	ID       string
	Runner   *string
	Image    *string
	Timeout  *string
	Strict   *bool
	Isolated *bool
	Network  *bool
}

func Load(path string) (*Config, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, err := Parse(contents)
	if err != nil {
		return nil, err
	}
	cfg.Path = path
	return cfg, nil
}

func Parse(contents []byte) (*Config, error) {
	parser := yamlParser{}
	return parser.parse(string(contents))
}

type yamlParser struct {
	cfg        Config
	section    string
	subsection string
	block      *Block
	pass       *EnvPass
}

func (p *yamlParser) parse(contents string) (*Config, error) {
	lines := strings.Split(contents, "\n")
	for index, raw := range lines {
		lineNo := index + 1
		line := stripInlineComment(strings.TrimSuffix(raw, "\r"))
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, "\t") {
			return nil, fmt.Errorf("line %d: tabs are not supported in config indentation", lineNo)
		}

		indent := leadingSpaces(line)
		text := strings.TrimSpace(line)
		if indent == 0 {
			if err := p.parseTopLevel(lineNo, text); err != nil {
				return nil, err
			}
			continue
		}

		if p.section == "" {
			return nil, fmt.Errorf("line %d: nested field without a section", lineNo)
		}

		switch p.section {
		case "defaults":
			if indent != 2 {
				return nil, fmt.Errorf("line %d: defaults fields must use two-space indentation", lineNo)
			}
			if err := p.parseDefaults(lineNo, text); err != nil {
				return nil, err
			}
		case "files":
			if indent != 2 {
				return nil, fmt.Errorf("line %d: files entries must use two-space indentation", lineNo)
			}
			value, ok := listValue(text)
			if !ok {
				return nil, fmt.Errorf("line %d: files entries must be list items", lineNo)
			}
			p.cfg.Files = append(p.cfg.Files, unquote(value))
		case "env":
			if err := p.parseEnv(lineNo, indent, text); err != nil {
				return nil, err
			}
		case "blocks":
			if err := p.parseBlocks(lineNo, indent, text); err != nil {
				return nil, err
			}
		case "x":
			continue
		default:
			return nil, fmt.Errorf("line %d: unsupported section %q", lineNo, p.section)
		}
	}

	if p.cfg.Version == 0 {
		return nil, fmt.Errorf("config version is required")
	}
	if p.cfg.Version != 1 {
		return nil, fmt.Errorf("unsupported config version %d", p.cfg.Version)
	}
	return &p.cfg, nil
}

func (p *yamlParser) parseTopLevel(lineNo int, text string) error {
	key, value, ok := keyValue(text)
	if !ok {
		return fmt.Errorf("line %d: expected key: value", lineNo)
	}

	p.section = ""
	p.subsection = ""
	p.block = nil
	p.pass = nil

	switch key {
	case "version":
		if value == "" {
			return fmt.Errorf("line %d: version requires a value", lineNo)
		}
		version, err := strconv.Atoi(unquote(value))
		if err != nil {
			return fmt.Errorf("line %d: version must be an integer", lineNo)
		}
		p.cfg.Version = version
	case "defaults", "files", "env", "blocks":
		if value != "" {
			return fmt.Errorf("line %d: %s must be a section", lineNo, key)
		}
		p.section = key
	default:
		if strings.HasPrefix(key, "x-") {
			p.section = "x"
			return nil
		}
		return fmt.Errorf("line %d: unknown top-level field %q", lineNo, key)
	}
	return nil
}

func (p *yamlParser) parseDefaults(lineNo int, text string) error {
	if _, ok := listValue(text); ok {
		return fmt.Errorf("line %d: defaults fields must use key: value, not list items", lineNo)
	}
	key, value, ok := keyValue(text)
	if !ok || value == "" {
		return fmt.Errorf("line %d: defaults field requires key: value", lineNo)
	}
	switch key {
	case "runner":
		p.cfg.Defaults.Runner = stringPointer(unquote(value))
	case "image":
		p.cfg.Defaults.Image = stringPointer(unquote(value))
	case "timeout":
		p.cfg.Defaults.Timeout = stringPointer(unquote(value))
	case "requireBlocks":
		parsed, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("line %d: %v", lineNo, err)
		}
		p.cfg.Defaults.RequireBlocks = &parsed
	case "strict":
		parsed, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("line %d: %v", lineNo, err)
		}
		p.cfg.Defaults.Strict = &parsed
	case "isolated":
		parsed, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("line %d: %v", lineNo, err)
		}
		p.cfg.Defaults.Isolated = &parsed
	case "network":
		parsed, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("line %d: %v", lineNo, err)
		}
		p.cfg.Defaults.Network = &parsed
	default:
		return fmt.Errorf("line %d: unknown defaults field %q", lineNo, key)
	}
	return nil
}

func (p *yamlParser) parseEnv(lineNo int, indent int, text string) error {
	if indent == 2 {
		key, value, ok := keyValue(text)
		if !ok || value != "" {
			return fmt.Errorf("line %d: env fields must be sections", lineNo)
		}
		switch key {
		case "allow", "pass":
			p.subsection = key
			p.pass = nil
			return nil
		default:
			return fmt.Errorf("line %d: unknown env field %q", lineNo, key)
		}
	}

	if p.subsection == "" {
		return fmt.Errorf("line %d: env entry without allow or pass section", lineNo)
	}
	switch p.subsection {
	case "allow":
		if indent != 4 {
			return fmt.Errorf("line %d: env.allow entries must use four-space indentation", lineNo)
		}
		value, ok := listValue(text)
		if !ok {
			return fmt.Errorf("line %d: env.allow entries must be list items", lineNo)
		}
		p.cfg.Env.Allow = append(p.cfg.Env.Allow, unquote(value))
	case "pass":
		if indent == 4 {
			value, ok := listValue(text)
			if !ok {
				return fmt.Errorf("line %d: env.pass entries must be list items", lineNo)
			}
			pass := EnvPass{}
			if value != "" {
				if err := assignEnvPass(&pass, lineNo, value); err != nil {
					return err
				}
			}
			p.cfg.Env.Pass = append(p.cfg.Env.Pass, pass)
			p.pass = &p.cfg.Env.Pass[len(p.cfg.Env.Pass)-1]
			return nil
		}
		if indent != 6 || p.pass == nil {
			return fmt.Errorf("line %d: env.pass fields must use six-space indentation", lineNo)
		}
		return assignEnvPass(p.pass, lineNo, text)
	}
	return nil
}

func (p *yamlParser) parseBlocks(lineNo int, indent int, text string) error {
	if indent == 2 {
		value, ok := listValue(text)
		if !ok {
			if key, _, hasKey := keyValue(text); hasKey {
				return fmt.Errorf("line %d: blocks entries must start with \"- \"; found field %q without a block list item", lineNo, key)
			}
			return fmt.Errorf("line %d: blocks entries must be list items", lineNo)
		}
		block := Block{}
		if value != "" {
			if err := assignBlock(&block, lineNo, value); err != nil {
				return err
			}
		}
		p.cfg.Blocks = append(p.cfg.Blocks, block)
		p.block = &p.cfg.Blocks[len(p.cfg.Blocks)-1]
		return nil
	}

	if indent != 4 || p.block == nil {
		return fmt.Errorf("line %d: block fields must use four-space indentation", lineNo)
	}
	return assignBlock(p.block, lineNo, text)
}

func assignEnvPass(pass *EnvPass, lineNo int, text string) error {
	key, value, ok := keyValue(text)
	if !ok || value == "" {
		return fmt.Errorf("line %d: env.pass field requires key: value", lineNo)
	}
	switch key {
	case "name":
		pass.Name = unquote(value)
	case "secret":
		parsed, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("line %d: %v", lineNo, err)
		}
		pass.Secret = &parsed
	case "required":
		parsed, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("line %d: %v", lineNo, err)
		}
		pass.Required = &parsed
	default:
		return fmt.Errorf("line %d: unknown env.pass field %q", lineNo, key)
	}
	return nil
}

func assignBlock(block *Block, lineNo int, text string) error {
	key, value, ok := keyValue(text)
	if !ok || value == "" {
		return fmt.Errorf("line %d: block field requires key: value", lineNo)
	}
	switch key {
	case "file":
		block.File = unquote(value)
	case "id":
		block.ID = unquote(value)
	case "runner":
		block.Runner = stringPointer(unquote(value))
	case "image":
		block.Image = stringPointer(unquote(value))
	case "timeout":
		block.Timeout = stringPointer(unquote(value))
	case "strict":
		parsed, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("line %d: %v", lineNo, err)
		}
		block.Strict = &parsed
	case "isolated":
		parsed, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("line %d: %v", lineNo, err)
		}
		block.Isolated = &parsed
	case "network":
		parsed, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("line %d: %v", lineNo, err)
		}
		block.Network = &parsed
	default:
		return fmt.Errorf("line %d: unknown block field %q", lineNo, key)
	}
	return nil
}

func keyValue(text string) (string, string, bool) {
	key, value, ok := strings.Cut(text, ":")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

func listValue(text string) (string, bool) {
	if !strings.HasPrefix(text, "-") {
		return "", false
	}
	if len(text) > 1 && text[1] != ' ' {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(text, "-")), true
}

func parseBool(value string) (bool, error) {
	switch unquote(value) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("boolean value must be true or false")
	}
}

func unquote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}

func stringPointer(value string) *string {
	return &value
}

func leadingSpaces(line string) int {
	count := 0
	for count < len(line) && line[count] == ' ' {
		count++
	}
	return count
}

func stripInlineComment(line string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
				return strings.TrimRight(line[:i], " \t")
			}
		}
	}
	return line
}
