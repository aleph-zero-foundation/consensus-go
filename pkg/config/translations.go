package config

// shallbedone: delete this file

import (
	"time"
)

func generateAlertConfig(conf *Params, m *Member, c *Committee) *Alert {
	addresses := c.Addresses[len(c.Addresses)-1]
	timeout := 2 * time.Second
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

func generateSyncConfig(conf *Params, m *Member, c *Committee) []*Sync {
	nTypes := len(conf.Sync)
	syncConfs := make([]*Sync, nTypes)
	for i := range syncConfs {
		syncConfs[i] = &Sync{
			Type:            conf.Sync[i].Type,
			Pid:             m.Pid,
			LocalAddress:    c.Addresses[i][m.Pid],
			RemoteAddresses: c.Addresses[i],
			Params:          conf.Sync[i].Params,
			Pubs:            c.RMCVerificationKeys,
			Priv:            m.RMCSecretKey,
		}
	}
	return syncConfs
}

// GenerateConfig translates the configuration and committee information into a process config.
func (conf *Params) GenerateConfig(m *Member, c *Committee) Config {
	cnf := NewMain(m, c)
	cnf.CreateDelay = time.Duration(conf.CreateDelay * float32(time.Second))
	cnf.LogLevel = conf.LogLevel
	cnf.Alert = generateAlertConfig(conf, m, c)
	cnf.Sync = generateSyncConfig(conf, m, c)
	return cnf
}
