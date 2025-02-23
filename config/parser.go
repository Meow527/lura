// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Parser reads a configuration file, parses it and returns the content as an init ServiceConfig struct
type Parser interface {
	Parse(configFile string) (ServiceConfig, error)
}

// ParserFunc type is an adapter to allow the use of ordinary functions as subscribers.
// If f is a function with the appropriate signature, ParserFunc(f) is a Parser that calls f.
type ParserFunc func(string) (ServiceConfig, error)

// Parse implements the Parser interface
func (f ParserFunc) Parse(configFile string) (ServiceConfig, error) { return f(configFile) }

// NewParser creates a new parser using the json library
func NewParser() Parser {
	return NewParserWithFileReader(os.ReadFile)
}

// NewParserWithFileReader returns a Parser with the injected FileReaderFunc function
func NewParserWithFileReader(f FileReaderFunc) Parser {
	return parser{fileReader: f}
}

type parser struct {
	fileReader FileReaderFunc
}

// Parser implements the Parse interface
func (p parser) Parse(configFile string) (ServiceConfig, error) {
	var result ServiceConfig
	var cfg parseableServiceConfig
	data, err := p.fileReader(configFile)
	if err != nil {
		return result, CheckErr(err, configFile)
	}
	if err = json.Unmarshal(data, &cfg); err != nil {
		return result, CheckErr(err, configFile)
	}
	result = cfg.normalize()

	if err = result.Init(); err != nil {
		return result, CheckErr(err, configFile)
	}

	return result, nil
}

// CheckErr returns a proper documented error
func CheckErr(err error, configFile string) error {
	switch e := err.(type) {
	case *json.SyntaxError:
		return NewParseError(err, configFile, int(e.Offset))
	case *json.UnmarshalTypeError:
		return NewParseError(err, configFile, int(e.Offset))
	case *os.PathError:
		return fmt.Errorf(
			"'%s' (%s): %s",
			configFile,
			e.Op,
			e.Err.Error(),
		)
	default:
		return fmt.Errorf("'%s': %v", configFile, err)
	}
}

// NewParseError returns a new ParseError
func NewParseError(err error, configFile string, offset int) *ParseError {
	b, _ := os.ReadFile(configFile)
	row, col := getErrorRowCol(b, offset)
	return &ParseError{
		ConfigFile: configFile,
		Err:        err,
		Offset:     offset,
		Row:        row,
		Col:        col,
	}
}

func getErrorRowCol(source []byte, offset int) (row, col int) {
	if len(source) < offset {
		offset = len(source) - 1
	}
	for i := 0; i < offset; i++ {
		v := source[i]
		if v == '\r' {
			continue
		}
		if v == '\n' {
			col = 0
			row++
			continue
		}
		col++
	}
	return
}

// ParseError is an error containing details regarding the row and column where
// an parse error occurred
type ParseError struct {
	ConfigFile string
	Offset     int
	Row        int
	Col        int
	Err        error
}

// Error returns the error message for the ParseError
func (p *ParseError) Error() string {
	return fmt.Sprintf(
		"'%s': %v, offset: %v, row: %v, col: %v",
		p.ConfigFile,
		p.Err.Error(),
		p.Offset,
		p.Row,
		p.Col,
	)
}

// FileReaderFunc is a function used to read the content of a config file
type FileReaderFunc func(string) ([]byte, error)

type parseableServiceConfig struct {
	Name                  string                     `json:"name"`
	Endpoints             []*parseableEndpointConfig `json:"endpoints"`
	AsyncAgents           []*parseableAsyncAgent     `json:"async_agent"`
	Timeout               string                     `json:"timeout"`
	CacheTTL              string                     `json:"cache_ttl"`
	Host                  []string                   `json:"host"`
	Port                  int                        `json:"port"`
	Version               int                        `json:"version"`
	ExtraConfig           *ExtraConfig               `json:"extra_config,omitempty"`
	ReadTimeout           string                     `json:"read_timeout"`
	WriteTimeout          string                     `json:"write_timeout"`
	IdleTimeout           string                     `json:"idle_timeout"`
	ReadHeaderTimeout     string                     `json:"read_header_timeout"`
	DisableKeepAlives     bool                       `json:"disable_keep_alives"`
	DisableCompression    bool                       `json:"disable_compression"`
	DisableStrictREST     bool                       `json:"disable_rest"`
	MaxIdleConns          int                        `json:"max_idle_connections"`
	MaxIdleConnsPerHost   int                        `json:"max_idle_connections_per_host"`
	IdleConnTimeout       string                     `json:"idle_connection_timeout"`
	ResponseHeaderTimeout string                     `json:"response_header_timeout"`
	ExpectContinueTimeout string                     `json:"expect_continue_timeout"`
	OutputEncoding        string                     `json:"output_encoding"`
	DialerTimeout         string                     `json:"dialer_timeout"`
	DialerFallbackDelay   string                     `json:"dialer_fallback_delay"`
	DialerKeepAlive       string                     `json:"dialer_keep_alive"`
	Debug                 bool                       `json:"debug_endpoint"`
	Echo                  bool                       `json:"echo_endpoint"`
	Plugin                *Plugin                    `json:"plugin,omitempty"`
	TLS                   *parseableTLS              `json:"tls,omitempty"`
}

