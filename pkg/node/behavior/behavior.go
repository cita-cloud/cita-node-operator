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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/utils/exec"
)

type Interface interface {
	Backup(sourcePath string, destPath string) (int64, error)
	Restore(sourcePath string, destPath string) error
	Fallback(blockHeight int64, nodeRoot string, configPath string, crypto string, consensus string) error
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

func (receiver Behavior) Backup(sourcePath string, destPath string) (int64, error) {
	totalSize, err := receiver.calculateSize(sourcePath)
	if err != nil {
		return 0, err
	}

	receiver.logger.Info(fmt.Sprintf("calculate backup total size success: [%d]", totalSize))

	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	done := make(chan bool)

	go func() {
		_ = receiver.backup(sourcePath, destPath)
		done <- true
	}()

LOOP:
	for {
		select {
		case <-done:
			break LOOP
		case <-ticker.C:
			// calculate progress
			currentSize, err := receiver.calculateSize(destPath)
			if err != nil {
				receiver.logger.Error(err, "calculate current size failed")
				return 0, err
			}
			percent, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(currentSize)*100/float64(totalSize)), 64)
			receiver.logger.Info(fmt.Sprintf("backup progress: [%.2f%%]", percent))
		}
	}

	return totalSize, nil
}

func (receiver Behavior) backup(sourcePath string, destPath string) error {
	err := receiver.execer.Command("/bin/sh", "-c", fmt.Sprintf("cp -a %s/* %s", sourcePath, destPath)).Run()
	if err != nil {
		receiver.logger.Error(err, "copy file failed")
		return err
	}
	receiver.logger.Info("copy file completed")
	return nil
}

func (receiver Behavior) calculateSize(path string) (int64, error) {
	// calculate size
	usageByte, err := receiver.execer.Command("du", "-sb", path).CombinedOutput()
	if err != nil {
		receiver.logger.Info("calculate backup total size failed")
		return 0, err
	}
	usageStr := strings.Split(string(usageByte), "\t")
	usage, err := strconv.ParseInt(usageStr[0], 10, 64)
	if err != nil {
		return 0, err
	}
	return usage, nil
}

func (receiver Behavior) Restore(sourcePath string, destPath string) error {
	totalSize, err := receiver.calculateSize(sourcePath)
	if err != nil {
		return err
	}

	receiver.logger.Info(fmt.Sprintf("calculate need restore total size success: [%d]", totalSize))

	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	done := make(chan bool)

	go func() {
		_ = receiver.restore(sourcePath, destPath)
		done <- true
	}()

LOOP:
	for {
		select {
		case <-done:
			break LOOP
		case <-ticker.C:
			// calculate progress
			currentSize, err := receiver.calculateSize(destPath)
			if err != nil {
				receiver.logger.Error(err, "calculate current size failed")
				return err
			}
			percent, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(currentSize)*100/float64(totalSize)), 64)
			receiver.logger.Info(fmt.Sprintf("backup progress: [%.2f%%]", percent))
		}
	}
	return nil
}

func (receiver Behavior) restore(sourcePath string, destPath string) error {
	err := receiver.execer.Command("/bin/sh", "-c", fmt.Sprintf("rm -rf %s/*", destPath)).Run()
	if err != nil {
		receiver.logger.Error(err, "clean dest dir failed")
		return err
	}
	err = receiver.execer.Command("/bin/sh", "-c", fmt.Sprintf("cp -af %s/* %s", sourcePath, destPath)).Run()
	if err != nil {
		receiver.logger.Error(err, "restore file failed")
		return err
	}
	receiver.logger.Info("restore file completed")
	return nil
}

func (receiver Behavior) Fallback(blockHeight int64, nodeRoot string, configPath string, crypto string, consensus string) error {
	receiver.logger.Info(fmt.Sprintf("exec block height fallback: [height: %d]...", blockHeight))
	err := receiver.execer.Command("cloud-op", "recover", fmt.Sprintf("%d", blockHeight),
		"--node-root", fmt.Sprintf("%s", nodeRoot),
		"--config-path", fmt.Sprintf("%s/config.toml", configPath),
		"--crypto", crypto,
		"--consensus", consensus).Run()
	if err != nil {
		receiver.logger.Error(err, "exec block height fallback failed")
		return err
	}
	receiver.logger.Info(fmt.Sprintf("exec block height fallback: [height: %d] successful", blockHeight))
	return nil
}
