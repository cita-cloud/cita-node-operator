package switchover

import (
	"context"
	"fmt"
	"github.com/cita-cloud/cita-node-operator/pkg/node"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type switchoverMgr struct {
	client.Client
	sourceNode string
	destNode   string
	namespace  string
}

func NewSwitchoverMgr() *switchoverMgr {
	return &switchoverMgr{}
}

func (s *switchoverMgr) Switch(ctx context.Context, sourceNode, destNode node.Node) error {
	err := sourceNode.Stop(ctx)
	if err != nil {
		return err
	}
	err = sourceNode.CheckStopped(ctx)
	if err != nil {
		return err
	}
	err = destNode.Stop(ctx)
	if err != nil {
		return err
	}
	err = destNode.CheckStopped(ctx)
	if err != nil {
		return err
	}
	// swap account configmap for two node
	err = sourceNode.UpdateAccountConfigmap(ctx, fmt.Sprintf("%s-account", destNode.GetName()))
	if err != nil {
		return err
	}
	err = destNode.UpdateAccountConfigmap(ctx, fmt.Sprintf("%s-account", sourceNode.GetName()))
	if err != nil {
		return err
	}
	err = sourceNode.Start(ctx)
	if err != nil {
		return err
	}
	err = destNode.Start(ctx)
	if err != nil {
		return err
	}
	return nil
}