func (p *parseableServiceConfig) normalize() ServiceConfig {
	cfg := ServiceConfig{
		Name:                  p.Name,
		Timeout:               parseDuration(p.Timeout),
		CacheTTL:              parseDuration(p.CacheTTL),
		Host:                  p.Host,
		Port:                  p.Port,
		Version:               p.Version,
		Debug:                 p.Debug,
		ReadTimeout:           parseDuration(p.ReadTimeout),
		WriteTimeout:          parseDuration(p.WriteTimeout),
		IdleTimeout:           parseDuration(p.IdleTimeout),
		ReadHeaderTimeout:     parseDuration(p.ReadHeaderTimeout),
		DisableKeepAlives:     p.DisableKeepAlives,
		DisableCompression:    p.DisableCompression,
		DisableStrictREST:     p.DisableStrictREST,
		MaxIdleConns:          p.MaxIdleConns,
		MaxIdleConnsPerHost:   p.MaxIdleConnsPerHost,
		IdleConnTimeout:       parseDuration(p.IdleConnTimeout),
		ResponseHeaderTimeout: parseDuration(p.ResponseHeaderTimeout),
		ExpectContinueTimeout: parseDuration(p.ExpectContinueTimeout),
		DialerTimeout:         parseDuration(p.DialerTimeout),
		DialerFallbackDelay:   parseDuration(p.DialerFallbackDelay),
		DialerKeepAlive:       parseDuration(p.DialerKeepAlive),
		OutputEncoding:        p.OutputEncoding,
		Plugin:                p.Plugin,
	}
	if p.TLS != nil {
		cfg.TLS = &TLS{
			IsDisabled:               p.TLS.IsDisabled,
			PublicKey:                p.TLS.PublicKey,
			PrivateKey:               p.TLS.PrivateKey,
			CaCerts:                  p.TLS.CaCerts,
			MinVersion:               p.TLS.MinVersion,
			MaxVersion:               p.TLS.MaxVersion,
			CurvePreferences:         p.TLS.CurvePreferences,
			PreferServerCipherSuites: p.TLS.PreferServerCipherSuites,
			CipherSuites:             p.TLS.CipherSuites,
			EnableMTLS:               p.TLS.EnableMTLS,
			DisableSystemCaPool:      p.TLS.DisableSystemCaPool,
		}
	}
	if p.ExtraConfig != nil {
		cfg.ExtraConfig = *p.ExtraConfig
	}
	endpoints := make([]*EndpointConfig, 0, len(p.Endpoints))
	for _, e := range p.Endpoints {
		endpoints = append(endpoints, e.normalize())
	}
	cfg.Endpoints = endpoints
	agents := make([]*AsyncAgent, 0, len(p.AsyncAgents))
	for _, a := range p.AsyncAgents {
		agents = append(agents, a.normalize())
	}
	cfg.AsyncAgents = agents
	return cfg
}

type parseableTLS struct {
	IsDisabled               bool     `json:"disabled"`
	PublicKey                string   `json:"public_key"`
	PrivateKey               string   `json:"private_key"`
	CaCerts                  []string `json:"ca_certs"`
	MinVersion               string   `json:"min_version"`
	MaxVersion               string   `json:"max_version"`
	CurvePreferences         []uint16 `json:"curve_preferences"`
	PreferServerCipherSuites bool     `json:"prefer_server_cipher_suites"`
	CipherSuites             []uint16 `json:"cipher_suites"`
	EnableMTLS               bool     `json:"enable_mtls"`
	DisableSystemCaPool      bool     `json:"disable_system_ca_pool"`
}

