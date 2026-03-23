package rcon

import "xashloger/internal/core/ports"

type Factory struct{}

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) New(addr, password string) ports.RCONClient {
	return NewRCON(addr, password)
}
