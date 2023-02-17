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

package common

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v4"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AddLogToPodAnnotation(ctx context.Context, client client.Client, fn func() error) error {
	jobErr := fn()
	if jobErr != nil {
		jobErrMsg := errors.Wrap(jobErr, "foo failed")
		// get job's pod
		pod := &corev1.Pod{}
		err := client.Get(ctx, types.NamespacedName{
			Namespace: os.Getenv("MY_POD_NAMESPACE"),
			Name:      os.Getenv("MY_POD_NAME"),
		}, pod)
		if err != nil {
			return err
		}
		// update err logs to job's pod
		annotations := map[string]string{"err-log": fmt.Sprintf("%+v", jobErrMsg)}
		pod.Annotations = annotations
		err = client.Update(ctx, pod)
		if err != nil {
			return err
		}
		return jobErr
	}
	return nil
}

func Archive(ctx context.Context, inputFiles map[string]string, outputFile string) error {
	files, err := archiver.FilesFromDisk(nil, inputFiles)
	if err != nil {
		return err
	}
	// create the output file we'll write to
	out, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer out.Close()

	// we can use the CompressedArchive type to gzip a tarball
	// (compression is not required; you could use Tar directly)
	format := archiver.CompressedArchive{
		Compression: archiver.Gz{},
		Archival:    archiver.Tar{},
	}
	// create the archive
	err = format.Archive(ctx, out, files)
	if err != nil {
		return err
	}
	return nil
}

func UnArchive(ctx context.Context, sourceFile string, targetPath string) error {
	source, err := os.Open(sourceFile)
	if err != nil {
		return err
	}
	defer source.Close()
	sourceName := filepath.Base(sourceFile)
	format, reader, err := archiver.Identify(sourceName, source)
	if err != nil {
		return err
	}

	err = format.(archiver.Extractor).Extract(ctx, reader, nil, func(ctx context.Context, f archiver.File) error {
		if strings.Contains(f.NameInArchive, "__MACOSX") {
			return nil
		}

		wp := filepath.Join(targetPath, f.NameInArchive)

		if f.IsDir() {
			if err := os.MkdirAll(wp, os.ModePerm); err != nil {
				return err
			}
			return nil
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		wf, err := os.Create(wp)
		if err != nil {
			return err
		}
		defer wf.Close()

		if _, err := io.Copy(wf, rc); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func CalcFileMD5(filename string) (string, error) {
	f, err := os.Open(filename)
	if nil != err {
		fmt.Println(err)
		return "", err
	}
	defer f.Close()

	md5Handle := md5.New()
	_, err = io.Copy(md5Handle, f)
	if nil != err {
		fmt.Println(err)
		return "", err
	}
	md := md5Handle.Sum(nil)
	md5str := fmt.Sprintf("%x", md)
	return md5str, nil
}
