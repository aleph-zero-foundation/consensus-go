package config

import (
	"time"
)

func parseDuration(s string) time.Duration {
	ret, err := time.ParseDuration(s)
	if err != nil {
		return time.Duration(0)
	}
	return ret
}

func generateAlertConfig(conf *Configuration, m *Member, c *Committee) *Alert {
	addresses := c.Addresses[len(c.Addresses)-1]
	timeout, _ := time.ParseDuration("2s")
	return &Alert{
		Pid:             m.Pid,
		PublicKeys:      c.PublicKeys,
		Pubs:            c.RMCVerificationKeys,
		Priv:            m.RMCSecretKey,
		LocalAddress:    addresses[m.Pid],
		RemoteAddresses: addresses,
		Timeout:         timeout,
	}
}

func generateSyncSetupConfig(conf *Configuration, m *Member, c *Committee) []*Sync {
	nTypes := len(conf.SyncSetup)
	syncConfs := make([]*Sync, nTypes)
	for i := range syncConfs {
		syncConfs[i] = &Sync{
			Type:            conf.SyncSetup[i].Type,
			Pid:             m.Pid,
			LocalAddress:    c.SetupAddresses[i][m.Pid],
			RemoteAddresses: c.SetupAddresses[i],
			Params:          conf.SyncSetup[i].Params,
			Fallback:        conf.SyncSetup[i].Fallback,
			Pubs:            c.RMCVerificationKeys,
			Priv:            m.RMCSecretKey,
		}
	}
	return syncConfs
}

func generateSyncConfig(conf *Configuration, m *Member, c *Committee) []*Sync {
	nTypes := len(conf.Sync)
	syncConfs := make([]*Sync, nTypes)
	for i := range syncConfs {
		syncConfs[i] = &Sync{
			Type:            conf.Sync[i].Type,
			Pid:             m.Pid,
			LocalAddress:    c.Addresses[i][m.Pid],
			RemoteAddresses: c.Addresses[i],
			Params:          conf.Sync[i].Params,
			Fallback:        conf.Sync[i].Fallback,
			Pubs:            c.RMCVerificationKeys,
			Priv:            m.RMCSecretKey,
		}
	}
	return syncConfs
}

func generateCreateSetupConfig(conf *Configuration, m *Member, c *Committee) *Create {
	return &Create{
		Pid:          m.Pid,
		CanSkipLevel: false,
		PrivateKey:   m.PrivateKey,
		Delay:        time.Duration(conf.CreateDelay * float32(time.Second)),
		MaxLevel:     conf.LevelLimit,
	}
}

func generateCreateConfig(conf *Configuration, m *Member, c *Committee) *Create {
	return &Create{
		Pid:          m.Pid,
		CanSkipLevel: conf.CanSkipLevel,
		PrivateKey:   m.PrivateKey,
		Delay:        time.Duration(conf.CreateDelay * float32(time.Second)),
		MaxLevel:     conf.LevelLimit,
	}
}

func generateOrderSetupConfig(conf *Configuration, m *Member, c *Committee) *Order {
	return &Order{
		Pid:             m.Pid,
		OrderStartLevel: 6,
		CRPFixedPrefix:  conf.CRPFixedPrefix,
	}
}

func generateOrderConfig(conf *Configuration, m *Member, c *Committee) *Order {
	return &Order{
		Pid:             m.Pid,
		OrderStartLevel: conf.OrderStartLevel,
		CRPFixedPrefix:  conf.CRPFixedPrefix,
	}
}

// GenerateConfig translates the configuration and committee information into a process config.
func (conf *Configuration) GenerateConfig(m *Member, c *Committee) Config {
	return Config{

		Sync:          generateSyncConfig(conf, m, c),
		SyncSetup:     generateSyncSetupConfig(conf, m, c),
		Create:        generateCreateConfig(conf, m, c),
		CreateSetup:   generateCreateSetupConfig(conf, m, c),
		Order:         generateOrderConfig(conf, m, c),
		OrderSetup:    generateOrderSetupConfig(conf, m, c),
		MemLog:        conf.LogMemInterval,
		Setup:         conf.Setup,
		P2PPublicKeys: c.P2PPublicKeys,
		P2PSecretKey:  m.P2PSecretKey,
		PublicKeys:    c.PublicKeys,
	}
}
