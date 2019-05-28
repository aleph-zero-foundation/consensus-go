package config

import (
	"time"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

func generatePosetConfig(c *Committee) *gomel.PosetConfig {
	return &gomel.PosetConfig{
		Keys: c.PublicKeys,
	}
}

func generateSyncConfig(conf *Configuration, c *Committee) *process.Sync {
	// TODO: Timeout should also be read from config.
	return &process.Sync{
		Pid:                  c.Pid,
		LocalAddress:         c.Addresses[c.Pid],
		RemoteAddresses:      c.Addresses,
		ListenQueueLength:    conf.NRecvSync,
		SyncQueueLength:      conf.NInitSync,
		InitializedSyncLimit: conf.NInitSync,
		ReceivedSyncLimit:    conf.NRecvSync,
		SyncInitDelay:        time.Duration(conf.SyncInitDelay * float32(time.Second)),
		Timeout:              2 * time.Second,
	}
}

func generateCreateConfig(conf *Configuration, c *Committee) *process.Create {
	// TODO: magic number
	maxHeight := 2137
	if conf.UnitsLimit != nil {
		maxHeight = int(*conf.UnitsLimit)
	}
	// TODO: magic number in adjust factor
	return &process.Create{
		Pid:          c.Pid,
		MaxParents:   int(conf.NParents),
		PrivateKey:   c.PrivateKey,
		InitialDelay: time.Duration(conf.CreateDelay * float32(time.Second)),
		AdjustFactor: 0.14,
		MaxLevel:     int(conf.LevelLimit),
		MaxHeight:    maxHeight,
	}
}

func generateOrderConfig(conf *Configuration) *process.Order {
	return &process.Order{
		VotingLevel:  int(conf.VotingLevel),
		PiDeltaLevel: int(conf.PiDeltaLevel),
	}
}

func generateTxValidateConfig(dbFilename string) *process.TxValidate {
	return &process.TxValidate{
		UserDb: dbFilename,
	}
}

func generateTxGenerateConfig(dbFilename string) *process.TxGenerate {
	return &process.TxGenerate{
		UserDb: dbFilename,
	}
}

// GenerateConfig translates the configuration and committee information into a process config.
func (conf *Configuration) GenerateConfig(c *Committee, dbFilename string) process.Config {
	return process.Config{
		Poset:      generatePosetConfig(c),
		Sync:       generateSyncConfig(conf, c),
		Create:     generateCreateConfig(conf, c),
		Order:      generateOrderConfig(conf),
		TxValidate: generateTxValidateConfig(dbFilename),
		TxGenerate: generateTxGenerateConfig(dbFilename),
	}
}
