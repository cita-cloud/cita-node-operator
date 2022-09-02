package switchover

import (
	"context"
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
	err, sourceAccount := sourceNode.GetAccount(ctx)
	if err != nil {
		return err
	}
	err, destAccount := destNode.GetAccount(ctx)
	if err != nil {
		return err
	}
	err = sourceNode.Stop(ctx)
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
	// swap account configmap content
	sourceAccount.Data, destAccount.Data = destAccount.Data, sourceAccount.Data
	err = sourceNode.UpdateAccount(ctx, sourceAccount)
	if err != nil {
		return err
	}
	err = destNode.UpdateAccount(ctx, destAccount)
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