type parseableEndpointConfig struct {
	Endpoint        string              `json:"endpoint"`
	Method          string              `json:"method"`
	Backend         []*parseableBackend `json:"backend"`
	ConcurrentCalls int                 `json:"concurrent_calls"`
	Timeout         string              `json:"timeout"`
	CacheTTL        string              `json:"cache_ttl"`
	QueryString     []string            `json:"input_query_strings"`
	ExtraConfig     *ExtraConfig        `json:"extra_config,omitempty"`
	HeadersToPass   []string            `json:"input_headers"`
	OutputEncoding  string              `json:"output_encoding"`
}

func (p *parseableEndpointConfig) normalize() *EndpointConfig {
	e := EndpointConfig{
		Endpoint:        p.Endpoint,
		Method:          p.Method,
		ConcurrentCalls: p.ConcurrentCalls,
		Timeout:         parseDuration(p.Timeout),
		CacheTTL:        parseDuration(p.CacheTTL),
		QueryString:     p.QueryString,
		HeadersToPass:   p.HeadersToPass,
		OutputEncoding:  p.OutputEncoding,
	}
	if p.ExtraConfig != nil {
		e.ExtraConfig = *p.ExtraConfig
	}
	backends := make([]*Backend, 0, len(p.Backend))
	for _, b := range p.Backend {
		backends = append(backends, b.normalize())
	}
	e.Backend = backends
	return &e
}

type parseableAsyncAgent struct {
	Name       string `json:"name"`
	Connection struct {
		MaxRetries      int    `json:"max_retries"`
		BackoffStrategy string `json:"backoff_strategy"`
		HealthInterval  string `json:"health_interval"`
	} `json:"connection"`
	Consumer struct {
		Timeout string  `json:"timeout"`
		Workers int     `json:"workers"`
		Topic   string  `json:"topic"`
		MaxRate float64 `json:"max_rate"`
	} `json:"consumer"`
	Encoding    string              `json:"encoding"`
	Backend     []*parseableBackend `json:"backend"`
	ExtraConfig ExtraConfig         `json:"extra_config"`
}

func (p *parseableAsyncAgent) normalize() *AsyncAgent {
	e := AsyncAgent{
		Name:     p.Name,
		Encoding: p.Encoding,
		Connection: Connection{
			MaxRetries:      p.Connection.MaxRetries,
			BackoffStrategy: p.Connection.BackoffStrategy,
			HealthInterval:  parseDuration(p.Connection.HealthInterval),
		},
		Consumer: Consumer{
			Timeout: parseDuration(p.Consumer.Timeout),
			Workers: p.Consumer.Workers,
			Topic:   p.Consumer.Topic,
			MaxRate: p.Consumer.MaxRate,
		},
	}
	if p.ExtraConfig != nil {
		e.ExtraConfig = p.ExtraConfig
	}
	backends := make([]*Backend, 0, len(p.Backend))
	for _, b := range p.Backend {
		backends = append(backends, b.normalize())
	}
	e.Backend = backends
	return &e
}

type parseableBackend struct {
	Group                    string            `json:"group"`
	Method                   string            `json:"method"`
	Host                     []string          `json:"host"`
	HostSanitizationDisabled bool              `json:"disable_host_sanitize"`
	URLPattern               string            `json:"url_pattern"`
	AllowList                []string          `json:"allow"`
	DenyList                 []string          `json:"deny"`
	Mapping                  map[string]string `json:"mapping"`
	Encoding                 string            `json:"encoding"`
	IsCollection             bool              `json:"is_collection"`
	Target                   string            `json:"target"`
	ExtraConfig              *ExtraConfig      `json:"extra_config,omitempty"`
	SD                       string            `json:"sd"`
}

func (p *parseableBackend) normalize() *Backend {
	b := Backend{
		Group:                    p.Group,
		Method:                   p.Method,
		Host:                     p.Host,
		HostSanitizationDisabled: p.HostSanitizationDisabled,
		URLPattern:               p.URLPattern,
		Mapping:                  p.Mapping,
		Encoding:                 p.Encoding,
		IsCollection:             p.IsCollection,
		Target:                   p.Target,
		SD:                       p.SD,
		AllowList:                p.AllowList,
		DenyList:                 p.DenyList,
	}
	if p.ExtraConfig != nil {
		b.ExtraConfig = *p.ExtraConfig
	}
	return &b
}

func parseDuration(v string) time.Duration {
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0
	}
	return d
}
