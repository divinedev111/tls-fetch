package tlsfetch

import (
	"log/slog"
	"time"

	"github.com/mukuln-official/tls-fetch/internal"
)

// PoolConfig is an alias for internal.PoolConfig exposed at the package level.
type PoolConfig = internal.PoolConfig

type Option func(*config)

type config struct {
	profile            *BrowserProfile
	profilePath        string
	ja3String          string
	proxyURL           string
	timeout            time.Duration
	logger             *slog.Logger
	pool               PoolConfig
	insecureSkipVerify bool
	followRedirects    bool
}

func defaultConfig() *config {
	return &config{
		timeout:         30 * time.Second,
		followRedirects: true,
		pool: PoolConfig{
			MaxConnsPerHost: 10,
			MaxIdleConns:    100,
			IdleTimeout:     90 * time.Second,
		},
	}
}

func WithProfile(p BrowserProfile) Option {
	return func(c *config) { c.profile = &p }
}

func WithProfileFromFile(path string) Option {
	return func(c *config) { c.profilePath = path }
}

func WithProfileFromJA3(ja3 string) Option {
	return func(c *config) { c.ja3String = ja3 }
}

func WithProxy(url string) Option {
	return func(c *config) { c.proxyURL = url }
}

func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

func WithLogger(l *slog.Logger) Option {
	return func(c *config) { c.logger = l }
}

func WithPoolConfig(p PoolConfig) Option {
	return func(c *config) { c.pool = p }
}

func WithInsecureSkipVerify() Option {
	return func(c *config) { c.insecureSkipVerify = true }
}

func WithNoRedirects() Option {
	return func(c *config) { c.followRedirects = false }
}
