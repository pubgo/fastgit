package copilotperm

import "sync"

var (
	globalBrokerMu sync.RWMutex
	globalBroker   *Broker
)

// SetGlobalBroker registers the active Copilot permission broker (e.g. from agentline TUI).
func SetGlobalBroker(b *Broker) {
	globalBrokerMu.Lock()
	globalBroker = b
	globalBrokerMu.Unlock()
}

// GlobalBroker returns the active broker, if any.
func GlobalBroker() *Broker {
	globalBrokerMu.RLock()
	defer globalBrokerMu.RUnlock()
	return globalBroker
}

// PrompterForMode returns the best prompter for ask mode.
func PrompterForMode(mode Mode, fallback Prompter) Prompter {
	if mode != ModeAsk {
		return fallback
	}
	if b := GlobalBroker(); b != nil {
		return b.Prompter()
	}
	return fallback
}
