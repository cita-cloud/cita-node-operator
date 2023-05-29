/*
Copyright Rivtower Technologies LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package behavior

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/utils/exec"

	"github.com/cita-cloud/cita-node-operator/pkg/common"
)

type Result struct {
	Size int64
	Md5  string
}

func newResult(size int64, md5 string) *Result {
	return &Result{
		Size: size,
		Md5:  md5,
	}
}

type Interface interface {
	Backup(sourcePath string, destPath string, options *common.CompressOptions) (*Result, error)
	Restore(sourcePath string, destPath string, options *common.DecompressOptions, deleteConsensusData bool) error
	Fallback(blockHeight int64, nodeRoot string, configPath string, crypto string, consensus string, deleteConsensusData bool) error
	Snapshot(blockHeight int64, nodeRoot string, configPath string, backupPath string, crypto string, consensus string) (int64, error)
	SnapshotRecover(blockHeight int64, nodeRoot string, configPath string, backupPath string, crypto string, consensus string, deleteConsensusData bool) error
	ChangeOwner(uid, gid int64, path string) error
}

type Behavior struct {
	execer exec.Interface
	logger logr.Logger
}

func NewBehavior(execer exec.Interface, logger logr.Logger) Interface {
	return &Behavior{
		execer: execer,
		logger: logger,
	}
}

func (receiver Behavior) Backup(sourcePath string, destPath string, options *common.CompressOptions) (*Result, error) {
	receiver.logger.Info(fmt.Sprintf("source path: %s, dest path: %s", sourcePath, destPath))
	var md5 string
	totalSize, err := receiver.calculateSize(sourcePath)
	if err != nil {
		return nil, err
	}

	receiver.logger.Info(fmt.Sprintf("calculate backup total size success: [%d]", totalSize))

	// execute backup
	err = receiver.backup(sourcePath, destPath, options)
	if err != nil {
		return nil, err
	}
	if options != nil && options.Enable {
		//  check md5
		md5, err = common.CalcFileMD5(filepath.Join(destPath, options.Output))
		if err != nil {
			return nil, err
		}
	}
	return newResult(totalSize, md5), nil
}

func (receiver Behavior) backup(sourcePath string, destPath string, options *common.CompressOptions) error {
	if options.Enable {
		receiver.logger.Info(fmt.Sprintf("compress enable, type: %s, output: %s", options.CType, options.Output))
		inputFiles := map[string]string{
			fmt.Sprintf("%s/data", sourcePath):       "",
			fmt.Sprintf("%s/chain_data", sourcePath): "",
		}
		outputFilePath := filepath.Join(destPath, options.Output)
		err := common.Archive(context.Background(), inputFiles, outputFilePath)
		if err != nil {
			receiver.logger.Error(err, "archive file failed")
			return err
		}
	} else {
		cmd := fmt.Sprintf("cp -a %s/data %s/chain_data %s", sourcePath, sourcePath, destPath)
		receiver.logger.Info(cmd)
		err := receiver.execer.Command("/bin/sh", "-c", cmd).Run()
		if err != nil {
			receiver.logger.Error(err, "copy file failed")
			return err
		}
	}
	receiver.logger.Info("copy file completed")
	return nil
}

func (receiver Behavior) calculateSize(path string) (int64, error) {
	// calculate size
	usageByte, err := receiver.execer.Command("du", "-sb", path).CombinedOutput()
	if err != nil {
		receiver.logger.Error(err, "calculate backup total size failed")
		return 0, err
	}
	usageStr := strings.Split(string(usageByte), "\t")
	usage, err := strconv.ParseInt(usageStr[0], 10, 64)
	if err != nil {
		return 0, err
	}
	return usage, nil
}

func (receiver Behavior) Restore(sourcePath string, destPath string, options *common.DecompressOptions, deleteConsensusData bool) error {
	err := receiver.restore(sourcePath, destPath, options, deleteConsensusData)
	return err
}

func (receiver Behavior) restore(sourcePath string, destPath string, options *common.DecompressOptions, deleteConsensusData bool) error {
	if options.Enable {
		if options.Md5 != "" {
			md5, err := common.CalcFileMD5(filepath.Join(sourcePath, options.Input))
			if err != nil {
				return err
			}
			if options.Md5 != md5 {
				return fmt.Errorf("md5 check error")
			}
		}
		// clean
		err := receiver.execer.Command("/bin/sh", "-c", fmt.Sprintf("rm -rf %s/chain_data %s/data", destPath, destPath)).Run()
		if err != nil {
			receiver.logger.Error(err, "clean dest dir failed")
			return err
		}
		if deleteConsensusData {
			err = removeConsensusData(destPath)
			if err != nil {
				receiver.logger.Error(err, "remove consensus data failed")
				return err
			}
			receiver.logger.Info("remove consensus data succeed")
		}
		// unpack
		err = common.UnArchive(context.Background(), filepath.Join(sourcePath, options.Input), destPath)
		if err != nil {
			return err
		}
	} else {
		err := receiver.execer.Command("/bin/sh", "-c", fmt.Sprintf("rm -rf %s/chain_data %s/data", destPath, destPath)).Run()
		if err != nil {
			receiver.logger.Error(err, "clean dest dir failed")
			return err
		}
		if deleteConsensusData {
			err = removeConsensusData(destPath)
			if err != nil {
				receiver.logger.Error(err, "remove consensus data failed")
				return err
			}
			receiver.logger.Info("remove consensus data succeed")
		}
		cmd := fmt.Sprintf("cp -af %s/chain_data %s/data %s", sourcePath, sourcePath, destPath)
		receiver.logger.Info(cmd)
		err = receiver.execer.Command("/bin/sh", "-c", cmd).Run()
		if err != nil {
			receiver.logger.Error(err, "restore file failed")
			return err
		}
	}
	receiver.logger.Info("restore file completed")
	return nil
}

const (
	RaftConsensusDir     string = "raft_data_dir"
	OverLordConsensusDir string = "overlord_wal"
)

func removeConsensusData(parentDir string) error {
	_, err := os.Stat(filepath.Join(parentDir, RaftConsensusDir))
	if err == nil {
		// exist, delete it
		err := os.RemoveAll(filepath.Join(parentDir, RaftConsensusDir))
		if err != nil {
			return err
		}
	}
	if !os.IsNotExist(err) {
		return err
	}
	_, err = os.Stat(filepath.Join(parentDir, OverLordConsensusDir))
	if err == nil {
		// exist, delete it
		err := os.RemoveAll(filepath.Join(parentDir, OverLordConsensusDir))
		if err != nil {
			return err
		}
	}
	if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (receiver Behavior) Snapshot(blockHeight int64, nodeRoot string, configPath string, backupPath string, crypto string, consensus string) (int64, error) {
	receiver.logger.Info(
		fmt.Sprintf("exec snapshot: [height: %d, node-root: %s, config-path: %s, backup-path: %s, crypto: %s, consensus: %s]...",
			blockHeight, nodeRoot, configPath, backupPath, crypto, consensus))
	err := receiver.execer.Command("cloud-op", "state-backup", fmt.Sprintf("%d", blockHeight),
		"--node-root", nodeRoot,
		"--config-path", fmt.Sprintf("%s/config.toml", configPath),
		"--backup-path", backupPath,
		"--crypto", crypto,
		"--consensus", consensus).Run()
	if err != nil {
		receiver.logger.Error(err, "exec snapshot failed")
		return 0, err
	}
	snapshotSize, err := receiver.calculateSize(backupPath)
	receiver.logger.Info(fmt.Sprintf("exec snapshot: [height: %d, size: %d] successful", blockHeight, snapshotSize))
	return snapshotSize, nil
}

func (receiver Behavior) SnapshotRecover(blockHeight int64, nodeRoot string, configPath string, backupPath string, crypto string, consensus string, deleteConsensusData bool) error {
	receiver.logger.Info(
		fmt.Sprintf("exec snapshot recover: [height: %d, node-root: %s, config-path: %s, backup-path: %s, crypto: %s, consensus: %s, deleteConsensusData: %t]...",
			blockHeight, nodeRoot, configPath, backupPath, crypto, consensus, deleteConsensusData))
	args := buildSnapshotRecoverArgs(blockHeight, nodeRoot, configPath, backupPath, crypto, consensus, deleteConsensusData)
	err := receiver.execer.Command("cloud-op", args...).Run()
	if err != nil {
		receiver.logger.Error(err, "exec snapshot recover failed")
		return err
	}
	receiver.logger.Info(fmt.Sprintf("exec snapshot recover: [height: %d] successful", blockHeight))
	return nil
}

func buildSnapshotRecoverArgs(blockHeight int64,
	nodeRoot string, configPath string, backupPath string, crypto string, consensus string,
	deleteConsensusData bool) []string {
	args := []string{
		"state-recover",
		fmt.Sprintf("%d", blockHeight),
		"--node-root", fmt.Sprintf("%s", nodeRoot),
		"--config-path", fmt.Sprintf("%s/config.toml", configPath),
		"--backup-path", backupPath,
		"--crypto", crypto,
		"--consensus", consensus,
	}
	if deleteConsensusData {
		args = append(args, "--is-clear")
	}
	return args
}

func buildFallbackArgs(blockHeight int64,
	nodeRoot string, configPath string, crypto string, consensus string,
	deleteConsensusData bool) []string {
	args := []string{
		"recover",
		fmt.Sprintf("%d", blockHeight),
		"--node-root", fmt.Sprintf("%s", nodeRoot),
		"--config-path", fmt.Sprintf("%s/config.toml", configPath),
		"--crypto", crypto,
		"--consensus", consensus,
	}
	if deleteConsensusData {
		args = append(args, "--is-clear")
	}
	return args
}

func (receiver Behavior) Fallback(blockHeight int64, nodeRoot string, configPath string, crypto string, consensus string, deleteConsensusData bool) error {
	receiver.logger.Info(fmt.Sprintf("exec block height fallback: [height: %d]...", blockHeight))
	args := buildFallbackArgs(blockHeight, nodeRoot, configPath, crypto, consensus, deleteConsensusData)
	err := receiver.execer.Command("cloud-op", args...).Run()
	if err != nil {
		receiver.logger.Error(err, "exec block height fallback failed")
		return err
	}
	receiver.logger.Info(fmt.Sprintf("exec block height fallback: [height: %d] successful", blockHeight))
	return nil
}

func (receiver Behavior) ChangeOwner(uid, gid int64, path string) error {
	receiver.logger.Info(fmt.Sprintf("exec chown: [path: %s, uid: %d, gid: %d]...", path, uid, gid))
	err := receiver.execer.Command("chown", "-R", fmt.Sprintf("%d:%d", uid, gid), path).Run()
	if err != nil {
		receiver.logger.Error(err, "exec chown failed")
		return err
	}
	receiver.logger.Info(fmt.Sprintf("exec chown: [path: %s, uid: %d, gid: %d] successful", path, uid, gid))
	return nil
}
