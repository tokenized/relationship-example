package config

import (
	"encoding/json"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/specification/dist/golang/actions"
)

// EnvironmentConfig is used to hold all runtime configuration.
type EnvironmentConfig struct {
	Key         string `envconfig:"XKEY" json:"XKEY"`
	Entity      string `envconfig:"ENTITY" json:"ENTITY"`
	CommandPath string `default:"./tmp/command" envconfig:"COMMAND_PATH" json:"COMMAND_PATH"`
	Bitcoin     struct {
		Network    string  `default:"mainnet" envconfig:"BITCOIN_CHAIN" json:"BITCOIN_CHAIN"`
		IsTest     bool    `default:"true" envconfig:"IS_TEST" json:"IS_TEST"`
		DustLimit  uint64  `default:"576" envconfig:"DUST_LIMIT" json:"DUST_LIMIT"` // 576 for P2PK
		FeeRate    float32 `default:"1.0" envconfig:"FEE_RATE" json:"FEE_RATE"`
		AddressGap int     `default:"5" envconfig:"ADDRESS_GAP" json:"ADDRESS_GAP"`
		WalletPath string  `default:"m/7400'/0'/0'/0" envconfig:"WALLET_PATH" json:"WALLET_PATH"`
	}
	SpyNode struct {
		Address        string `default:"127.0.0.1:8333" envconfig:"NODE_ADDRESS"`
		UserAgent      string `default:"/Tokenized:0.1.0/" envconfig:"NODE_USER_AGENT"`
		StartHash      string `envconfig:"START_HASH"`
		UntrustedNodes int    `default:"25" envconfig:"UNTRUSTED_NODES"`
		SafeTxDelay    int    `default:"2000" envconfig:"SAFE_TX_DELAY"`
		ShotgunCount   int    `default:"100" envconfig:"SHOTGUN_COUNT"`
		MaxRetries     int    `default:"25" envconfig:"NODE_MAX_RETRIES"`
		RetryDelay     int    `default:"5000" envconfig:"NODE_RETRY_DELAY"`
	}
	RpcNode struct {
		Host       string `envconfig:"RPC_HOST"`
		Username   string `envconfig:"RPC_USERNAME"`
		Password   string `envconfig:"RPC_PASSWORD"`
		MaxRetries int    `default:"10" envconfig:"RPC_MAX_RETRIES"`
		RetryDelay int    `default:"2000" envconfig:"RPC_RETRY_DELAY"`
	}
	AWS struct {
		Region          string `default:"ap-southeast-2" envconfig:"AWS_REGION" json:"AWS_REGION"`
		AccessKeyID     string `envconfig:"AWS_ACCESS_KEY_ID" json:"AWS_ACCESS_KEY_ID"`
		SecretAccessKey string `envconfig:"AWS_SECRET_ACCESS_KEY" json:"AWS_SECRET_ACCESS_KEY"`
		MaxRetries      int    `default:"10" envconfig:"AWS_MAX_RETRIES"`
		RetryDelay      int    `default:"2000" envconfig:"AWS_RETRY_DELAY"`
	}
	NodeStorage struct {
		Bucket string `default:"standalone" envconfig:"NODE_STORAGE_BUCKET"`
		Root   string `default:"./tmp" envconfig:"NODE_STORAGE_ROOT"`
	}
	Storage struct {
		Bucket string `default:"standalone" envconfig:"STORAGE_BUCKET"`
		Root   string `default:"./tmp" envconfig:"STORAGE_ROOT"`
	}
	Identity struct {
		URL string `envconfig:"IDENTITY_URL" json:"IDENTITY_URL"`
	}
}

// SafeConfig masks sensitive config values
func (c *EnvironmentConfig) SafeConfig() *EnvironmentConfig {
	cfgSafe := *c

	if len(cfgSafe.Key) > 0 {
		cfgSafe.Key = "*** Masked ***"
	}
	if len(cfgSafe.RpcNode.Password) > 0 {
		cfgSafe.RpcNode.Password = "*** Masked ***"
	}
	if len(cfgSafe.AWS.AccessKeyID) > 0 {
		cfgSafe.AWS.AccessKeyID = "*** Masked ***"
	}
	if len(cfgSafe.AWS.SecretAccessKey) > 0 {
		cfgSafe.AWS.SecretAccessKey = "*** Masked ***"
	}

	return &cfgSafe
}

// Environment returns configuration sourced from environment variables
func Environment() (*EnvironmentConfig, error) {
	var cfg EnvironmentConfig

	if err := envconfig.Process("NODE", &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Config is used to reference configuration values during operation.
type Config struct {
	Entity actions.EntityField

	Net        bitcoin.Network
	IsTest     bool // tokenized test signature
	DustLimit  uint64
	FeeRate    float32
	AddressGap int
	WalletPath string

	CommandPath string
}

func (c EnvironmentConfig) Config() (*Config, error) {
	result := &Config{
		IsTest:      c.Bitcoin.IsTest,
		DustLimit:   c.Bitcoin.DustLimit,
		FeeRate:     c.Bitcoin.FeeRate,
		AddressGap:  c.Bitcoin.AddressGap,
		WalletPath:  c.Bitcoin.WalletPath,
		CommandPath: c.CommandPath,
	}

	if len(c.Entity) > 0 {
		// Put json data into opReturn struct
		if err := json.Unmarshal([]byte(c.Entity), &result.Entity); err != nil {
			return nil, errors.Wrap(err, "unmarshal entity")
		}
	}

	result.Net = bitcoin.NetworkFromString(c.Bitcoin.Network)
	if result.Net == bitcoin.InvalidNet {
		return nil, errors.New("Invalid bitcoin network")
	}

	return result, nil
}
