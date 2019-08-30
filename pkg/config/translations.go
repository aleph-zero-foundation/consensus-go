package config

import (
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

func generateDagConfig(c *Committee) *gomel.DagConfig {
	return &gomel.DagConfig{
		Keys: c.PublicKeys,
	}
}

func generateSyncSetupConfig(conf *Configuration, m *Member, c *Committee) []*process.Sync {
	nTypes := len(c.SetupAddresses)
	syncConfs := make([]*process.Sync, nTypes)
	for i := range syncConfs {
		syncConfs[i] = &process.Sync{
			Type:            conf.SyncSetup[i].Type,
			Pid:             m.Pid,
			LocalAddress:    c.SetupAddresses[i][m.Pid],
			RemoteAddresses: c.SetupAddresses[i],
			Params:          conf.SyncSetup[i].Params,
			Fallback:        conf.SyncSetup[i].Fallback,
		}
	}
	return syncConfs
}

func generateSyncConfig(conf *Configuration, m *Member, c *Committee) []*process.Sync {
	nTypes := len(conf.Sync)
	syncConfs := make([]*process.Sync, nTypes)
	for i := range syncConfs {
		syncConfs[i] = &process.Sync{
			Type:            conf.Sync[i].Type,
			Pid:             m.Pid,
			LocalAddress:    c.Addresses[i][m.Pid],
			RemoteAddresses: c.Addresses[i],
			Params:          conf.Sync[i].Params,
			Fallback:        conf.Sync[i].Fallback,
		}
	}
	return syncConfs
}

func generateCreateSetupConfig(conf *Configuration, m *Member, c *Committee) *process.Create {
	return &process.Create{
		Pid:          m.Pid,
		MaxParents:   int(conf.NParents),
		PrimeOnly:    conf.PrimeOnly,
		CanSkipLevel: false,
		PrivateKey:   m.PrivateKey,
		InitialDelay: time.Duration(conf.CreateDelay * float32(time.Second)),
		AdjustFactor: conf.StepSize,
		MaxLevel:     int(conf.LevelLimit),
	}
}

func generateCreateConfig(conf *Configuration, m *Member, c *Committee) *process.Create {
	return &process.Create{
		Pid:          m.Pid,
		MaxParents:   int(conf.NParents),
		PrimeOnly:    conf.PrimeOnly,
		CanSkipLevel: conf.CanSkipLevel,
		PrivateKey:   m.PrivateKey,
		InitialDelay: time.Duration(conf.CreateDelay * float32(time.Second)),
		AdjustFactor: conf.StepSize,
		MaxLevel:     int(conf.LevelLimit),
	}
}

func generateOrderSetupConfig(conf *Configuration, m *Member, c *Committee) *process.Order {
	return &process.Order{
		Pid:             m.Pid,
		OrderStartLevel: 6,
		CRPFixedPrefix:  int(conf.CRPFixedPrefix),
	}
}

func generateOrderConfig(conf *Configuration, m *Member, c *Committee) *process.Order {
	return &process.Order{
		Pid:             m.Pid,
		OrderStartLevel: int(conf.OrderStartLevel),
		CRPFixedPrefix:  int(conf.CRPFixedPrefix),
	}
}

func generateTxValidateConfig() *process.TxValidate {
	return &process.TxValidate{}
}

func generateTxGenerateConfig(conf *Configuration) *process.TxGenerate {
	return &process.TxGenerate{
		CompressionLevel: 5,
		Txpu:             uint32(conf.Txpu),
	}
}

// GenerateConfig translates the configuration and committee information into a process config.
func (conf *Configuration) GenerateConfig(m *Member, c *Committee) process.Config {
	return process.Config{
		Dag:         generateDagConfig(c),
		Sync:        generateSyncConfig(conf, m, c),
		SyncSetup:   generateSyncSetupConfig(conf, m, c),
		Create:      generateCreateConfig(conf, m, c),
		CreateSetup: generateCreateSetupConfig(conf, m, c),
		Order:       generateOrderConfig(conf, m, c),
		OrderSetup:  generateOrderSetupConfig(conf, m, c),
		TxValidate:  generateTxValidateConfig(),
		TxGenerate:  generateTxGenerateConfig(conf),
		MemLog:      conf.LogMemInterval,
		Setup:       conf.Setup,
	}
}
