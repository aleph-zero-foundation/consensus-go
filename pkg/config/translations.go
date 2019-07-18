package config

import (
	"time"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

func generateDagConfig(c *Committee) *gomel.DagConfig {
	return &gomel.DagConfig{
		Keys: c.PublicKeys,
	}
}

func generateSyncConfig(conf *Configuration, c *Committee) *process.Sync {
	return &process.Sync{
		Pid:             c.Pid,
		LocalAddress:    c.Addresses[c.Pid],
		RemoteAddresses: c.Addresses,
		OutSyncLimit:    conf.NOutSync,
		InSyncLimit:     conf.NInSync,
		Timeout:         2 * time.Second,
	}
}

func generateCreateConfig(conf *Configuration, c *Committee) *process.Create {
	return &process.Create{
		Pid:          c.Pid,
		MaxParents:   int(conf.NParents),
		PrimeOnly:    conf.PrimeOnly,
		PrivateKey:   c.PrivateKey,
		InitialDelay: time.Duration(conf.CreateDelay * float32(time.Second)),
		AdjustFactor: conf.StepSize,
		MaxLevel:     int(conf.LevelLimit),
	}
}

func generateOrderConfig(conf *Configuration, c *Committee) *process.Order {
	return &process.Order{
		Pid:          c.Pid,
		VotingLevel:  int(conf.VotingLevel),
		PiDeltaLevel: int(conf.PiDeltaLevel),
	}
}

func generateTxValidateConfig(dbFilename string) *process.TxValidate {
	return &process.TxValidate{
		UserDb: dbFilename,
	}
}

func generateTxGenerateConfig(conf *Configuration, dbFilename string) *process.TxGenerate {
	return &process.TxGenerate{
		CompressionLevel: 5,
		UserDb:           dbFilename,
		Txpu:             uint32(conf.Txpu),
	}
}

// GenerateConfig translates the configuration and committee information into a process config.
func (conf *Configuration) GenerateConfig(c *Committee, dbFilename string) process.Config {
	return process.Config{
		Dag:        generateDagConfig(c),
		Sync:       generateSyncConfig(conf, c),
		Create:     generateCreateConfig(conf, c),
		Order:      generateOrderConfig(conf, c),
		TxValidate: generateTxValidateConfig(dbFilename),
		TxGenerate: generateTxGenerateConfig(conf, dbFilename),
		MemLog:     conf.LogMemInterval,
	}
}
