package main

import (
	"log"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/talos-systems/os-engine/api/types"
)

type runtime struct {
	sync.Mutex

	Blockdevices map[string]*types.Blockdevice
}

var Sink = runtime{
	Blockdevices: map[string]*types.Blockdevice{},
}

func (r *runtime) Send(msg proto.Message) {
	r.Lock()
	defer r.Unlock()

	switch t := msg.(type) {
	case *types.Blockdevice:
		if t.Operation == "add" {
			r.Blockdevices[t.Name] = t
		} else {
			delete(r.Blockdevices, t.Name)
		}
	default:
		return
	}

	log.Printf("%+v", r)
}

func (*runtime) Close() error {
	return nil
}
